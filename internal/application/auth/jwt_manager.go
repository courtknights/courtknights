package auth

import (
	"fmt"

	"github.com/courtknights/courtknights/internal/domain/user"
	jwtinfra "github.com/courtknights/courtknights/internal/infrastructure/jwt"
)

// jwtAdapter is the subset of the JWT infrastructure adapter used by JWTManager.
type jwtAdapter interface {
	Sign(u *user.User) (string, error)
	Validate(tokenStr string) (*jwtinfra.Claims, error)
}

// JWTManager encapsulates all JWT business logic.
// It is the only application-layer component that talks to the JWT infrastructure adapter.
type JWTManager struct {
	adapter jwtAdapter
}

// NewJWTManager returns a JWTManager backed by the given adapter.
func NewJWTManager(adapter jwtAdapter) *JWTManager {
	return &JWTManager{adapter: adapter}
}

// Sign issues a signed JWT for the given user.
func (m *JWTManager) Sign(u *user.User) (string, error) {
	token, err := m.adapter.Sign(u)
	if err != nil {
		return "", fmt.Errorf("jwt manager: sign: %w", err)
	}
	return token, nil
}

// Validate parses and verifies a JWT string, returning its claims.
func (m *JWTManager) Validate(tokenStr string) (*jwtinfra.Claims, error) {
	claims, err := m.adapter.Validate(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("jwt manager: validate: %w", err)
	}
	return claims, nil
}
