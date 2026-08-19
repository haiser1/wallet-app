package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
)

// UserRepository handles data access for users.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a new user and their wallet atomically.
func (r *UserRepository) Create(ctx context.Context, req domain.CreateUserRequest) (*domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var user domain.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (username, email) VALUES ($1, $2)
		 RETURNING id, username, email, created_at`,
		req.Username, req.Email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, appErrors.ErrDuplicateUser
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Create wallet for the user
	_, err = tx.Exec(ctx,
		`INSERT INTO wallets (user_id) VALUES ($1)`,
		user.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert wallet: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, email, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}
