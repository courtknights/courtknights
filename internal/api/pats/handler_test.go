package pats

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

func contextWithRole(e *echo.Echo, method, path string, body string, role string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(common.ContextKeyRole, string(role))
	return c, rec
}

// ---- POST /api/v1/pats ----

func TestCreatePAT_AsAdmin_Returns201WithToken(t *testing.T) {
	userID := uuid.New()
	mgr := &mockAuthManager{}
	mgr.On("CreatePAT", mock.Anything, userID, (*time.Time)(nil)).
		Return("raw-pat-key", nil)

	h, e := newTestHandler(mgr)
	body := `{"user_id":"` + userID.String() + `"}`
	c, rec := contextWithRole(e, http.MethodPost, "/api/v1/pats", body, string(user.RoleAdmin))

	err := h.createPAT(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "raw-pat-key")
}

func TestCreatePAT_AsNonAdmin_Returns403(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	body := `{"user_id":"` + uuid.New().String() + `"}`
	c, rec := contextWithRole(e, http.MethodPost, "/api/v1/pats", body, string(user.RoleUser))

	err := h.createPAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	_ = rec
}

func TestCreatePAT_MissingUserID_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	c, rec := contextWithRole(e, http.MethodPost, "/api/v1/pats", `{}`, string(user.RoleAdmin))

	err := h.createPAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}

func TestCreatePAT_InvalidUserID_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	c, rec := contextWithRole(e, http.MethodPost, "/api/v1/pats", `{"user_id":"not-a-uuid"}`, string(user.RoleAdmin))

	err := h.createPAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}

// ---- GET /api/v1/pats ----

func TestListPATs_Returns200WithPATList(t *testing.T) {
	now := time.Now()
	pats := []*pat.PAT{
		{ID: uuid.New(), KeyHash: "hash1", Salt: "salt1", CreatedAt: now},
	}
	mgr := &mockAuthManager{}
	mgr.On("ListPATs", mock.Anything).Return(pats, nil)

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(common.ContextKeyRole, string(user.RoleUser))

	err := h.listPATs(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListPATs_ManagerError_Returns500(t *testing.T) {
	mgr := &mockAuthManager{}
	mgr.On("ListPATs", mock.Anything).Return(nil, errors.New("db error"))

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.listPATs(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusInternalServerError, he.Code)
	_ = rec
}

// ---- DELETE /api/v1/pats/:id ----

func TestRevokePAT_AsAdmin_Returns204(t *testing.T) {
	patID := uuid.New()
	mgr := &mockAuthManager{}
	mgr.On("RevokePAT", mock.Anything, patID).Return(nil)

	h, e := newTestHandler(mgr)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pats/"+patID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(patID.String())
	c.Set(common.ContextKeyRole, string(user.RoleAdmin))

	err := h.revokePAT(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRevokePAT_AsNonAdmin_Returns403(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	patID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pats/"+patID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(patID.String())
	c.Set(common.ContextKeyRole, string(user.RoleUser))

	err := h.revokePAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	_ = rec
}

func TestRevokePAT_InvalidID_Returns400(t *testing.T) {
	h, e := newTestHandler(&mockAuthManager{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pats/bad-id", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("bad-id")
	c.Set(common.ContextKeyRole, string(user.RoleAdmin))

	err := h.revokePAT(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}
