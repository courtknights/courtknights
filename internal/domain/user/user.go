// Package user defines the User domain entity and its associated types.
package user

import (
	"time"

	"github.com/google/uuid"
)

// Role represents the access level of a user within the platform.
type Role string

const (
	// RoleAdmin grants elevated permissions (e.g. PAT management, role promotion).
	RoleAdmin Role = "admin"
	// RoleUser is the default role for regular platform users.
	RoleUser Role = "user"
)

// Provider identifies the authentication provider used to create a user account.
type Provider string

const (
	// ProviderGoogle identifies users authenticated via Google OAuth2.
	ProviderGoogle Provider = "google"
	// ProviderGitHub identifies users authenticated via GitHub OAuth2.
	ProviderGitHub Provider = "github"
	// ProviderPAT identifies users created for PAT-only authentication (e.g. bootstrap admin).
	ProviderPAT Provider = "pat"
)

// User is the core identity entity of the platform.
// Each User is linked to exactly one external identity (provider + provider_id).
type User struct {
	// ID is the internal UUID primary key.
	ID uuid.UUID
	// Email is the user's email address. Always non-null.
	Email string
	// Name is the user's display name.
	Name string
	// Role determines the user's access level.
	Role Role
	// Provider is the authentication provider that created this account.
	Provider Provider
	// ProviderID is the unique identifier within the provider's namespace.
	// For PAT users this is set to the linked PAT's UUID.
	ProviderID string
	// CreatedAt is the timestamp when the user was created.
	CreatedAt time.Time
	// UpdatedAt is the timestamp of the last update.
	UpdatedAt time.Time
}

// IsAdmin reports whether the user has the admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
