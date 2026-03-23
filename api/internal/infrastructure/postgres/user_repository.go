// Package postgres provides PostgreSQL implementations of the domain repositories.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// UserRepository implements user.UserRepository using PostgreSQL.
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository returns a UserRepository backed by the given connection pool.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID implements user.UserRepository.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	const q = `
		SELECT id, email, name, role, provider, provider_id, created_at, updated_at
		FROM users
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: FindByID: %w", ckerrors.ErrUserNotFound)
		}
		return nil, fmt.Errorf("postgres: FindByID: %w", err)
	}
	return u, nil
}

// FindByProvider implements user.UserRepository.
func (r *UserRepository) FindByProvider(ctx context.Context, provider user.Provider, providerID string) (*user.User, error) {
	const q = `
		SELECT id, email, name, role, provider, provider_id, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2`

	row := r.db.QueryRow(ctx, q, provider, providerID)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: FindByProvider: %w", ckerrors.ErrUserNotFound)
		}
		return nil, fmt.Errorf("postgres: FindByProvider: %w", err)
	}
	return u, nil
}

// Upsert implements user.UserRepository.
// Creates the user on first call; updates name, email, and updated_at on subsequent calls
// for the same (provider, provider_id) pair.
func (r *UserRepository) Upsert(ctx context.Context, u *user.User) (*user.User, error) {
	const q = `
		INSERT INTO users (email, name, role, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_id)
		DO UPDATE SET
			email      = EXCLUDED.email,
			name       = EXCLUDED.name,
			updated_at = NOW()
		RETURNING id, email, name, role, provider, provider_id, created_at, updated_at`

	row := r.db.QueryRow(ctx, q, u.Email, u.Name, u.Role, u.Provider, u.ProviderID)
	result, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: Upsert: %w", err)
	}
	return result, nil
}

// CountAll implements user.UserRepository.
func (r *UserRepository) CountAll(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: CountAll: %w", err)
	}
	return n, nil
}

// UpdateRole implements user.UserRepository.
func (r *UserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role user.Role) error {
	const q = `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`

	tag, err := r.db.Exec(ctx, q, role, id)
	if err != nil {
		return fmt.Errorf("postgres: UpdateRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: UpdateRole: %w", ckerrors.ErrUserNotFound)
	}
	return nil
}

// scanUser reads a user row from a pgx.Row.
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.Role,
		&u.Provider,
		&u.ProviderID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return &u, nil
}
