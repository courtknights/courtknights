package auth

import "github.com/google/uuid"

// parseUUID is a convenience wrapper around uuid.Parse.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
