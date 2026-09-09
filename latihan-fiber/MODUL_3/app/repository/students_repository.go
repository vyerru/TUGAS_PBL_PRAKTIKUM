package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/MODUL_3/app/model" 
)

type StudentRepository interface {
	FindAll(ctx context.Context, q model.ListQuery) ([]model.Student, int, error)
	FindByID(ctx context.Context, id int) (model.Student, error)
	Create(ctx context.Context, s model.Student) (model.Student, error)
	Update(ctx context.Context, s model.Student) (model.Student, error)
	Delete(ctx context.Context, id int) error
}

// Daftar putih kolom untuk ORDER BY — mencegah SQL injection lewat parameter sort.
var kolomUrutStudent = map[string]string{
	"id":         "id",
	"nim":        "nim",
	"name":       "name",
	"grade":      "grade",
	"created_at": "created_at",
}

type studentPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewStudentRepository(pool *pgxpool.Pool) StudentRepository {
	return &studentPostgresRepository{pool: pool}
}

func buildFilterStudent(q model.ListQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}

	if q.Search != "" {
		where += fmt.Sprintf(" AND (name ILIKE $%d OR nim ILIKE $%d)",
			len(args)+1, len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	if q.IsActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", len(args)+1)
		args = append(args, *q.IsActive)
	}

	return where, args
}

func (r *studentPostgresRepository) FindAll(
	ctx context.Context, q model.ListQuery,
) ([]model.Student, int, error) {
	where, args := buildFilterStudent(q)

	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM students"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("menghitung student: %w", err)
	}

	arah := "ASC"
	if q.Order == "desc" {
		arah = "DESC"
	}

	kolom, ok := kolomUrutStudent[q.Sort]
	if !ok {
		kolom = "id" // default aman kalau sort tidak dikenal
	}

	sqlText := fmt.Sprintf(
		`SELECT id, nim, name, grade, is_active, created_at
		 FROM students%s
		 ORDER BY %s %s
		 LIMIT $%d OFFSET $%d`,
		where, kolom, arah, len(args)+1, len(args)+2,
	)
	args = append(args, q.Limit, q.Offset())

	rows, err := r.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mengambil daftar student: %w", err)
	}
	defer rows.Close()

	hasil := []model.Student{}
	for rows.Next() {
		var s model.Student
		if err := rows.Scan(&s.ID, &s.NIM, &s.Name, &s.Grade, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("membaca baris student: %w", err)
		}
		hasil = append(hasil, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("membaca hasil query: %w", err)
	}

	return hasil, total, nil
}