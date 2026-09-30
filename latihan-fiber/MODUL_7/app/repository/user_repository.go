package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"MODUL_7/app/model" 
)

type UserRepository interface {
	FindAll(ctx context.Context, q model.ListQuery) ([]model.User, int, error)
	FindByID(ctx context.Context, id int) (model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	Create(ctx context.Context, u model.User) (model.User, error)
	Update(ctx context.Context, u model.User) (model.User, error)
	UpdateRole(ctx context.Context, id int, role string) (model.User, error)
	Delete(ctx context.Context, id int) error
}

var kolomUrut = map[string]string{
	"id":         "id",
	"username":   "username",
	"email":      "email",
	"created_at": "created_at",
}

type userPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userPostgresRepository{pool: pool}
}

const userColumns = "id, username, email, password, role, is_active, created_at"

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Password, &u.Role, &u.IsActive, &u.CreatedAt)
	return u, err
}

func buildFilter(q model.ListQuery) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	if q.Search != "" {
		where += fmt.Sprintf(" AND (username ILIKE $%d OR email ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	if q.IsActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", len(args)+1)
		args = append(args, *q.IsActive)
	}
	return where, args
}

func (r *userPostgresRepository) FindAll(ctx context.Context, q model.ListQuery) ([]model.User, int, error) {
	where, args := buildFilter(q)
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("menghitung user: %w", err)
	}
	
	arah := "ASC"
	if q.Order == "desc" {
		arah = "DESC"
	}
	
	sqlText := fmt.Sprintf(
		"SELECT %s FROM users %s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		userColumns, where, kolomUrut[q.Sort], arah, len(args)+1, len(args)+2,
	)
	args = append(args, q.Limit, q.Offset())
	
	rows, err := r.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mengambil daftar user: %w", err)
	}
	defer rows.Close()
	
	hasil := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("membaca baris user: %w", err)
		}
		hasil = append(hasil, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("membaca hasil query: %w", err)
	}
	return hasil, total, nil
}

func (r *userPostgresRepository) FindByID(ctx context.Context, id int) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengambil user: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE LOWER(username) = LOWER($1)", username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengambil user: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) Create(ctx context.Context, u model.User) (model.User, error) {
	err := r.pool.QueryRow(ctx,
		"INSERT INTO users (username, email, password, role, is_active) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at",
		u.Username, u.Email, u.Password, u.Role, u.IsActive,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrDuplicate
		}
		return model.User{}, fmt.Errorf("menyimpan user: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) Update(ctx context.Context, u model.User) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		"UPDATE users SET username = $1, email = $2, is_active = $3 WHERE id = $4 RETURNING "+userColumns,
		u.Username, u.Email, u.IsActive, u.ID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return model.User{}, ErrDuplicate
		}
		return model.User{}, fmt.Errorf("memperbarui user: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) UpdateRole(ctx context.Context, id int, role string) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		"UPDATE users SET role = $1 WHERE id = $2 RETURNING "+userColumns,
		role, id,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengubah role user: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("menghapus user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}