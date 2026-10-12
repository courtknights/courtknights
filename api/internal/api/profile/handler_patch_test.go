package profile

import (
	"encoding/json"
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

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// patchProfile calls updateMyProfile with a merge-patch body and returns the
// recorder and the handler error (an *echo.HTTPError for 4xx/5xx).
func patchProfile(t *testing.T, mgr *mockProfileManager, userID uuid.UUID, body string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	h, e := newTestHandler(mgr)
	c, rec := contextWithAuthAndSub(e, http.MethodPatch, "/api/v1/users/me/profile", body, userID.String())
	c.Request().Header.Set(echo.HeaderContentType, "application/merge-patch+json")
	return rec, h.updateMyProfile(c)
}

// capturedPatch runs a PATCH that the manager accepts and returns the patch it received.
func capturedPatch(t *testing.T, body string) profile.ProfilePatch {
	t.Helper()
	userID := uuid.New()
	mgr := &mockProfileManager{}
	var got profile.ProfilePatch
	mgr.On("Update", mock.Anything, userID, mock.Anything).
		Run(func(args mock.Arguments) { got = args.Get(2).(profile.ProfilePatch) }).
		Return(buildTestProfile(userID), nil)

	rec, err := patchProfile(t, mgr, userID, body)
	require.NoError(t, err, "body %s", body)
	require.Equal(t, http.StatusOK, rec.Code)
	return got
}

// requireBadRequest asserts a 400 and that the manager was not called.
func requireBadRequest(t *testing.T, body string) {
	t.Helper()
	mgr := &mockProfileManager{}
	_, err := patchProfile(t, mgr, uuid.New(), body)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he, "body %s", body)
	assert.Equal(t, http.StatusBadRequest, he.Code, "body %s", body)
	mgr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestPatchProfile_Members(t *testing.T) {
	dob := time.Date(1990, 5, 14, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		member  string
		value   string
		field   func(profile.ProfilePatch) any
		wantSet any
		wantClr any
	}{
		{"city", `"Madrid"`, func(p profile.ProfilePatch) any { return p.City }, profile.SetTo("Madrid"), profile.Cleared[string]()},
		{"country", `"ES"`, func(p profile.ProfilePatch) any { return p.Country }, profile.SetTo("ES"), profile.Cleared[string]()},
		{"region", `"ES-MD"`, func(p profile.ProfilePatch) any { return p.Region }, profile.SetTo("ES-MD"), profile.Cleared[string]()},
		{"gender", `"female"`, func(p profile.ProfilePatch) any { return p.Gender }, profile.SetTo(profile.GenderFemale), profile.Cleared[profile.Gender]()},
		{"date_of_birth", `"1990-05-14"`, func(p profile.ProfilePatch) any { return p.DateOfBirth }, profile.SetTo(dob), profile.Cleared[time.Time]()},
		{"category", `"third"`, func(p profile.ProfilePatch) any { return p.Category }, profile.SetTo(profile.CategoryThird), profile.Cleared[profile.Category]()},
	}
	for _, tt := range tests {
		t.Run(tt.member, func(t *testing.T) {
			omitted := capturedPatch(t, `{}`)
			assert.True(t, omitted.IsEmpty(), "omitted → unchanged")

			assert.Equal(t, tt.wantSet, tt.field(capturedPatch(t, fmt.Sprintf(`{%q: %s}`, tt.member, tt.value))), "value → set")
			assert.Equal(t, tt.wantClr, tt.field(capturedPatch(t, fmt.Sprintf(`{%q: null}`, tt.member))), "null → cleared")

			requireBadRequest(t, fmt.Sprintf(`{%q: ""}`, tt.member))
			requireBadRequest(t, fmt.Sprintf(`{%q: "   "}`, tt.member))
			requireBadRequest(t, fmt.Sprintf(`{%q: 42}`, tt.member))
			requireBadRequest(t, fmt.Sprintf(`{%q: ["x"]}`, tt.member))
		})
	}

	t.Run("display_name", func(t *testing.T) {
		got := capturedPatch(t, `{"display_name": "Alice"}`)
		require.NotNil(t, got.DisplayName)
		assert.Equal(t, "Alice", *got.DisplayName)
		requireBadRequest(t, `{"display_name": null}`)
		requireBadRequest(t, `{"display_name": "  "}`)
		requireBadRequest(t, `{"display_name": 1}`)
	})
}

func TestPatchProfile_InvalidValues(t *testing.T) {
	for _, body := range []string{
		`{"gender": "other"}`,
		`{"category": "sixth"}`,
		`{"date_of_birth": "14/05/1990"}`,
		`{"date_of_birth": "1990-13-01"}`,
	} {
		requireBadRequest(t, body)
	}
}

func TestPatchProfile_Preferences(t *testing.T) {
	got := capturedPatch(t, `{"preferences": {"court_side": "drive"}}`)
	assert.Equal(t, profile.PreferencesPatch{CourtSide: profile.SetTo(profile.CourtSideDrive)}, got.Preferences)

	got = capturedPatch(t, `{"preferences": {"handedness": null}}`)
	assert.Equal(t, profile.PreferencesPatch{Handedness: profile.Cleared[profile.Handedness]()}, got.Preferences)

	got = capturedPatch(t, `{"preferences": null}`)
	assert.Equal(t, profile.PreferencesPatch{Clear: true}, got.Preferences)

	got = capturedPatch(t, `{"preferences": {}}`)
	assert.True(t, got.Preferences.IsEmpty())

	for _, body := range []string{
		`{"preferences": "drive"}`,
		`{"preferences": []}`,
		`{"preferences": {"foot": "left"}}`,
		`{"preferences": {"court_side": "middle"}}`,
		`{"preferences": {"handedness": ""}}`,
	} {
		requireBadRequest(t, body)
	}
}

func TestPatchProfile_UnknownMember(t *testing.T) {
	mgr := &mockProfileManager{}
	_, err := patchProfile(t, mgr, uuid.New(), `{"date_of_brith": "1990-05-14"}`)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	assert.Contains(t, fmt.Sprint(he.Message), `"date_of_brith"`)
	mgr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestPatchProfile_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"invalid location", fmt.Errorf("%w: region requires country", ckerrors.ErrInvalidLocation), http.StatusBadRequest},
		{"profile not found", fmt.Errorf("profile manager: Update: %w", ckerrors.ErrProfileNotFound), http.StatusNotFound},
		{"conflict", fmt.Errorf("profile manager: Update: %w", ckerrors.ErrProfileConflict), http.StatusConflict},
		{"other", errors.New("pq: connection refused"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()
			mgr := &mockProfileManager{}
			mgr.On("Update", mock.Anything, userID, mock.Anything).Return(nil, tt.err)

			_, err := patchProfile(t, mgr, userID, `{"region": "ES-MD"}`)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tt.wantStatus, he.Code)
			if tt.wantStatus == http.StatusInternalServerError {
				assert.NotContains(t, fmt.Sprint(he.Message), "connection refused", "internal errors must not leak")
			}
		})
	}
}

func TestPatchProfile_MediaType(t *testing.T) {
	h, e := newTestHandler(&mockProfileManager{})
	c, _ := contextWithAuthAndSub(e, http.MethodPatch, "/api/v1/users/me/profile", `{"city": "Madrid"}`, uuid.NewString())
	c.Request().Header.Set(echo.HeaderContentType, "text/plain")

	var he *echo.HTTPError
	require.ErrorAs(t, h.updateMyProfile(c), &he)
	assert.Equal(t, http.StatusUnsupportedMediaType, he.Code)
}

func TestPatchProfile_EmptyObject(t *testing.T) {
	got := capturedPatch(t, `{}`)
	assert.True(t, got.IsEmpty())
}

func TestPatchProfile_ResponseShape(t *testing.T) {
	userID := uuid.New()
	cleared := &profile.Profile{UserID: userID, DisplayName: "Alice", UpdatedAt: time.Now()}
	mgr := &mockProfileManager{}
	mgr.On("Update", mock.Anything, userID, mock.Anything).Return(cleared, nil)

	rec, err := patchProfile(t, mgr, userID, `{"city": null, "gender": null}`)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	for _, field := range []string{"city", "region", "country", "gender", "date_of_birth", "category"} {
		v, present := body[field]
		assert.True(t, present, "%s must be present", field)
		assert.Nil(t, v, "%s must be null", field)
	}
	assert.Equal(t, map[string]any{}, body["preferences"])
}

// PUT is gone. The protected /api/v1 group has middleware (JWT), so Echo's
// catch-all route answers 404 for any unregistered method, as for every other
// endpoint of the API.
func TestPutProfile_Removed(t *testing.T) {
	h, e := newTestHandler(&mockProfileManager{})
	passthrough := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	NewRoutes(h).Register(e.Group("/api/v1", passthrough))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/profile", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
