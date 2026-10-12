// Package pats provides the HTTP handlers for PAT management endpoints.
package pats

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/courtknights/courtknights/internal/api/common"
	appauth "github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
)

// Handler handles all PAT management HTTP requests.
type Handler struct {
	manager appauth.AuthManager
}

// NewHandler returns a Handler backed by the given AuthManager.
func NewHandler(manager appauth.AuthManager) *Handler {
	return &Handler{manager: manager}
}

// createPAT creates a new PAT for a given user. Admin only.
// POST /api/v1/pats
func (h *Handler) createPAT(c echo.Context) error {
	if !common.IsAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}

	var req struct {
		UserID    string  `json:"user_id"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid expires_at format, use RFC3339")
		}
		expiresAt = &t
	}

	rawKey, err := h.manager.CreatePAT(c.Request().Context(), userID, expiresAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create PAT")
	}

	return c.JSON(http.StatusCreated, map[string]string{"token": rawKey})
}

// patResponse is the public representation of a PAT. It never includes the
// key hash or salt (#135).
type patResponse struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// listPATs returns all PATs, without their secrets. Admin only.
// GET /api/v1/pats
func (h *Handler) listPATs(c echo.Context) error {
	if !common.IsAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}
	pats, err := h.manager.ListPATs(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list PATs")
	}

	resp := make([]patResponse, len(pats))
	for i, p := range pats {
		resp[i] = patResponse{ID: p.ID.String(), CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt}
	}
	return c.JSON(http.StatusOK, resp)
}

// revokePAT deletes a PAT by ID. Admin only.
// DELETE /api/v1/pats/:id
func (h *Handler) revokePAT(c echo.Context) error {
	if !common.IsAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "admin role required")
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid PAT id")
	}

	if err := h.manager.RevokePAT(c.Request().Context(), id); err != nil {
		if errors.Is(err, ckerrors.ErrPATNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "PAT not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to revoke PAT")
	}

	return c.NoContent(http.StatusNoContent)
}
