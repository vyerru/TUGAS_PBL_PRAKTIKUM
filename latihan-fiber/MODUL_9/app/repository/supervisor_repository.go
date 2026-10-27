package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"MODUL_9/app/model"
)

type SupervisorRepository interface {
	WithTx(tx DBTX) SupervisorRepository
	FindAll(ctx context.Context) ([]model.Supervisor, error)
	FindByID(ctx context.Context, id int) (model.Supervisor, error)
	FindByStudentID(ctx context.Context, studentID int) (model.Supervision, error)
	LockStudent(ctx context.Context, studentID int) error
	LockSupervisors(ctx context.Context, ids ...int) error
	IncrementSupervised(ctx context.Context, supervisorID int) (model.Supervisor, error)
	DecrementSupervised(ctx context.Context, supervisorID int) (model.Supervisor, error)
	InsertSupervision(ctx context.Context, studentID, supervisorID int) error
	DeleteSupervision(ctx context.Context, studentID, supervisorID int) error
	UpdateQuota(ctx context.Context, id, quota, version int) (model.Supervisor, error)
}

var (
	ErrSupervisorFull            = errors.New("kuota pembimbing sudah penuh")
	ErrAlreadySupervised         = errors.New("mahasiswa sudah memiliki pembimbing")
	ErrNotSupervised             = errors.New("mahasiswa tidak dibimbing oleh dosen tersebut")
	ErrSupervisorVersionConflict = errors.New("data pembimbing sudah diubah pihak lain")
	ErrQuotaBelowSupervised      = errors.New("kuota tidak boleh lebih kecil dari jumlah mahasiswa bimbingan saat ini")
)

const supervisorColumns = `id, name, quota, supervised_count, version, created_at`

type supervisorPostgresRepository struct {
	db DBTX
}

func NewSupervisorRepository(db DBTX) SupervisorRepository {
	return &supervisorPostgresRepository{db: db}
}

func (r *supervisorPostgresRepository) WithTx(tx DBTX) SupervisorRepository {
	return &supervisorPostgresRepository{db: tx}
}

func scanSupervisor(row pgx.Row) (model.Supervisor, error) {
	var s model.Supervisor
	err := row.Scan(&s.ID, &s.Name, &s.Quota, &s.SupervisedCount, &s.Version, &s.CreatedAt)
	return s, err
}

func (r *supervisorPostgresRepository) FindAll(ctx context.Context) ([]model.Supervisor, error) {
	rows, err := r.db.Query(ctx, "SELECT "+supervisorColumns+" FROM supervisors ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar pembimbing: %w", err)
	}
	defer rows.Close()

	result := []model.Supervisor{}
	for rows.Next() {
		s, err := scanSupervisor(rows)
		if err != nil {
			return nil, fmt.Errorf("membaca row pembimbing: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *supervisorPostgresRepository) FindByID(ctx context.Context, id int) (model.Supervisor, error) {
	s, err := scanSupervisor(r.db.QueryRow(ctx,
		"SELECT "+supervisorColumns+" FROM supervisors WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Supervisor{}, ErrNotFound
		}
		return model.Supervisor{}, fmt.Errorf("mengambil pembimbing: %w", err)
	}
	return s, nil
}

func (r *supervisorPostgresRepository) FindByStudentID(ctx context.Context, studentID int) (model.Supervision, error) {
	var sv model.Supervision
	err := r.db.QueryRow(ctx,
		"SELECT id, student_id, supervisor_id, created_at FROM supervisions WHERE student_id = $1",
		studentID).Scan(&sv.ID, &sv.StudentID, &sv.SupervisorID, &sv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Supervision{}, ErrNotFound
		}
		return model.Supervision{}, fmt.Errorf("mengambil bimbingan: %w", err)
	}
	return sv, nil
}

// LockStudent mengunci baris students untuk mencegah satu mahasiswa ditetapkan
// ke dua pembimbing secara serentak (Pessimistic Locking).
func (r *supervisorPostgresRepository) LockStudent(ctx context.Context, studentID int) error {
	var id int
	err := r.db.QueryRow(ctx,
		"SELECT id FROM students WHERE id = $1 FOR UPDATE", studentID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mengunci data mahasiswa: %w", err)
	}
	return nil
}

// LockSupervisors mengunci beberapa pembimbing sekaligus dalam urutan ID menaik (ORDER BY id)
// untuk mencegah deadlock saat terjadi pergantian pembimbing berlawanan arah secara serentak.
func (r *supervisorPostgresRepository) LockSupervisors(ctx context.Context, ids ...int) error {
	arr := make([]int32, len(ids))
	for i, id := range ids {
		arr[i] = int32(id)
	}
	rows, err := r.db.Query(ctx,
		"SELECT id FROM supervisors WHERE id = ANY($1) ORDER BY id FOR UPDATE", arr)
	if err != nil {
		return fmt.Errorf("mengunci pembimbing: %w", err)
	}
	defer rows.Close()

	found := 0
	for rows.Next() {
		found++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("mengunci pembimbing: %w", err)
	}
	if found != len(ids) {
		return ErrNotFound
	}
	return nil
}

// IncrementSupervised menambah jumlah bimbingan secara ATOMIK dengan UPDATE bersyarat.
// Kuota diperiksa dan diperbarui dalam satu query (supervised_count < quota).
func (r *supervisorPostgresRepository) IncrementSupervised(ctx context.Context, supervisorID int) (model.Supervisor, error) {
	s, err := scanSupervisor(r.db.QueryRow(ctx,
		`UPDATE supervisors SET supervised_count = supervised_count + 1
		WHERE id = $1 AND supervised_count < quota
		RETURNING `+supervisorColumns, supervisorID))
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, fmt.Errorf("menambah bimbingan: %w", err)
	}
	if _, findErr := r.FindByID(ctx, supervisorID); findErr != nil {
		return model.Supervisor{}, findErr
	}
	return model.Supervisor{}, ErrSupervisorFull
}

func (r *supervisorPostgresRepository) DecrementSupervised(ctx context.Context, supervisorID int) (model.Supervisor, error) {
	s, err := scanSupervisor(r.db.QueryRow(ctx,
		`UPDATE supervisors SET supervised_count = supervised_count - 1
		WHERE id = $1
		RETURNING `+supervisorColumns, supervisorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Supervisor{}, ErrNotFound
		}
		return model.Supervisor{}, fmt.Errorf("mengurangi bimbingan: %w", err)
	}
	return s, nil
}

func (r *supervisorPostgresRepository) InsertSupervision(ctx context.Context, studentID, supervisorID int) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO supervisions (student_id, supervisor_id) VALUES ($1, $2)", studentID, supervisorID)
	if err != nil {
		if pgErrorCode(err) == codeUniqueViolation {
			return ErrAlreadySupervised
		}
		return fmt.Errorf("mencatat bimbingan: %w", err)
	}
	return nil
}

func (r *supervisorPostgresRepository) DeleteSupervision(ctx context.Context, studentID, supervisorID int) error {
	tag, err := r.db.Exec(ctx,
		"DELETE FROM supervisions WHERE student_id = $1 AND supervisor_id = $2", studentID, supervisorID)
	if err != nil {
		return fmt.Errorf("menghapus bimbingan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotSupervised
	}
	return nil
}

// UpdateQuota menerapkan Optimistic Locking dengan menaikkan kolom version = version + 1
// dan memeriksa bahwa versi di database masih sama dengan yang dilihat client.
func (r *supervisorPostgresRepository) UpdateQuota(ctx context.Context, id, quota, version int) (model.Supervisor, error) {
	s, err := scanSupervisor(r.db.QueryRow(ctx,
		`UPDATE supervisors SET quota = $1, version = version + 1
		WHERE id = $2 AND version = $3 AND $1 >= supervised_count
		RETURNING `+supervisorColumns, quota, id, version))
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, fmt.Errorf("mengubah kuota pembimbing: %w", err)
	}
	current, findErr := r.FindByID(ctx, id)
	if findErr != nil {
		return model.Supervisor{}, findErr
	}
	if current.Version != version {
		return model.Supervisor{}, ErrSupervisorVersionConflict
	}
	return model.Supervisor{}, ErrQuotaBelowSupervised
}
