package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pbs_help_desk/internal/models"
)

// UserRepository defines persistence operations for users and refresh tokens.
type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash, firstName, lastName string) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	GetByID(ctx context.Context, userID string) (models.User, error)
	SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	ConsumeRefreshToken(ctx context.Context, tokenHash string) (models.User, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
}

// PostgresUserRepository implements UserRepository backed by PostgreSQL / Supabase.
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a PostgresUserRepository using the given pgx connection pool.
func NewUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) CreateUser(ctx context.Context, email, passwordHash, firstName, lastName string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)

	var u models.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, first_name, last_name, role, is_active, password_hash, created_at, updated_at
	`, email, passwordHash, firstName, lastName).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.User{}, models.ErrAlreadyExists
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, first_name, last_name, role, is_active, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, models.ErrNotFound
		}
		return models.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, userID string) (models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, first_name, last_name, role, is_active, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, models.ErrNotFound
		}
		return models.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *PostgresUserRepository) SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) ConsumeRefreshToken(ctx context.Context, tokenHash string) (models.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		DELETE FROM refresh_tokens
		WHERE token_hash = $1
		RETURNING user_id, expires_at
	`, tokenHash).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, models.ErrUnauthorized
		}
		return models.User{}, fmt.Errorf("delete refresh token: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		_ = tx.Commit(ctx)
		return models.User{}, models.ErrUnauthorized
	}

	var u models.User
	err = tx.QueryRow(ctx, `
		SELECT id, email, first_name, last_name, role, is_active, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return models.User{}, fmt.Errorf("fetch user for refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.User{}, fmt.Errorf("commit consume refresh token: %w", err)
	}

	return u, nil
}

func (r *PostgresUserRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	return err
}
