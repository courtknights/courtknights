// Package users provides the HTTP handlers for user management endpoints.
package users

import (
	"errors"
	"fmt"
	"net/http"
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
