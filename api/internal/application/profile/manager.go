// Package profile provides the application layer for profile management.
// It implements business logic that sits between the HTTP layer and the
// domain/infrastructure layers.
package profile

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/courtknights/courtknights/internal/domain/profile"
)

// ProfileManager is the entry point for all profile-related operations.
// It validates input, delegates persistence to the repository, and returns
// domain objects to the HTTP layer.
type ProfileManager interface {
	// EnsureExists creates a profile for the given user if one does not already exist.
	// It is idempotent and safe to call on every login.
	EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error

	// GetByUserID returns the profile for the given user.
	// Returns ckerrors.ErrProfileNotFound if no profile exists.
	GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error)

	// Update validates the patch and applies it to the user's profile.
	// If Country or Region are non-nil, location codes are validated before
	// the repository is called. Returns a validation error without calling
	// the repository if any code is invalid.
	Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error)

	// List returns a paginated list of profiles joined with basic user data.
	List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error)
}

type profileManager struct {
	profiles profile.ProfileRepository
}

// NewProfileManager returns a ProfileManager backed by the given repository.
func NewProfileManager(repo profile.ProfileRepository) ProfileManager {
	return &profileManager{profiles: repo}
}

func (m *profileManager) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error {
	if err := m.profiles.EnsureExists(ctx, userID, displayName); err != nil {
		return fmt.Errorf("profile manager: EnsureExists: %w", err)
	}
	return nil
}

func (m *profileManager) GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error) {
	p, err := m.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile manager: GetByUserID: %w", err)
	}
	return p, nil
}

func (m *profileManager) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error) {
	if patch.Country != nil || patch.Region != nil {
		if err := profile.ValidateLocation(patch.Country, patch.Region); err != nil {
			return nil, fmt.Errorf("profile manager: Update: %w", err)
		}
	}

	p, err := m.profiles.Update(ctx, userID, patch)
	if err != nil {
		return nil, fmt.Errorf("profile manager: Update: %w", err)
	}
	return p, nil
}

func (m *profileManager) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error) {
	items, total, err := m.profiles.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("profile manager: List: %w", err)
	}
	return items, total, nil
}
