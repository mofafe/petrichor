package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mofafe/petrichor/apps/server/internal/auth/model"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user model.User) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO users (id, username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.PasswordHash,
		formatTime(user.CreatedAt),
		formatTime(user.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (model.User, error) {
	return r.findOne(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM users WHERE id = ?`, id)
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	return r.findOne(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = ?`, username)
}

func (r *UserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check username exists: %w", err)
	}

	return exists, nil
}

func (r *UserRepository) findOne(ctx context.Context, query string, args ...any) (model.User, error) {
	var user model.User
	var createdAt string
	var updatedAt string

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user updated_at: %w", err)
	}

	user.CreatedAt = parsedCreatedAt
	user.UpdatedAt = parsedUpdatedAt

	return user, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
