package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

// parseUUID is a convenience wrapper around uuid.Parse.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// hashKey returns the SHA-256 hex digest of key+salt.
func hashKey(key, salt string) string {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:])
}

// generateRawKey generates a cryptographically random 32-byte hex string.
func generateRawKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSalt generates a cryptographically random 16-byte hex salt.
func generateSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
