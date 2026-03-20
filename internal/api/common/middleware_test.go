package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/user"
	jwtinfra "github.com/courtknights/courtknights/internal/infrastructure/jwt"
)

// mockJWTManager implements auth.JWTManager for testing.
type mockJWTManager struct{ mock.Mock }

func (m *mockJWTManager) Sign(u *user.User) (string, error) {
	args := m.Called(u)
	return args.String(0), args.Error(1)
}

func (m *mockJWTManager) Validate(tokenStr string) (*jwtinfra.Claims, error) {
	args := m.Called(tokenStr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwtinfra.Claims), args.Error(1)
}

// handler is a test echo.HandlerFunc that records the context values it receives.
func captureHandler(sub, email, role *string) echo.HandlerFunc {
	return func(c echo.Context) error {
		if s, ok := c.Get(ContextKeySub).(string); ok {
			*sub = s
		}
		if e, ok := c.Get(ContextKeyEmail).(string); ok {
			*email = e
		}
		if r, ok := c.Get(ContextKeyRole).(string); ok {
			*role = r
		}
		return c.NoContent(http.StatusOK)
	}
}

func newEchoRequest(method, path, authHeader string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestJWTMiddleware_ValidToken_SetsContextValues(t *testing.T) {
	jwtMgr := &mockJWTManager{}
	claims := &jwtinfra.Claims{}
	claims.Subject = "user-uuid-123"
	claims.Email = "alice@example.com"
	claims.Role = user.RoleAdmin

	jwtMgr.On("Validate", "valid.token.here").Return(claims, nil)

	mw := JWTMiddleware(jwtMgr)

	var sub, email, role string
	c, rec := newEchoRequest(http.MethodGet, "/", "Bearer valid.token.here")
	err := mw(captureHandler(&sub, &email, &role))(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-uuid-123", sub)
	assert.Equal(t, "alice@example.com", email)
	assert.Equal(t, "admin", role)
}

func TestJWTMiddleware_MissingHeader_Returns401(t *testing.T) {
	mw := JWTMiddleware(&mockJWTManager{})
	c, _ := newEchoRequest(http.MethodGet, "/", "")

	err := mw(func(c echo.Context) error { return nil })(c)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestJWTMiddleware_ExpiredToken_Returns401(t *testing.T) {
	jwtMgr := &mockJWTManager{}
	jwtMgr.On("Validate", "expired.token").
		Return(nil, ckerrors.ErrUnauthorized)

	mw := JWTMiddleware(jwtMgr)
	c, _ := newEchoRequest(http.MethodGet, "/", "Bearer expired.token")

	err := mw(func(c echo.Context) error { return nil })(c)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestJWTMiddleware_TamperedToken_Returns401(t *testing.T) {
	jwtMgr := &mockJWTManager{}
	jwtMgr.On("Validate", "tampered.token").
		Return(nil, ckerrors.ErrUnauthorized)

	mw := JWTMiddleware(jwtMgr)
	c, _ := newEchoRequest(http.MethodGet, "/", "Bearer tampered.token")

	err := mw(func(c echo.Context) error { return nil })(c)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestJWTMiddleware_MalformedHeader_Returns401(t *testing.T) {
	mw := JWTMiddleware(&mockJWTManager{})
	c, _ := newEchoRequest(http.MethodGet, "/", "Token notbearer")

	err := mw(func(c echo.Context) error { return nil })(c)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}
