package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appauth "github.com/courtknights/courtknights/internal/application/auth"
)

// providerError maps application-layer errors to HTTP responses.
func providerError(err error) error {
	if errors.Is(err, appauth.ErrProviderNotConfigured) {
		return echo.NewHTTPError(http.StatusNotImplemented, "provider not configured")
	}
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
