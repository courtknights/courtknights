// Package jwt provides JWT signing and validation for the CourtKnights API.
// It implements HS256 signing using the golang-jwt library.
// Configuration is read via Viper:
//
//	COURTKNIGHTS_JWT_SECRET  — signing secret (required)
//	COURTKNIGHTS_JWT_EXPIRY  — token lifetime, e.g. "1h" (default: 1h)
package jwt

import (
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// Claims holds the standard + CourtKnights custom claims embedded in every JWT.
type Claims struct {
	gojwt.RegisteredClaims
	// Email is the user's email address.
	Email string `json:"email"`
	// Role is the user's access level ("admin" or "user").
	Role user.Role `json:"role"`
}

// Adapter signs and validates JWTs using a shared HS256 secret.
type Adapter struct {
	secret []byte
	expiry time.Duration
}

// New returns an Adapter configured from Viper.
// It returns an error if the secret is empty.
func New() (*Adapter, error) {
	secret := viper.GetString("jwt.secret")
	if secret == "" {
		return nil, fmt.Errorf("jwt: COURTKNIGHTS_JWT_SECRET is required")
	}

	expiry := viper.GetDuration("jwt.expiry")
	if expiry == 0 {
		expiry = time.Hour
	}

	return &Adapter{
		secret: []byte(secret),
		expiry: expiry,
	}, nil
}

// NewWithConfig returns an Adapter with explicit configuration.
// Intended for use in tests.
func NewWithConfig(secret string, expiry time.Duration) (*Adapter, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt: secret must not be empty")
	}
	return &Adapter{
		secret: []byte(secret),
		expiry: expiry,
	}, nil
}

// Sign issues an HS256-signed JWT for the given user.
// The token contains claims: sub, email, role, exp.
func (a *Adapter) Sign(u *user.User) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(a.expiry)),
		},
		Email: u.Email,
		Role:  u.Role,
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secret)
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}
	return signed, nil
}

// Validate parses and verifies a JWT string.
// Returns ErrUnauthorized for any validation failure (expired, tampered, malformed).
func (a *Adapter) Validate(tokenStr string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil {
		if errors.Is(err, gojwt.ErrTokenExpired) {
			return nil, fmt.Errorf("jwt: validate: %w: %w", ckerrors.ErrUnauthorized, err)
		}
		return nil, fmt.Errorf("jwt: validate: %w: %w", ckerrors.ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt: validate: %w", ckerrors.ErrUnauthorized)
	}
	return claims, nil
}
