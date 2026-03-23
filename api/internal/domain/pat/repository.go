package pat

import (
	"context"

	"github.com/google/uuid"
)

// PATRepository defines the persistence operations for the PAT entity.
// Implementations live in internal/infrastructure/postgres.
type PATRepository interface {
	// FindAll returns all PATs stored in the system.
	FindAll(ctx context.Context) ([]*PAT, error)

	// FindByID returns the PAT with the given UUID.
	// Returns ckerrors.ErrPATNotFound if no PAT exists with that ID.
	FindByID(ctx context.Context, id uuid.UUID) (*PAT, error)

	// Save persists a new PAT record and returns it with the assigned ID and timestamps.
	Save(ctx context.Context, p *PAT) (*PAT, error)

	// Delete removes the PAT with the given UUID.
	// Returns ckerrors.ErrPATNotFound if no PAT exists with that ID.
	Delete(ctx context.Context, id uuid.UUID) error
}
