package users

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

	"github.com/courtknights/courtknights/internal/api/common"
	appprofile "github.com/courtknights/courtknights/internal/application/profile"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// ---- mock ProfileManager ----

type mockProfileManager struct{ mock.Mock }

func (m *mockProfileManager) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error {
	return m.Called(ctx, userID, displayName).Error(0)
}

func (m *mockProfileManager) GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*profile.Profile), args.Error(1)
}

func (m *mockProfileManager) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error) {
	args := m.Called(ctx, userID, patch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*profile.Profile), args.Error(1)
}

func (m *mockProfileManager) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*profile.ProfileListItem), args.Get(1).(int64), args.Error(2)
}

// compile-time check
var _ appprofile.ProfileManager = (*mockProfileManager)(nil)

// ---- helpers ----

func newProfileTestHandler(profiles appprofile.ProfileManager) (*Handler, *echo.Echo) {
	return NewHandler(&mockAuthManager{}, profiles), echo.New()
}

func contextWithAuthAndSub(e *echo.Echo, method, target, body, sub string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(common.ContextKeySub, sub)
	c.Set(common.ContextKeyEmail, "user@example.com")
	c.Set(common.ContextKeyRole, "user")
	return c, rec
}

func buildTestProfile(userID uuid.UUID) *profile.Profile {
	city := "Madrid"
	region := "ES-MD"
	country := "ES"
	gender := profile.GenderMale
	dob := time.Date(1990, 5, 14, 0, 0, 0, 0, time.UTC)
	category := profile.CategoryThird
	cs := profile.CourtSideDrive
	hand := profile.HandednessRight

	return &profile.Profile{
		UserID:      userID,
		DisplayName: "Álvaro Agea",
		City:        &city,
		Region:      &region,
		Country:     &country,
		Gender:      &gender,
		DateOfBirth: &dob,
		Category:    &category,
		Preferences: profile.Preferences{
			CourtSide:  &cs,
			Handedness: &hand,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

// ---- GET /users/me/profile ----

// HP-01: Returns own profile with 200.
func TestGetMyProfile_ReturnsProfile(t *testing.T) {
	userID := uuid.New()
	p := buildTestProfile(userID)
	mgr := &mockProfileManager{}
	mgr.On("GetByUserID", mock.Anything, userID).Return(p, nil)

	h, e := newProfileTestHandler(mgr)
	c, rec := contextWithAuthAndSub(e, http.MethodGet, "/api/v1/users/me/profile", "", userID.String())

	err := h.getMyProfile(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, userID.String())
	assert.Contains(t, body, "Álvaro Agea")
	assert.Contains(t, body, "Madrid")
	mgr.AssertExpectations(t)
}

// HP-02: Profile not found returns 404.
func TestGetMyProfile_ProfileNotFound_Returns404(t *testing.T) {
	userID := uuid.New()
	mgr := &mockProfileManager{}
	mgr.On("GetByUserID", mock.Anything, userID).Return(nil, ckerrors.ErrProfileNotFound)

	h, e := newProfileTestHandler(mgr)
	c, _ := contextWithAuthAndSub(e, http.MethodGet, "/api/v1/users/me/profile", "", userID.String())

	err := h.getMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusNotFound, he.Code)
	mgr.AssertExpectations(t)
}

// HP-03: Missing / invalid sub claim returns 400.
func TestGetMyProfile_InvalidSub_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newProfileTestHandler(mgr)
	c, _ := contextWithAuthAndSub(e, http.MethodGet, "/api/v1/users/me/profile", "", "not-a-uuid")

	err := h.getMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// HP-03b: Sub claim absent returns 400.
func TestGetMyProfile_MissingSub_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newProfileTestHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/profile", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// sub not set in context

	err := h.getMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// ---- PUT /users/me/profile ----

// HP-04: Partial update — display_name only returns 200 with updated profile.
func TestUpdateMyProfile_DisplayNameOnly_Returns200(t *testing.T) {
	userID := uuid.New()
	newName := "X"
	p := buildTestProfile(userID)
	p.DisplayName = newName

	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.MatchedBy(func(patch profile.ProfilePatch) bool {
		return patch.DisplayName != nil && *patch.DisplayName == newName
	})).Return(p, nil)

	h, e := newProfileTestHandler(mgr)
	c, rec := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile",
		`{"display_name":"X"}`, userID.String())

	err := h.updateMyProfile(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), newName)
	mgr.AssertExpectations(t)
}

// HP-05: Full update — all fields present returns 200.
func TestUpdateMyProfile_AllFields_Returns200(t *testing.T) {
	userID := uuid.New()
	p := buildTestProfile(userID)
	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.Anything).Return(p, nil)

	body := `{
		"display_name": "Álvaro",
		"city": "Madrid",
		"region": "ES-MD",
		"country": "ES",
		"gender": "male",
		"date_of_birth": "1990-05-14",
		"category": "third",
		"preferences": {"court_side": "drive", "handedness": "right"}
	}`

	h, e := newProfileTestHandler(mgr)
	c, rec := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile", body, userID.String())

	err := h.updateMyProfile(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mgr.AssertExpectations(t)
}

// HP-06: Invalid country code — manager returns validation error → 400.
func TestUpdateMyProfile_InvalidCountry_Returns400(t *testing.T) {
	userID := uuid.New()
	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.Anything).Return(nil, errors.New("invalid country code"))

	h, e := newProfileTestHandler(mgr)
	c, _ := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile",
		`{"country":"XX"}`, userID.String())

	err := h.updateMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertExpectations(t)
}

// HP-07: Region without country — manager returns validation error → 400.
func TestUpdateMyProfile_RegionWithoutCountry_Returns400(t *testing.T) {
	userID := uuid.New()
	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.Anything).Return(nil, errors.New("region requires country"))

	h, e := newProfileTestHandler(mgr)
	c, _ := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile",
		`{"region":"ES-MD"}`, userID.String())

	err := h.updateMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertExpectations(t)
}

// HP-08: Unknown enum value for gender — handler rejects before calling manager → 400.
func TestUpdateMyProfile_InvalidGender_Returns400(t *testing.T) {
	userID := uuid.New()
	mgr := &mockProfileManager{}

	h, e := newProfileTestHandler(mgr)
	c, _ := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile",
		`{"gender":"unknown"}`, userID.String())

	err := h.updateMyProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	// Manager should not be called for invalid enum.
	mgr.AssertNotCalled(t, "Update")
}

// HP-09: Empty body — manager called with all-nil patch; returns existing profile → 200.
func TestUpdateMyProfile_EmptyBody_Returns200(t *testing.T) {
	userID := uuid.New()
	p := buildTestProfile(userID)
	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.MatchedBy(func(patch profile.ProfilePatch) bool {
		return patch.DisplayName == nil && patch.Country == nil && patch.Region == nil &&
			patch.Gender == nil && patch.Category == nil
	})).Return(p, nil)

	h, e := newProfileTestHandler(mgr)
	c, rec := contextWithAuthAndSub(e, http.MethodPut, "/api/v1/users/me/profile", `{}`, userID.String())

	err := h.updateMyProfile(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mgr.AssertExpectations(t)
}

// ---- GET /users/:id/profile ----

// HP-10: Returns profile for a valid user ID → 200.
func TestGetUserProfile_ValidID_Returns200(t *testing.T) {
	userID := uuid.New()
	p := buildTestProfile(userID)
	mgr := &mockProfileManager{}
	mgr.On("GetByUserID", mock.Anything, userID).Return(p, nil)

	h, e := newProfileTestHandler(mgr)
	requesterID := uuid.New().String()
	c, rec := contextWithAuthAndSub(e, http.MethodGet, fmt.Sprintf("/api/v1/users/%s/profile", userID), "", requesterID)
	c.SetParamNames("id")
	c.SetParamValues(userID.String())

	err := h.getUserProfile(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), userID.String())
	mgr.AssertExpectations(t)
}

// HP-11: Profile not found → 404.
func TestGetUserProfile_ProfileNotFound_Returns404(t *testing.T) {
	userID := uuid.New()
	mgr := &mockProfileManager{}
	mgr.On("GetByUserID", mock.Anything, userID).Return(nil, ckerrors.ErrProfileNotFound)

	h, e := newProfileTestHandler(mgr)
	requesterID := uuid.New().String()
	c, _ := contextWithAuthAndSub(e, http.MethodGet, fmt.Sprintf("/api/v1/users/%s/profile", userID), "", requesterID)
	c.SetParamNames("id")
	c.SetParamValues(userID.String())

	err := h.getUserProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusNotFound, he.Code)
	mgr.AssertExpectations(t)
}

// HP-12: Malformed UUID in path → 400.
func TestGetUserProfile_MalformedUUID_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newProfileTestHandler(mgr)
	requesterID := uuid.New().String()
	c, _ := contextWithAuthAndSub(e, http.MethodGet, "/api/v1/users/not-a-uuid/profile", "", requesterID)
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")

	err := h.getUserProfile(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertNotCalled(t, "GetByUserID")
}
