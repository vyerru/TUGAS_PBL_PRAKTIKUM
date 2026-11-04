package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"MODUL_11/app/model"
	"MODUL_11/app/repository"
)

// TaskEnqueuer adalah kemampuan melakukan enqueue task.
// *asynq.Client memenuhi interface ini.
type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// OutboxRelay memindahkan event dari tabel outbox ke queue Redis.
//
// Relay adalah satu-satunya komponen yang membaca outbox. API tidak pernah
// berbicara dengan Redis; ia cukup menulis ke PostgreSQL. Karena itu
// mahasiswa tetap dapat mengisi KRS walaupun Redis sedang mati — notifikasinya
// hanya tertunda, tidak hilang.
type OutboxRelay struct {
	outbox   repository.OutboxRepository
	courses  repository.CourseRepository
	tx       *repository.TxManager
	queue    TaskEnqueuer
	logger   *slog.Logger
	interval time.Duration
	batch    int
}

func NewOutboxRelay(
	outbox repository.OutboxRepository,
	courses repository.CourseRepository,
	tx *repository.TxManager,
	queue TaskEnqueuer,
	logger *slog.Logger,
	interval time.Duration,
	batch int,
) *OutboxRelay {
	return &OutboxRelay{outbox: outbox, courses: courses, tx: tx, queue: queue,
		logger: logger, interval: interval, batch: batch}
}

// Run berjalan terus sampai ctx dibatalkan.
func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		n, err := r.RelayOnce(ctx)
		if err != nil {
			r.logger.Warn("relay_delayed",
				slog.Int("published", n), slog.String("error", err.Error()))
		} else if n > 0 {
			r.logger.Info("relay_done", slog.Int("published", n))
		}

		// Satu batch penuh berarti kemungkinan besar masih ada sisa:
		// langsung lanjutkan tanpa menunggu. Selain itu, tunggu giliran
		// berikutnya.
		if err == nil && n == r.batch && ctx.Err() == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RelayOnce mem-publish paling banyak satu batch. Mengembalikan jumlah
// event yang berhasil di-publish.
func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	var published int
	var enqueueErr error

	err := r.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
		repo := r.outbox.WithTx(tx)

		events, err := repo.FetchUnpublished(ctx, r.batch)
		if err != nil {
			return err
		}

		ids := make([]int64, 0, len(events))
		for _, e := range events {
			// Berhenti pada kegagalan pertama. Bila Redis mati, event
			// berikutnya pun pasti gagal; tidak ada gunanya dicoba satu per
			// satu. Yang belum di-publish tetap NULL dan diambil lagi pada
			// putaran berikutnya.
			if err := r.publishEvent(ctx, tx, e); err != nil {
				enqueueErr = err
				break
			}
			// Hanya dicatat SETELAH enqueue berhasil. Bila id dimasukkan
			// lebih dulu, event yang gagal ikut ditandai published dan
			// hilang selamanya.
			ids = append(ids, e.ID)
		}

		// Hanya event yang SUDAH berhasil di-enqueue yang ditandai.
		if err := repo.MarkPublished(ctx, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, enqueueErr
}

// publishEvent mengubah SATU baris outbox menjadi satu atau banyak task.
//
// Event KRS biasa hanya punya satu penerima, jadi satu task sudah cukup.
// course.quota_changed berlaku untuk seluruh peserta: event itu dipecah
// menjadi satu task per mahasiswa. Keputusan ini diambil di relay, bukan di
// worker, karena hanya dengan begitu retry milik satu task tidak
// mengirim ulang email mahasiswa lain.
func (r *OutboxRelay) publishEvent(ctx context.Context, tx repository.DBTX, e model.OutboxEvent) error {
	if e.EventType != model.EventCourseQuotaChanged {
		return r.publish(ctx, e, 0)
	}

	var event model.CourseQuotaEvent
	if err := json.Unmarshal(e.Payload, &event); err != nil {
		return fmt.Errorf("payload outbox %d rusak: %w", e.ID, err)
	}

	// Daftar peserta dibaca di dalam transaction yang sama dengan
	// FetchUnpublished, sehingga susunan penerima konsisten dengan event.
	recipients, err := r.courses.WithTx(tx).ListParticipantIDs(ctx, event.CourseID)
	if err != nil {
		return fmt.Errorf("mengambil peserta kelas %d: %w", event.CourseID, err)
	}

	for _, userID := range recipients {
		if err := r.publish(ctx, e, userID); err != nil {
			return err
		}
	}
	return nil
}

func (r *OutboxRelay) publish(ctx context.Context, e model.OutboxEvent, recipientID int) error {
	job := model.NotificationJob{
		OutboxID: e.ID, EventType: e.EventType, Payload: e.Payload,
		RecipientID: recipientID,
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("menyusun payload task outbox %d: %w", e.ID, err)
	}

	// TaskID diturunkan dari id outbox DAN id penerima. Untuk event KRS
	// biasa recipientID nol sehingga TaskID-nya tetap "outbox-<id>".
	// Enqueue ulang event yang sama ditolak Redis, termasuk bila relay mati
	// SETELAH enqueue tetapi SEBELUM sempat commit.
	taskID := fmt.Sprintf("outbox-%d", e.ID)
	if recipientID > 0 {
		taskID = fmt.Sprintf("outbox-%d-u%d", e.ID, recipientID)
	}

	task := asynq.NewTask(model.TaskSendNotification, payload,
		asynq.TaskID(taskID),
		asynq.Queue(model.QueueNotifications),
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
		// Task yang selesai disimpan 24 jam. Selama itu, TaskID-nya
		// tetap tercatat sehingga perlindungan di atas tetap berlaku.
		asynq.Retention(24*time.Hour),
	)

	_, err = r.queue.EnqueueContext(ctx, task)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		// Sudah pernah di-enqueue pada putaran sebelumnya. Bagi relay,
		// itu sama dengan berhasil.
		return nil
	}
	if err != nil {
		return fmt.Errorf("enqueue outbox %d: %w", e.ID, err)
	}
	return nil
}
