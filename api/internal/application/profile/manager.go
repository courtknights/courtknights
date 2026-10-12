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

	// Update applies a JSON Merge Patch (ADR-015) to the user's profile.
	// It loads the stored profile and applies the location rules: a changed or
	// cleared country clears a region the patch does not set, and a region is
	// validated against the effective country (the patched one, otherwise the
	// stored one). An invalid location returns an error wrapping
	// ckerrors.ErrInvalidLocation without calling the repository; that error is
	// returned unprefixed so its message is safe to show to clients.
	// An empty patch returns the stored profile without writing.
	// ckerrors.ErrProfileNotFound and ckerrors.ErrProfileConflict are returned wrapped.
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

// EnsureExists is called by AuthManager on every successful login (OAuthCallback,
// DevicePoll, BootstrapAdmin) after the user row is resolved or created. It
// delegates to the repository which issues an INSERT ... ON CONFLICT DO NOTHING,
// so on the first login a profile row is created and on subsequent logins the
// call is a no-op.
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
	stored, err := m.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile manager: Update: %w", err)
	}
	if patch.IsEmpty() {
		return stored, nil
	}

	patch, err = applyLocationRules(stored, patch)
	if err != nil {
		return nil, err
	}

	p, err := m.profiles.Update(ctx, userID, patch)
	if err != nil {
		return nil, fmt.Errorf("profile manager: Update: %w", err)
	}
	return p, nil
}

// applyLocationRules keeps country and region consistent
// (FEATURE_clearable_profile_fields, 02_architecture.md → Location rules):
//
//  1. a country with a value must be a valid ISO 3166-1 alpha-2 code;
//  2. a country that changes or is cleared, with no region in the patch,
//     clears the stored region; the same country keeps it;
//  3. a region with a value must belong to the effective country (the
//     patched country if present, otherwise the stored one);
//  4. a region with no effective country is rejected.
func applyLocationRules(stored *profile.Profile, patch profile.ProfilePatch) (profile.ProfilePatch, error) {
	country := stored.Country
	if patch.Country.Set {
		country = patch.Country.Value
		if !patch.Region.Set && !sameValue(stored.Country, patch.Country.Value) {
			patch.Region = profile.Cleared[string]()
		}
	}

	switch {
	case patch.Region.Value != nil:
		if err := profile.ValidateLocation(country, patch.Region.Value); err != nil {
			return profile.ProfilePatch{}, err
		}
	case patch.Country.Value != nil:
		if err := profile.ValidateLocation(patch.Country.Value, nil); err != nil {
			return profile.ProfilePatch{}, err
		}
	}
	return patch, nil
}

// sameValue reports whether two optional strings hold the same value.
func sameValue(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (m *profileManager) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error) {
	items, total, err := m.profiles.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("profile manager: List: %w", err)
	}
	return items, total, nil
}
