package profile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/profile"
)

// ---- helpers ----

func buildListRequest(e *echo.Echo, query string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users"+query, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func buildListItem(id uuid.UUID, displayName, country string) *profile.ProfileListItem {
	dn := displayName
	c := country
	return &profile.ProfileListItem{ID: &id, DisplayName: &dn, Country: &c}
}

// ---- tests ----

// LU-01: Default params — page=1, page_size=20, all fields.
func TestListUsers_DefaultParams_Returns200(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	items := []*profile.ProfileListItem{
		buildListItem(id1, "Alice", "ES"),
		buildListItem(id2, "Bob", "PT"),
	}
	mgr := &mockProfileManager{}
	mgr.On("List", mock.Anything, profile.ListParams{Page: 1, PageSize: 20, Fields: nil}).
		Return(items, int64(2), nil)

	h, e := newTestHandler(mgr)
	c, rec := buildListRequest(e, "")

	err := h.listUsers(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp listUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(2), resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	assert.Len(t, resp.Data, 2)
	mgr.AssertExpectations(t)
}

// LU-02: Explicit page and page_size.
func TestListUsers_ExplicitPageAndPageSize_Returns200(t *testing.T) {
	mgr := &mockProfileManager{}
	mgr.On("List", mock.Anything, profile.ListParams{Page: 2, PageSize: 5, Fields: nil}).
		Return([]*profile.ProfileListItem{}, int64(10), nil)

	h, e := newTestHandler(mgr)
	c, rec := buildListRequest(e, "?page=2&page_size=5")

	err := h.listUsers(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp listUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 5, resp.PageSize)
	assert.Equal(t, int64(10), resp.Total)
	mgr.AssertExpectations(t)
}

// LU-03: Field selection — manager called with requested fields, and the
// response only includes those fields even when the repository returns a
// fully-populated item (i.e. filtering happens in the handler, not just
// as a passthrough parameter).
func TestListUsers_FieldSelection_Returns200(t *testing.T) {
	id := uuid.New()
	idStr := id.String()
	dn := "Alice"
	country := "ES"
	city := "Barcelona"
	region := "ES-CT"
	gender := profile.GenderFemale
	category := profile.CategoryFirst
	item := &profile.ProfileListItem{
		ID: &id, DisplayName: &dn, City: &city, Region: &region,
		Country: &country, Gender: &gender, Category: &category,
	}

	mgr := &mockProfileManager{}
	mgr.On("List", mock.Anything, profile.ListParams{
		Page:     1,
		PageSize: 20,
		Fields:   []string{"id", "display_name", "country"},
	}).Return([]*profile.ProfileListItem{item}, int64(1), nil)

	h, e := newTestHandler(mgr)
	c, rec := buildListRequest(e, "?fields=id,display_name,country")

	err := h.listUsers(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp listUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, &idStr, resp.Data[0].ID)
	assert.Equal(t, &dn, resp.Data[0].DisplayName)
	assert.Equal(t, &country, resp.Data[0].Country)
	assert.Nil(t, resp.Data[0].City, "city was not requested and must be omitted")
	assert.Nil(t, resp.Data[0].Region, "region was not requested and must be omitted")
	assert.Nil(t, resp.Data[0].Gender, "gender was not requested and must be omitted")
	assert.Nil(t, resp.Data[0].Category, "category was not requested and must be omitted")
	mgr.AssertExpectations(t)
}

// LU-04: page_size exceeds 100 → 400.
func TestListUsers_PageSizeExceedsMax_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newTestHandler(mgr)
	c, _ := buildListRequest(e, "?page_size=101")

	err := h.listUsers(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertNotCalled(t, "List")
}

// LU-05: page less than 1 → 400.
func TestListUsers_PageLessThanOne_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newTestHandler(mgr)
	c, _ := buildListRequest(e, "?page=0")

	err := h.listUsers(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertNotCalled(t, "List")
}

// LU-06: Unknown field name → 400.
func TestListUsers_UnknownField_Returns400(t *testing.T) {
	mgr := &mockProfileManager{}
	h, e := newTestHandler(mgr)
	c, _ := buildListRequest(e, "?fields=id,unknown_field")

	err := h.listUsers(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
	mgr.AssertNotCalled(t, "List")
}

// LU-07: Empty result set → 200 with data:[] and total:0.
func TestListUsers_EmptyResult_Returns200(t *testing.T) {
	mgr := &mockProfileManager{}
	mgr.On("List", mock.Anything, profile.ListParams{Page: 1, PageSize: 20, Fields: nil}).
		Return([]*profile.ProfileListItem{}, int64(0), nil)

	h, e := newTestHandler(mgr)
	c, rec := buildListRequest(e, "")

	err := h.listUsers(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp listUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
	mgr.AssertExpectations(t)
}

// LU-08: Manager returns error → 500.
func TestListUsers_ManagerError_Returns500(t *testing.T) {
	mgr := &mockProfileManager{}
	mgr.On("List", mock.Anything, mock.Anything).
		Return(nil, int64(0), assert.AnError)

	h, e := newTestHandler(mgr)
	c, _ := buildListRequest(e, "")

	err := h.listUsers(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusInternalServerError, he.Code)
	mgr.AssertExpectations(t)
}
