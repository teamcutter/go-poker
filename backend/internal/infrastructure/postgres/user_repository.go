package postgres

import (
	"context"
	"fmt"

	"github.com/teamcutter/go-poker/internal/domain/user"
)

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	const sql = `INSERT INTO users (id, public_id, username, first_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := r.db.q(ctx).Exec(ctx, sql, u.ID, u.PublicID, u.Username, u.FirstName, u.CreatedAt, u.UpdatedAt); err != nil {
		return fmt.Errorf("create user: %w", normalize(err))
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*user.User, error) {
	const sql = `SELECT id, public_id, username, first_name, created_at, updated_at FROM users WHERE id = $1`
	var u user.User
	err := r.db.q(ctx).QueryRow(ctx, sql, id).Scan(&u.ID, &u.PublicID, &u.Username, &u.FirstName, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}
