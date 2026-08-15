// Package users provides the HTTP handlers for user management endpoints.
package users

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/courtknights/courtknights/internal/api/common"
	appauth "github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// Handler handles all user management HTTP requests.
type Handler struct {
	manager appauth.AuthManager
}

// NewHandler returns a Handler backed by the given AuthManager.
func NewHandler(manager appauth.AuthManager) *Handler {
	return &Handler{manager: manager}
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
