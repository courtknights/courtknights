// Package profile defines the Profile domain entity and its associated types.
package profile

import (
	"time"

	"github.com/google/uuid"
)

// Gender represents the user's self-declared gender, used for tournament category assignment.
type Gender string

const (
	// GenderMale identifies a male gender.
	GenderMale Gender = "male"
	// GenderFemale identifies a female gender.
	GenderFemale Gender = "female"
)

// Category represents the federative padel category, self-declared by the user.
type Category string

const (
	// CategoryFirst is the top federative category.
	CategoryFirst Category = "first"
	// CategorySecond is the second federative category.
	CategorySecond Category = "second"
	// CategoryThird is the third federative category.
	CategoryThird Category = "third"
	// CategoryFourth is the fourth federative category.
	CategoryFourth Category = "fourth"
	// CategoryFifth is the fifth federative category.
	CategoryFifth Category = "fifth"
)

// CourtSide represents the preferred side of the court the player occupies.
type CourtSide string

const (
	// CourtSideDrive indicates the player prefers the drive (right) side.
	CourtSideDrive CourtSide = "drive"
	// CourtSideBackhand indicates the player prefers the backhand (left) side.
	CourtSideBackhand CourtSide = "backhand"
	// CourtSideBoth indicates the player is comfortable on both sides.
	CourtSideBoth CourtSide = "both"
)

// Handedness represents the dominant hand used by the player.
type Handedness string

const (
	// HandednessRight indicates the player is right-handed.
	HandednessRight Handedness = "right"
	// HandednessLeft indicates the player is left-handed.
	HandednessLeft Handedness = "left"
)

// Profile is the extended identity entity for a platform user.
// It is created automatically at registration and can be updated at any time.
type Profile struct {
	// UserID is the UUID of the associated user (primary key reference).
	UserID uuid.UUID
	// DisplayName is the public name shown to other users. Pre-filled from the OAuth provider.
	DisplayName string
	// City is an optional free-text city name.
	City *string
	// Region is an optional ISO 3166-2 subdivision code (e.g. "ES-MD").
	Region *string
	// Country is an optional ISO 3166-1 alpha-2 country code (e.g. "ES").
	Country *string
	// Gender is the user's self-declared gender.
	Gender *Gender
	// DateOfBirth is used for age-group tournament eligibility.
	DateOfBirth *time.Time
	// Category is the user's self-declared federative padel category.
	Category *Category
	// Preferences holds sport-specific settings stored as JSONB.
	Preferences Preferences
	// CreatedAt is the timestamp when the profile was created.
	CreatedAt time.Time
	// UpdatedAt is the timestamp of the last profile update.
	UpdatedAt time.Time
}
