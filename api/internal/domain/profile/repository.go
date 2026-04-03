package profile

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProfileRepository defines the persistence contract for Profile entities.
// Implementations live in api/internal/infrastructure/postgres/.
type ProfileRepository interface {
	// EnsureExists creates the profile with the given display name if it does not exist.
	// No-op if a profile already exists for the user. Safe to call on every login.
	EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error

	// FindByUserID returns the profile for the given user.
	// Returns ckerrors.ErrProfileNotFound if no profile exists.
	FindByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error)

	// Update applies a partial update to the profile.
	// Only non-nil fields in the patch are written.
	Update(ctx context.Context, userID uuid.UUID, patch ProfilePatch) (*Profile, error)

	// List returns a paginated list of profiles joined with basic user data.
	// It returns the items for the requested page, the total count of profiles, and any error.
	List(ctx context.Context, params ListParams) ([]*ProfileListItem, int64, error)
}

// ProfilePatch carries the fields to update. A nil pointer means "leave unchanged".
type ProfilePatch struct {
	// DisplayName is the new public name.
	DisplayName *string
	// City is the new city free-text value.
	City *string
	// Region is the new ISO 3166-2 subdivision code.
	Region *string
	// Country is the new ISO 3166-1 alpha-2 country code.
	Country *string
	// Gender is the new self-declared gender.
	Gender *Gender
	// DateOfBirth is the new date of birth.
	DateOfBirth *time.Time
	// Category is the new self-declared federative category.
	Category *Category
	// Preferences replaces the current preferences JSONB value.
	Preferences *Preferences
}

// ListParams controls pagination and field selection for the list endpoint.
type ListParams struct {
	// Page is the 1-based page number.
	Page int
	// PageSize is the maximum number of items per page.
	PageSize int
	// Fields is the subset of allowed fields to return. An empty slice means all fields.
	Fields []string
}

// ProfileListItem is a projection used by the list endpoint.
// All fields are pointers so that omitted fields serialise as null or absent.
type ProfileListItem struct {
	// ID is the user's UUID.
	ID *uuid.UUID
	// DisplayName is the user's public name.
	DisplayName *string
	// City is the user's optional city.
	City *string
	// Region is the user's optional ISO 3166-2 subdivision code.
	Region *string
	// Country is the user's optional ISO 3166-1 alpha-2 country code.
	Country *string
	// Gender is the user's optional self-declared gender.
	Gender *Gender
	// Category is the user's optional federative category.
	Category *Category
}
