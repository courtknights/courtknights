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

// JWTManager encapsulates all JWT operations.
// It is the only application-layer component that talks to the JWT infrastructure adapter.
type JWTManager interface {
	Sign(u *user.User) (string, error)
	Validate(tokenStr string) (*jwtinfra.Claims, error)
}

type jwtManager struct {
	adapter jwtAdapter
}

// NewJWTManager returns a JWTManager backed by the given adapter.
func NewJWTManager(adapter jwtAdapter) JWTManager {
	return &jwtManager{adapter: adapter}
}

func (m *jwtManager) Sign(u *user.User) (string, error) {
	token, err := m.adapter.Sign(u)
	if err != nil {
		return "", fmt.Errorf("jwt manager: sign: %w", err)
	}
	return token, nil
}

func (m *jwtManager) Validate(tokenStr string) (*jwtinfra.Claims, error) {
	claims, err := m.adapter.Validate(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("jwt manager: validate: %w", err)
	}
	return claims, nil
}
