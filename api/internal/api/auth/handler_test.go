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
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
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
func (m *mockAuthManager) UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error {
	return m.Called(ctx, userID, role).Error(0)
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

// #131: a login without a provider-verified email redirects with an error code.
func TestOAuthCallback_EmailNotVerified_RedirectsWithError(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("OAuthCallback", mock.Anything, user.ProviderGitHub, "code").
		Return("", fmt.Errorf("auth manager: OAuthCallback: github: %w", oauth2infra.ErrEmailNotVerified))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=code", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("github")

	require.NoError(t, h.oauthCallback(c))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/?error=email_not_verified", rec.Header().Get("Location"))
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

// Device-flow poll outcomes map to RFC 8628 error codes (#128).
func TestDeviceToken_ProtocolErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"slow_down", oauth2infra.ErrSlowDown, http.StatusAccepted, "slow_down"},
		{"access_denied", oauth2infra.ErrAccessDenied, http.StatusBadRequest, "access_denied"},
		{"expired_token", oauth2infra.ErrDeviceCodeExpired, http.StatusBadRequest, "expired_token"},
		{"invalid device code", oauth2infra.ErrInvalidDeviceCode, http.StatusBadRequest, "invalid_grant"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := &mockAuthManager{}
			mgr.On("DevicePoll", mock.Anything, user.ProviderGoogle, "dev-code").
				Return("", fmt.Errorf("auth manager: DevicePoll: oauth manager: device poll: google: device poll: %w", tc.err))

			h, e := newTestHandler(mgr)
			req := httptest.NewRequest(http.MethodPost, "/auth/device/token",
				strings.NewReader(`{"provider":"google","device_code":"dev-code"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			require.NoError(t, h.deviceToken(e.NewContext(req, rec)))
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.JSONEq(t, `{"error":"`+tc.wantCode+`"}`, rec.Body.String())
		})
	}
}

// #131: a device login without a provider-verified email is rejected with 400.
func TestDeviceToken_EmailNotVerified_Returns400(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DevicePoll", mock.Anything, user.ProviderGoogle, "dev-code").
		Return("", fmt.Errorf("google: %w", oauth2infra.ErrEmailNotVerified))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token",
		strings.NewReader(`{"provider":"google","device_code":"dev-code"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	require.NoError(t, h.deviceToken(e.NewContext(req, rec)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"email_not_verified"}`, rec.Body.String())
}

func TestDeviceToken_ProviderNotConfigured_Returns501(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DevicePoll", mock.Anything, user.ProviderGitHub, "dev-code").
		Return("", fmt.Errorf("oauth manager: %w", appauth.ErrProviderNotConfigured))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token",
		strings.NewReader(`{"provider":"github","device_code":"dev-code"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	err := h.deviceToken(e.NewContext(req, httptest.NewRecorder()))
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusNotImplemented, he.Code)
}

func TestDeviceToken_OtherError_Returns401(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DevicePoll", mock.Anything, user.ProviderGitHub, "dev-code").
		Return("", errors.New("github: userinfo: unexpected status 500"))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token",
		strings.NewReader(`{"provider":"github","device_code":"dev-code"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	err := h.deviceToken(e.NewContext(req, httptest.NewRecorder()))
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestDeviceAuth_ResponseUsesSnakeCaseFields(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("DeviceInit", mock.Anything, user.ProviderGoogle).
		Return(&oauth2infra.DeviceAuthResponse{
			DeviceCode: "dev-code", UserCode: "ABCD-1234",
			VerificationURI: "https://www.google.com/device", ExpiresIn: 1800, Interval: 5,
		}, nil)

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodPost, "/auth/device", strings.NewReader(`{"provider":"google"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	require.NoError(t, h.deviceAuth(e.NewContext(req, rec)))
	assert.JSONEq(t, `{"device_code":"dev-code","user_code":"ABCD-1234",
		"verification_uri":"https://www.google.com/device","expires_in":1800,"interval":5}`, rec.Body.String())
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
		Return("", fmt.Errorf("auth manager: RefreshJWT: jwt: validate: %w", ckerrors.ErrUnauthorized))

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

// #134: a refresh that fails for a reason other than an invalid token or a
// missing user (e.g. the database is down) is a server error, not a 401.
func TestRefreshJWT_InternalError_Returns500(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("RefreshJWT", mock.Anything, "valid-jwt").
		Return("", errors.New("auth manager: RefreshJWT: user manager: FindByID: connection refused"))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"token":"valid-jwt"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	err := h.refreshJWT(e.NewContext(req, httptest.NewRecorder()))
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusInternalServerError, he.Code)
}
