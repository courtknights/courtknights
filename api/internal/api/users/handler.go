// Package users provides the HTTP handlers for user management endpoints.
package users

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/courtknights/courtknights/internal/api/common"
	appauth "github.com/courtknights/courtknights/internal/application/auth"
	appprofile "github.com/courtknights/courtknights/internal/application/profile"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
	"github.com/courtknights/courtknights/internal/domain/user"
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

// Handler handles all user management HTTP requests.
type Handler struct {
	manager  appauth.AuthManager
	profiles appprofile.ProfileManager
}

// NewHandler returns a Handler backed by the given AuthManager and ProfileManager.
func NewHandler(manager appauth.AuthManager, profiles appprofile.ProfileManager) *Handler {
	return &Handler{manager: manager, profiles: profiles}
}

// me returns the authenticated user's profile from the JWT context.
// GET /api/v1/users/me
func (h *Handler) me(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"id":    c.Get(common.ContextKeySub).(string),
		"email": c.Get(common.ContextKeyEmail).(string),
		"role":  c.Get(common.ContextKeyRole).(string),
	})
}

// updateRole sets the role for a user by ID. Admin only.
// PUT /api/v1/users/:id/role
func (h *Handler) updateRole(c echo.Context) error {
	if !common.IsAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil || req.Role == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "role is required")
	}

	role := user.Role(req.Role)
	if role != user.RoleAdmin && role != user.RoleUser {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role, must be 'admin' or 'user'")
	}

	if err := h.manager.UpdateUserRole(c.Request().Context(), userID, role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update role")
	}

	return c.JSON(http.StatusOK, map[string]string{"id": userID.String(), "role": string(role)})
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

	p, err := h.profiles.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, ckerrors.ErrProfileNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get profile")
	}

	return c.JSON(http.StatusOK, toProfileResponse(p))
}

// updateMyProfile applies a partial update to the authenticated user's profile.
// PUT /api/v1/users/me/profile
func (h *Handler) updateMyProfile(c echo.Context) error {
	sub, ok := c.Get(common.ContextKeySub).(string)
	if !ok || sub == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing sub claim")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sub claim")
	}

	var req profilePatchRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	patch, err := req.toPatch()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	p, err := h.profiles.Update(c.Request().Context(), userID, patch)
	if err != nil {
		if errors.Is(err, ckerrors.ErrProfileNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
		// Validation errors from the manager (invalid location codes, etc.) are 400.
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
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

	p, err := h.profiles.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, ckerrors.ErrProfileNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get profile")
	}

	return c.JSON(http.StatusOK, toProfileResponse(p))
}

// profilePatchRequest is the JSON body for the update-profile endpoint.
// All fields are optional (partial update).
type profilePatchRequest struct {
	DisplayName *string             `json:"display_name"`
	City        *string             `json:"city"`
	Region      *string             `json:"region"`
	Country     *string             `json:"country"`
	Gender      *string             `json:"gender"`
	DateOfBirth *string             `json:"date_of_birth"`
	Category    *string             `json:"category"`
	Preferences *preferencesRequest `json:"preferences"`
}

// preferencesRequest is the JSON representation of profile Preferences in the request body.
type preferencesRequest struct {
	CourtSide  *string `json:"court_side"`
	Handedness *string `json:"handedness"`
}

// toPatch converts the JSON request into a domain ProfilePatch.
// Returns an error if any enum value is unrecognised.
func (r *profilePatchRequest) toPatch() (profile.ProfilePatch, error) {
	patch := profile.ProfilePatch{
		DisplayName: r.DisplayName,
		City:        r.City,
		Region:      r.Region,
		Country:     r.Country,
	}

	if r.Gender != nil {
		g := profile.Gender(*r.Gender)
		if g != profile.GenderMale && g != profile.GenderFemale {
			return profile.ProfilePatch{}, fmt.Errorf("invalid gender value: %q", *r.Gender)
		}
		patch.Gender = &g
	}

	if r.DateOfBirth != nil {
		t, err := time.Parse("2006-01-02", *r.DateOfBirth)
		if err != nil {
			return profile.ProfilePatch{}, fmt.Errorf("invalid date_of_birth: must be YYYY-MM-DD")
		}
		patch.DateOfBirth = &t
	}

	if r.Category != nil {
		cat := profile.Category(*r.Category)
		switch cat {
		case profile.CategoryFirst, profile.CategorySecond, profile.CategoryThird,
			profile.CategoryFourth, profile.CategoryFifth:
		default:
			return profile.ProfilePatch{}, fmt.Errorf("invalid category value: %q", *r.Category)
		}
		patch.Category = &cat
	}

	if r.Preferences != nil {
		prefs, err := toPreferences(r.Preferences)
		if err != nil {
			return profile.ProfilePatch{}, err
		}
		patch.Preferences = &prefs
	}

	return patch, nil
}

// toPreferences converts a preferencesRequest into a domain Preferences value.
func toPreferences(r *preferencesRequest) (profile.Preferences, error) {
	var prefs profile.Preferences

	if r.CourtSide != nil {
		cs := profile.CourtSide(*r.CourtSide)
		switch cs {
		case profile.CourtSideDrive, profile.CourtSideBackhand, profile.CourtSideBoth:
		default:
			return profile.Preferences{}, fmt.Errorf("invalid court_side value: %q", *r.CourtSide)
		}
		prefs.CourtSide = &cs
	}

	if r.Handedness != nil {
		h := profile.Handedness(*r.Handedness)
		if h != profile.HandednessRight && h != profile.HandednessLeft {
			return profile.Preferences{}, fmt.Errorf("invalid handedness value: %q", *r.Handedness)
		}
		prefs.Handedness = &h
	}

	return prefs, nil
}

// profileResponse is the JSON representation of a Profile returned by the API.
type profileResponse struct {
	UserID      string              `json:"user_id"`
	DisplayName string              `json:"display_name"`
	City        *string             `json:"city,omitempty"`
	Region      *string             `json:"region,omitempty"`
	Country     *string             `json:"country,omitempty"`
	Gender      *string             `json:"gender,omitempty"`
	DateOfBirth *string             `json:"date_of_birth,omitempty"`
	Category    *string             `json:"category,omitempty"`
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

	items, total, err := h.profiles.List(c.Request().Context(), profile.ListParams{
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
