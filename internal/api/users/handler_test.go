package users

import (
	"context"
	"errors"
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

	"github.com/courtknights/courtknights/internal/api/common"
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
func (m *mockAuthManager) UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error {
	return m.Called(ctx, userID, role).Error(0)
}
func (m *mockAuthManager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error) {
	args := m.Called(ctx, email, name, rawPAT)
	return args.String(0), args.Error(1)
}

// compile-time check
var _ appauth.AuthManager = (*mockAuthManager)(nil)

// ---- helpers ----

func newTestHandler(mgr appauth.AuthManager) (*Handler, *echo.Echo) {
	return NewHandler(mgr), echo.New()
}

func contextWithAuth(e *echo.Echo, method, path, body, role, sub, email string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(common.ContextKeyRole, role)
	c.Set(common.ContextKeySub, sub)
	c.Set(common.ContextKeyEmail, email)
	return c, rec
}

// ---- GET /api/v1/users/me ----

func TestMe_Returns200WithUserProfile(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	userID := uuid.New().String()
	c, rec := contextWithAuth(e, http.MethodGet, "/api/v1/users/me", "", string(user.RoleUser), userID, "alice@example.com")

	err := h.me(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), userID)
	assert.Contains(t, rec.Body.String(), "alice@example.com")
	assert.Contains(t, rec.Body.String(), string(user.RoleUser))
}

// ---- PUT /api/v1/users/:id/role ----

func TestUpdateRole_AsAdmin_Returns200(t *testing.T) {
	targetID := uuid.New()
	mgr := &mockAuthManager{}
	mgr.On("UpdateUserRole", mock.Anything, targetID, user.RoleUser).Return(nil)

	h, e := newTestHandler(mgr)
	body := `{"role":"user"}`
	c, rec := contextWithAuth(e, http.MethodPut, "/api/v1/users/"+targetID.String()+"/role", body, string(user.RoleAdmin), uuid.New().String(), "admin@example.com")
	c.SetParamNames("id")
	c.SetParamValues(targetID.String())

	err := h.updateRole(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), targetID.String())
}

func TestUpdateRole_AsNonAdmin_Returns403(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	targetID := uuid.New()
	c, rec := contextWithAuth(e, http.MethodPut, "/api/v1/users/"+targetID.String()+"/role", `{"role":"admin"}`, string(user.RoleUser), uuid.New().String(), "user@example.com")
	c.SetParamNames("id")
	c.SetParamValues(targetID.String())

	err := h.updateRole(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	_ = rec
}

func TestUpdateRole_InvalidID_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	c, rec := contextWithAuth(e, http.MethodPut, "/api/v1/users/bad-id/role", `{"role":"user"}`, string(user.RoleAdmin), uuid.New().String(), "admin@example.com")
	c.SetParamNames("id")
	c.SetParamValues("bad-id")

	err := h.updateRole(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}

func TestUpdateRole_InvalidRole_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	targetID := uuid.New()
	c, rec := contextWithAuth(e, http.MethodPut, "/api/v1/users/"+targetID.String()+"/role", `{"role":"superuser"}`, string(user.RoleAdmin), uuid.New().String(), "admin@example.com")
	c.SetParamNames("id")
	c.SetParamValues(targetID.String())

	err := h.updateRole(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}

func TestUpdateRole_ManagerError_Returns500(t *testing.T) {
	targetID := uuid.New()
	mgr := &mockAuthManager{}
	mgr.On("UpdateUserRole", mock.Anything, targetID, user.RoleUser).Return(errors.New("db error"))

	h, e := newTestHandler(mgr)
	body := `{"role":"user"}`
	c, rec := contextWithAuth(e, http.MethodPut, "/api/v1/users/"+targetID.String()+"/role", body, string(user.RoleAdmin), uuid.New().String(), "admin@example.com")
	c.SetParamNames("id")
	c.SetParamValues(targetID.String())

	err := h.updateRole(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusInternalServerError, he.Code)
	_ = rec
}
