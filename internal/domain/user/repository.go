package user

import (
	"context"

	"github.com/google/uuid"
)

// UserRepository defines the persistence operations for the User entity.
// Implementations live in internal/infrastructure/postgres.
type UserRepository interface {
	// FindByID returns the user with the given UUID.
	// Returns ckerrors.ErrUserNotFound if no user exists with that ID.
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	// FindByProvider returns the user that matches the given provider and provider-scoped ID.
	// Returns ckerrors.ErrUserNotFound if no matching user exists.
	FindByProvider(ctx context.Context, provider Provider, providerID string) (*User, error)

	// Upsert creates the user if no row exists for (provider, provider_id),
	// or updates name, email, and updated_at if it does.
	// Returns the persisted User (with ID and timestamps populated).
	Upsert(ctx context.Context, u *User) (*User, error)

	// UpdateRole sets the role for the user with the given ID.
	// Returns ckerrors.ErrUserNotFound if no user exists with that ID.
	UpdateRole(ctx context.Context, id uuid.UUID, role Role) error
}
