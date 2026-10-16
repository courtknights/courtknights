// Package profile provides the HTTP handlers for the user-profile endpoints.
package profile

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/courtknights/courtknights/internal/api/common"
	appprofile "github.com/courtknights/courtknights/internal/application/profile"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// allowedListFields is the set of field names accepted by the list endpoint.
var allowedListFields = map[string]struct{}{
	"id":           {},
	"display_name": {},
	"city":         {},
	"region":       {},
	"country":      {},
	"gender":       {},
	"category":     {},
}

// Handler handles all user-profile HTTP requests.
type Handler struct {
	manager appprofile.ProfileManager
}

// NewHandler returns a Handler backed by the given ProfileManager.
func NewHandler(manager appprofile.ProfileManager) *Handler {
	return &Handler{manager: manager}
}

// getMyProfile returns the authenticated user's profile.
// GET /api/v1/users/me/profile
func (h *Handler) getMyProfile(c echo.Context) error {
	sub, ok := c.Get(common.ContextKeySub).(string)
	if !ok || sub == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing sub claim")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sub claim")
	}

	p, err := h.manager.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, ckerrors.ErrProfileNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get profile")
	}

	return c.JSON(http.StatusOK, toProfileResponse(p))
}

// updateMyProfile applies a JSON Merge Patch (ADR-015) to the authenticated
// user's profile: omitted members are unchanged, null clears a field and a
// value sets it.
// PATCH /api/v1/users/me/profile
func (h *Handler) updateMyProfile(c echo.Context) error {
	sub, ok := c.Get(common.ContextKeySub).(string)
	if !ok || sub == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing sub claim")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sub claim")
	}

	members, err := common.DecodeMergePatch(c)
	if err != nil {
		return err
	}

	patch, err := parseProfilePatch(members)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	p, err := h.manager.Update(c.Request().Context(), userID, patch)
	if err != nil {
		switch {
		case errors.Is(err, ckerrors.ErrProfileNotFound):
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		case errors.Is(err, ckerrors.ErrInvalidLocation):
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		case errors.Is(err, ckerrors.ErrProfileConflict):
			return echo.NewHTTPError(http.StatusConflict, "profile changed concurrently, retry the request")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update profile")
		}
	}

	return c.JSON(http.StatusOK, toProfileResponse(p))
}

// getUserProfile returns the profile for a given user by UUID path param.
// GET /api/v1/users/:id/profile
func (h *Handler) getUserProfile(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	p, err := h.manager.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, ckerrors.ErrProfileNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get profile")
	}

	return c.JSON(http.StatusOK, toProfileResponse(p))
}

// profileResponse is the JSON representation of a Profile returned by the API.
// Nullable fields have no omitempty: per AC-02 they are always present and
// serialise as null when unset.
type profileResponse struct {
	UserID      string              `json:"user_id"`
	DisplayName string              `json:"display_name"`
	City        *string             `json:"city"`
	Region      *string             `json:"region"`
	Country     *string             `json:"country"`
	Gender      *string             `json:"gender"`
	DateOfBirth *string             `json:"date_of_birth"`
	Category    *string             `json:"category"`
	Preferences preferencesResponse `json:"preferences"`
	UpdatedAt   string              `json:"updated_at"`
}

// preferencesResponse is the JSON representation of Preferences in API responses.
type preferencesResponse struct {
	CourtSide  *string `json:"court_side,omitempty"`
	Handedness *string `json:"handedness,omitempty"`
}

// toProfileResponse converts a domain Profile to its API response representation.
func toProfileResponse(p *profile.Profile) profileResponse {
	resp := profileResponse{
		UserID:      p.UserID.String(),
		DisplayName: p.DisplayName,
		City:        p.City,
		Region:      p.Region,
		Country:     p.Country,
		UpdatedAt:   p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if p.Gender != nil {
		s := string(*p.Gender)
		resp.Gender = &s
	}

	if p.DateOfBirth != nil {
		s := p.DateOfBirth.UTC().Format("2006-01-02")
		resp.DateOfBirth = &s
	}

	if p.Category != nil {
		s := string(*p.Category)
		resp.Category = &s
	}

	if p.Preferences.CourtSide != nil {
		s := string(*p.Preferences.CourtSide)
		resp.Preferences.CourtSide = &s
	}

	if p.Preferences.Handedness != nil {
		s := string(*p.Preferences.Handedness)
		resp.Preferences.Handedness = &s
	}

	return resp
}

// listUsers returns a paginated, optionally field-filtered list of user profiles.
// GET /api/v1/users
func (h *Handler) listUsers(c echo.Context) error {
	page := 1
	pageSize := 20

	if v := c.QueryParam("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "page must be a positive integer")
		}
		page = n
	}

	if v := c.QueryParam("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "page_size must be a positive integer")
		}
		if n > 100 {
			return echo.NewHTTPError(http.StatusBadRequest, "page_size must not exceed 100")
		}
		pageSize = n
	}

	var fields []string
	if v := c.QueryParam("fields"); v != "" {
		for _, f := range strings.Split(v, ",") {
			f = strings.TrimSpace(f)
			if _, ok := allowedListFields[f]; !ok {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unknown field: %q", f))
			}
			fields = append(fields, f)
		}
	}

	items, total, err := h.manager.List(c.Request().Context(), profile.ListParams{
		Page:     page,
		PageSize: pageSize,
		Fields:   fields,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}

	data := make([]listItemResponse, len(items))
	for i, item := range items {
		data[i] = toListItemResponse(item, fields)
	}

	return c.JSON(http.StatusOK, listUsersResponse{
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// listUsersResponse is the JSON envelope for the list-users endpoint.
type listUsersResponse struct {
	Data     []listItemResponse `json:"data"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// listItemResponse is a single entry in the list-users response.
// All profile fields are optional (omitted when nil / not requested).
type listItemResponse struct {
	ID          *string `json:"id,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	City        *string `json:"city,omitempty"`
	Region      *string `json:"region,omitempty"`
	Country     *string `json:"country,omitempty"`
	Gender      *string `json:"gender,omitempty"`
	Category    *string `json:"category,omitempty"`
}

// toListItemResponse converts a domain ProfileListItem to its API response form.
// When fields is non-empty, only the requested fields are populated in the result.
func toListItemResponse(item *profile.ProfileListItem, fields []string) listItemResponse {
	r := listItemResponse{
		DisplayName: item.DisplayName,
		City:        item.City,
		Region:      item.Region,
		Country:     item.Country,
	}
	if item.ID != nil {
		s := item.ID.String()
		r.ID = &s
	}
	if item.Gender != nil {
		s := string(*item.Gender)
		r.Gender = &s
	}
	if item.Category != nil {
		s := string(*item.Category)
		r.Category = &s
	}

	if len(fields) == 0 {
		return r
	}

	requested := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		requested[f] = struct{}{}
	}

	filtered := listItemResponse{}
	if _, ok := requested["id"]; ok {
		filtered.ID = r.ID
	}
	if _, ok := requested["display_name"]; ok {
		filtered.DisplayName = r.DisplayName
	}
	if _, ok := requested["city"]; ok {
		filtered.City = r.City
	}
	if _, ok := requested["region"]; ok {
		filtered.Region = r.Region
	}
	if _, ok := requested["country"]; ok {
		filtered.Country = r.Country
	}
	if _, ok := requested["gender"]; ok {
		filtered.Gender = r.Gender
	}
	if _, ok := requested["category"]; ok {
		filtered.Category = r.Category
	}
	return filtered
}
