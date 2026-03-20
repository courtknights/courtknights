package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appauth "github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// ---- mock AuthManager ----

type mockAuthManager struct{ mock.Mock }

func (m *mockAuthManager) OAuthRedirectURL(provider user.Provider, state string) (string, error) {
	args := m.Called(provider, state)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error) {
	args := m.Called(ctx, provider, code)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	args := m.Called(ctx, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2infra.DeviceAuthResponse), args.Error(1)
}
func (m *mockAuthManager) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error) {
	args := m.Called(ctx, provider, deviceCode)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) ExchangePAT(ctx context.Context, rawPAT string) (string, error) {
	args := m.Called(ctx, rawPAT)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) RefreshJWT(ctx context.Context, tokenStr string) (string, error) {
	args := m.Called(ctx, tokenStr)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error) {
	args := m.Called(ctx, userID, expiresAt)
	return args.String(0), args.Error(1)
}
func (m *mockAuthManager) RevokePAT(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockAuthManager) ListPATs(ctx context.Context) ([]*pat.PAT, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*pat.PAT), args.Error(1)
}
func (m *mockAuthManager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error) {
	args := m.Called(ctx, email, name, rawPAT)
	return args.String(0), args.Error(1)
}

// ---- helpers ----

func newTestHandler(mgr appauth.AuthManager) (*Handler, *echo.Echo) {
	h := NewHandler(mgr)
	e := echo.New()
	return h, e
}

func performRequest(e *echo.Echo, method, path string, handlerFunc echo.HandlerFunc, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	for k, v := range params {
		c.SetParamNames(k)
		c.SetParamValues(v)
	}
	_ = handlerFunc(c)
	return rec
}

// ---- GET /auth/:provider ----

func TestRedirectToProvider_Google_Returns302WithGoogleURL(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthRedirectURL", user.ProviderGoogle, "ck-state").
		Return("https://accounts.google.com/o/oauth2/auth?state=ck-state", nil)

	h, e := newTestHandler(mgr)
	rec := performRequest(e, http.MethodGet, "/auth/google", h.redirectToProvider, map[string]string{"provider": "google"})

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "accounts.google.com")
}

func TestRedirectToProvider_GitHub_Returns302WithGitHubURL(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthRedirectURL", user.ProviderGitHub, "ck-state").
		Return("https://github.com/login/oauth/authorize?state=ck-state", nil)

	h, e := newTestHandler(mgr)
	rec := performRequest(e, http.MethodGet, "/auth/github", h.redirectToProvider, map[string]string{"provider": "github"})

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "github.com")
}

func TestRedirectToProvider_UnknownProvider_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodGet, "/auth/unknown", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("unknown")

	err := h.redirectToProvider(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestRedirectToProvider_ProviderNotConfigured_Returns501(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthRedirectURL", user.ProviderGoogle, "ck-state").
		Return("", appauth.ErrProviderNotConfigured)

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("google")

	err := h.redirectToProvider(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusNotImplemented, he.Code)
}

// ---- GET /auth/:provider/callback ----

func TestOAuthCallback_ValidCode_Returns302WithToken(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthCallback", mock.Anything, user.ProviderGoogle, "valid-code").
		Return("signed-jwt", nil)

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=valid-code", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("google")

	err := h.oauthCallback(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "signed-jwt")
}

func TestOAuthCallback_InvalidCode_Returns401(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthCallback", mock.Anything, user.ProviderGoogle, "bad-code").
		Return("", errors.New("invalid_grant"))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=bad-code", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("google")

	err := h.oauthCallback(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestOAuthCallback_MissingCode_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("google")

	err := h.oauthCallback(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// ---- POST /auth/device ----

func TestDeviceAuth_ValidProvider_Returns200WithUserCode(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DeviceInit", mock.Anything, user.ProviderGitHub).
		Return(&oauth2infra.DeviceAuthResponse{
			DeviceCode: "dev-code", UserCode: "ABCD-1234",
			VerificationURI: "https://github.com/login/device",
		}, nil)

	h, e := newTestHandler(mgr)
	body := `{"provider":"github"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.deviceAuth(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ABCD-1234")
}

func TestDeviceAuth_MissingProvider_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodPost, "/auth/device", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.deviceAuth(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// ---- POST /auth/device/token ----

func TestDeviceToken_Pending_Returns202WithAuthorizationPending(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DevicePoll", mock.Anything, user.ProviderGitHub, "dev-code").
		Return("", fmt.Errorf("%w", oauth2infra.ErrAuthorizationPending))

	h, e := newTestHandler(mgr)
	body := `{"provider":"github","device_code":"dev-code"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.deviceToken(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Contains(t, rec.Body.String(), "authorization_pending")
}

func TestDeviceToken_Authorised_Returns200WithJWT(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DevicePoll", mock.Anything, user.ProviderGitHub, "dev-code").
		Return("device-jwt", nil)

	h, e := newTestHandler(mgr)
	body := `{"provider":"github","device_code":"dev-code"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.deviceToken(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "device-jwt")
}

func TestDeviceToken_MissingFields_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token", strings.NewReader(`{"provider":"github"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.deviceToken(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// ---- POST /auth/token/pat ----

func TestExchangePAT_ValidToken_Returns200WithJWT(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("ExchangePAT", mock.Anything, "raw-pat-token").
		Return("signed-jwt", nil)

	h, e := newTestHandler(mgr)
	body := `{"token":"raw-pat-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token/pat", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.exchangePAT(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "signed-jwt")
}

func TestExchangePAT_InvalidToken_Returns401(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("ExchangePAT", mock.Anything, "bad-pat").
		Return("", errors.New("invalid token"))

	h, e := newTestHandler(mgr)
	body := `{"token":"bad-pat"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token/pat", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.exchangePAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestExchangePAT_MissingToken_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodPost, "/auth/token/pat", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.exchangePAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// ---- POST /auth/refresh ----

func TestRefreshJWT_ValidToken_Returns200WithNewJWT(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("RefreshJWT", mock.Anything, "old-jwt").
		Return("new-jwt", nil)

	h, e := newTestHandler(mgr)
	body := `{"token":"old-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.refreshJWT(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "new-jwt")
}

func TestRefreshJWT_ExpiredToken_Returns401(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("RefreshJWT", mock.Anything, "expired-jwt").
		Return("", errors.New("token expired"))

	h, e := newTestHandler(mgr)
	body := `{"token":"expired-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.refreshJWT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestRefreshJWT_MissingToken_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.refreshJWT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}
