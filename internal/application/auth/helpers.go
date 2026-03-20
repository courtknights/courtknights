package auth

import "github.com/google/uuid"

// parseUUID is a convenience wrapper around uuid.Parse that returns the zero
// value and an error instead of panicking.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
