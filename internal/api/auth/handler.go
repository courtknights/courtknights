// Package auth provides the HTTP handlers for authentication endpoints.
package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"

	appauth "github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// Handler handles all authentication HTTP requests.
// It depends on the AuthManager interface — never on concrete types.
type Handler struct {
	manager appauth.AuthManager
}

// NewHandler returns a Handler backed by the given AuthManager.
func NewHandler(manager appauth.AuthManager) *Handler {
	return &Handler{manager: manager}
}

// redirectToProvider redirects the browser to the OAuth2 provider's auth page.
// GET /auth/{provider}
func (h *Handler) redirectToProvider(c echo.Context) error {
	provider, err := parseProvider(c.Param("provider"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Use a fixed state for now; a production implementation would generate
	// a random CSRF token stored in a short-lived cookie.
	state := "ck-state"
	url, err := h.manager.OAuthRedirectURL(provider, state)
	if err != nil {
		return providerError(err)
	}

	return c.Redirect(http.StatusFound, url)
}

// oauthCallback exchanges the provider code for a JWT and redirects.
// GET /auth/{provider}/callback
func (h *Handler) oauthCallback(c echo.Context) error {
	provider, err := parseProvider(c.Param("provider"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	code := c.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing code")
	}

	token, err := h.manager.OAuthCallback(c.Request().Context(), provider, code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication failed")
	}

	// Redirect to the frontend with the JWT as a query parameter.
	// A production implementation would set an HttpOnly cookie instead.
	return c.Redirect(http.StatusFound, "/?token="+token)
}

// deviceAuth initiates the Device Authorization Grant for the given provider.
// POST /auth/device
func (h *Handler) deviceAuth(c echo.Context) error {
	var req struct {
		Provider string `json:"provider"`
	}
	if err := c.Bind(&req); err != nil || req.Provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	provider, err := parseProvider(req.Provider)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	resp, err := h.manager.DeviceInit(c.Request().Context(), provider)
	if err != nil {
		return providerError(err)
	}

	return c.JSON(http.StatusOK, resp)
}

// deviceToken polls for an authorised device token.
// POST /auth/device/token
func (h *Handler) deviceToken(c echo.Context) error {
	var req struct {
		Provider   string `json:"provider"`
		DeviceCode string `json:"device_code"`
	}
	if err := c.Bind(&req); err != nil || req.Provider == "" || req.DeviceCode == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider and device_code are required")
	}

	provider, err := parseProvider(req.Provider)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	token, err := h.manager.DevicePoll(c.Request().Context(), provider, req.DeviceCode)
	if err != nil {
		if isAuthorizationPending(err) {
			return c.JSON(http.StatusAccepted, map[string]string{"error": "authorization_pending"})
		}
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication failed")
	}

	return c.JSON(http.StatusOK, map[string]string{"token": token})
}

// parseProvider converts a path parameter string to a user.Provider.
func parseProvider(s string) (user.Provider, error) {
	switch s {
	case "google":
		return user.ProviderGoogle, nil
	case "github":
		return user.ProviderGitHub, nil
	default:
		return "", echo.NewHTTPError(http.StatusBadRequest, "unknown provider: "+s)
	}
}
