package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appauth "github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
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

// callbackErrorCode maps login errors the user can act on to the error code sent
// to the client: as a redirect query parameter in the browser flow, or as a JSON
// body in the device flow. ok is false for all other errors.
func callbackErrorCode(err error) (code string, ok bool) {
	if errors.Is(err, oauth2infra.ErrEmailNotVerified) {
		return "email_not_verified", true
	}
	return "", false
}

// emailConflictProvider reports whether err is an email already owned by
// another account (#132) and, if it may be disclosed, that account's
// provider. The email itself is never returned to the client.
func emailConflictProvider(err error) (provider string, ok bool) {
	var conflict *ckerrors.EmailAlreadyRegisteredError
	if errors.As(err, &conflict) {
		return conflict.Provider, true
	}
	if errors.Is(err, ckerrors.ErrEmailAlreadyRegistered) {
		return "", true
	}
	return "", false
}
