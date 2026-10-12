package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appauth "github.com/courtknights/courtknights/internal/application/auth"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// providerError maps application-layer errors to HTTP responses.
func providerError(err error) error {
	if errors.Is(err, appauth.ErrProviderNotConfigured) {
		return echo.NewHTTPError(http.StatusNotImplemented, "provider not configured")
	}
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}

// deviceTokenStatus maps a device-poll error to the HTTP status and RFC 8628 error
// code returned by POST /auth/device/token. ok is false for errors that are not
// part of the device-flow protocol.
//
//   - authorization_pending, slow_down → 202: the client polls again.
//   - access_denied, expired_token, invalid_grant → 400: terminal, the client must
//     restart the flow.
func deviceTokenStatus(err error) (status int, code string, ok bool) {
	switch {
	case errors.Is(err, oauth2infra.ErrAuthorizationPending):
		return http.StatusAccepted, "authorization_pending", true
	case errors.Is(err, oauth2infra.ErrSlowDown):
		return http.StatusAccepted, "slow_down", true
	case errors.Is(err, oauth2infra.ErrAccessDenied):
		return http.StatusBadRequest, "access_denied", true
	case errors.Is(err, oauth2infra.ErrDeviceCodeExpired):
		return http.StatusBadRequest, "expired_token", true
	case errors.Is(err, oauth2infra.ErrInvalidDeviceCode):
		return http.StatusBadRequest, "invalid_grant", true
	default:
		return 0, "", false
	}
}
