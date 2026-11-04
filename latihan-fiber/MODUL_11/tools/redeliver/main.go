package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"MODUL_11/app/model"
	"MODUL_11/config"
	"MODUL_11/database"
)

// Alat uji Bagian C.5 dan D.2: menaruh beberapa SALINAN task langsung ke
// Redis, meniru jaminan at-least-once (task yang sama datang berkali-kali).
//
// Penggunaan:
//
//	go run ./tools/redeliver -outbox <id> -copies 5
//	go run ./tools/redeliver -outbox <id> -copies 5 -user <id>
//
// Untuk event course.quota_changed, tanpa -user alat ini menyalin task
// untuk SELURUH peserta kelas; dengan -user hanya untuk mahasiswa itu.
func main() {
	outboxID := flag.Int64("outbox", 0, "id baris outbox yang task-nya akan disalin")
	copies := flag.Int("copies", 1, "jumlah salinan task yang ditaruh ke Redis")
	userID := flag.Int("user", 0, "id penerima (kosong = semua peserta untuk event kuota)")
	flag.Parse()

	if *outboxID <= 0 || *copies <= 0 {
		fmt.Fprintln(os.Stderr, "penggunaan: go run ./tools/redeliver -outbox <id> -copies <n> [-user <id>]")
		os.Exit(1)
	}

	config.LoadEnv()
	ctx := context.Background()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal terhubung ke database:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var eventType string
	var payload []byte
	var publishedAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT event_type, payload, published_at FROM outbox WHERE id = $1`,
		*outboxID).Scan(&eventType, &payload, &publishedAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal membaca outbox %d: %v\n", *outboxID, err)
		os.Exit(1)
	}

	recipients, err := recipientsFor(ctx, pool, eventType, payload, *userID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()

	for _, recipient := range recipients {
		job := model.NotificationJob{
			OutboxID: *outboxID, EventType: eventType, Payload: payload,
			RecipientID: recipient,
		}
		body, err := json.Marshal(job)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gagal menyusun payload task:", err)
			os.Exit(1)
		}

		for i := 1; i <= *copies; i++ {
			// TaskID berbeda dari task milik relay agar salinan ini
			// benar-benar diterima sebagai task terpisah.
			taskID := fmt.Sprintf("redeliver-%d-%d", *outboxID, i)
			if recipient > 0 {
				taskID = fmt.Sprintf("redeliver-%d-u%d-%d", *outboxID, recipient, i)
			}
			task := asynq.NewTask(model.TaskSendNotification, body,
				asynq.TaskID(taskID),
				asynq.Queue(model.QueueNotifications),
				asynq.MaxRetry(5),
				asynq.Timeout(30*time.Second),
			)
			info, err := client.Enqueue(task)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gagal enqueue salinan %d: %v\n", i, err)
				os.Exit(1)
			}
			if recipient > 0 {
				fmt.Printf("penerima %d salinan %d: id=%s queue=%s\n",
					recipient, i, info.ID, info.Queue)
			} else {
				fmt.Printf("salinan %d: id=%s queue=%s\n", i, info.ID, info.Queue)
			}
		}
	}

	fmt.Printf("%d salinan task outbox %d (%s) untuk %d penerima ditaruh di queue %s (published_at=%v)\n",
		*copies, *outboxID, eventType, len(recipients), model.QueueNotifications, publishedAt)
}

// recipientsFor menentukan siapa saja yang menerima salinan task.
//
// Event KRS membawa user_id di payload. Event kuota tidak membawa penerima
// sama sekali, sehingga daftarnya dibaca dari enrollments — persis seperti
// yang dilakukan relay.
func recipientsFor(ctx context.Context, pool *pgxpool.Pool, eventType string, payload []byte, userID int) ([]int, error) {
	if eventType != model.EventCourseQuotaChanged {
		if userID > 0 {
			return []int{userID}, nil
		}
		var event model.EnrollmentEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("payload event rusak: %w", err)
		}
		return []int{event.UserID}, nil
	}

	if userID > 0 {
		return []int{userID}, nil
	}

	var event model.CourseQuotaEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("payload event kuota rusak: %w", err)
	}
	rows, err := pool.Query(ctx,
		"SELECT user_id FROM enrollments WHERE course_id = $1 ORDER BY user_id", event.CourseID)
	if err != nil {
		return nil, fmt.Errorf("membaca peserta kelas: %w", err)
	}
	defer rows.Close()

	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("membaca peserta kelas: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
