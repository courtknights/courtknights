package jwt

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// testAdapter returns an Adapter configured for tests (short expiry allowed).
func testAdapter(t *testing.T, expiry time.Duration) *Adapter {
	t.Helper()
	a, err := NewWithConfig("test-secret-key-for-unit-tests", expiry)
	require.NoError(t, err)
	return a
}

func TestSign_Validate_RoundTrip(t *testing.T) {
	a := testAdapter(t, time.Hour)
	u := &user.User{
		ID:    uuid.New(),
		Email: "alice@example.com",
		Role:  user.RoleAdmin,
	}

	token, err := a.Sign(u)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := a.Validate(token)
	require.NoError(t, err)
	assert.Equal(t, u.ID.String(), claims.Subject)
	assert.Equal(t, u.Email, claims.Email)
	assert.Equal(t, u.Role, claims.Role)
}

func TestValidate_ExpiredToken(t *testing.T) {
	// Use negative expiry so the token is immediately expired.
	a := testAdapter(t, -time.Second)
	u := &user.User{
		ID:    uuid.New(),
		Email: "bob@example.com",
		Role:  user.RoleUser,
	}

	token, err := a.Sign(u)
	require.NoError(t, err)

	_, err = a.Validate(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrUnauthorized)
}

func TestValidate_TamperedSignature(t *testing.T) {
	a := testAdapter(t, time.Hour)
	u := &user.User{
		ID:    uuid.New(),
		Email: "charlie@example.com",
		Role:  user.RoleUser,
	}

	token, err := a.Sign(u)
	require.NoError(t, err)

	// Tamper with the signature by corrupting a character in the middle of
	// the signature segment. Modifying the last character is unreliable
	// because base64url trailing characters may carry only padding bits
	// that the parser ignores, making the decoded bytes identical.
	lastDot := strings.LastIndex(token, ".")
	sigMid := lastDot + 1 + (len(token)-lastDot-1)/2
	replacement := byte('X')
	if token[sigMid] == 'X' {
		replacement = 'Y'
	}
	tampered := token[:sigMid] + string(replacement) + token[sigMid+1:]

	_, err = a.Validate(tampered)
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrUnauthorized)
}

func TestNew_EmptySecret(t *testing.T) {
	_, err := NewWithConfig("", time.Hour)
	require.Error(t, err)
}

func TestSign_ClaimsContent(t *testing.T) {
	a := testAdapter(t, 2*time.Hour)
	u := &user.User{
		ID:    uuid.New(),
		Email: "diana@example.com",
		Role:  user.RoleUser,
	}

	token, err := a.Sign(u)
	require.NoError(t, err)

	claims, err := a.Validate(token)
	require.NoError(t, err)

	// Expiry should be approximately now + 2h.
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), claims.ExpiresAt.Time, 5*time.Second)
}
