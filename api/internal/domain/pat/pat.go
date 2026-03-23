// Package pat defines the PersonalAccessToken domain entity.
package pat

import (
	"time"

	"github.com/google/uuid"
)

// PAT represents a Personal Access Token used to authenticate without OAuth2.
// The raw token value is never stored; only the hash and salt are persisted.
type PAT struct {
	// ID is the internal UUID primary key.
	ID uuid.UUID
	// KeyHash is the hashed representation of the raw token.
	KeyHash string
	// Salt is the random salt used when hashing the raw token.
	Salt string
	// ExpiresAt is the optional expiry time of the token.
	// A nil value means the token never expires.
	ExpiresAt *time.Time
	// CreatedAt is the timestamp when the PAT was created.
	CreatedAt time.Time
}

// IsExpired reports whether the PAT has passed its expiry time.
// A PAT without an expiry (ExpiresAt == nil) is never considered expired.
func (p *PAT) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*p.ExpiresAt)
}
