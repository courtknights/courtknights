package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/user"
	jwtinfra "github.com/courtknights/courtknights/internal/infrastructure/jwt"
)

func TestJWTManager_Sign_DelegatesToAdapter(t *testing.T) {
	adapt := &mockJWTAdapter{}
	u := &user.User{ID: uuid.New(), Email: "alice@example.com", Role: user.RoleUser}
	adapt.On("Sign", u).Return("signed-token", nil)

	mgr := NewJWTManager(adapt)
	token, err := mgr.Sign(u)
	require.NoError(t, err)
	assert.Equal(t, "signed-token", token)
	adapt.AssertExpectations(t)
}

func TestJWTManager_Sign_PropagatesError(t *testing.T) {
	adapt := &mockJWTAdapter{}
	u := &user.User{}
	adapt.On("Sign", u).Return("", errors.New("signing failed"))

	mgr := NewJWTManager(adapt)
	_, err := mgr.Sign(u)
	require.Error(t, err)
}

func TestJWTManager_Validate_DelegatesToAdapter(t *testing.T) {
	adapt := &mockJWTAdapter{}
	claims := &jwtinfra.Claims{}
	claims.Subject = uuid.New().String()
	adapt.On("Validate", "valid-token").Return(claims, nil)

	mgr := NewJWTManager(adapt)
	got, err := mgr.Validate("valid-token")
	require.NoError(t, err)
	assert.Equal(t, claims.Subject, got.Subject)
}

func TestJWTManager_Validate_PropagatesError(t *testing.T) {
	adapt := &mockJWTAdapter{}
	adapt.On("Validate", "bad-token").Return(nil, errors.New("invalid"))

	mgr := NewJWTManager(adapt)
	_, err := mgr.Validate("bad-token")
	require.Error(t, err)
}
