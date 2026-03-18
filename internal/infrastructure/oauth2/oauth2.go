// Package oauth2 provides OAuth2 provider adapters for CourtKnights.
// Each provider implements the Provider interface for both the redirect
// (web) flow and the Device Authorization Grant (RFC 8628, CLI) flow.
package oauth2

import "context"

// UserInfo holds the normalised user data returned by an OAuth2 provider
// after a successful token exchange.
type UserInfo struct {
	// ProviderID is the user's unique identifier within the provider's namespace.
	ProviderID string
	// Email is the user's primary email address.
	Email string
	// Name is the user's display name.
	Name string
}

// DeviceAuthResponse contains the fields returned by the device authorisation endpoint.
// The user must visit VerificationURI and enter UserCode to grant access.
type DeviceAuthResponse struct {
	// DeviceCode is an opaque code used to poll for the token.
	DeviceCode string
	// UserCode is the short code shown to the end user.
	UserCode string
	// VerificationURI is the URL the user must visit.
	VerificationURI string
	// ExpiresIn is the lifetime of the device code in seconds.
	ExpiresIn int
	// Interval is the polling interval in seconds.
	Interval int
}

// Provider defines the OAuth2 operations required by the AuthService.
// Implementations must be safe for concurrent use.
type Provider interface {
	// AuthCodeURL returns the provider's authorisation URL for the redirect flow.
	// state is an opaque CSRF token.
	AuthCodeURL(state string) string

	// Exchange trades an authorisation code for user information.
	Exchange(ctx context.Context, code string) (*UserInfo, error)

	// DeviceAuth initiates a Device Authorization Grant.
	// Returns the device auth response containing the user_code and verification_uri.
	DeviceAuth(ctx context.Context) (*DeviceAuthResponse, error)

	// DevicePoll polls the provider's token endpoint using the device code.
	// Returns ErrAuthorizationPending when the user has not yet granted access.
	// Returns UserInfo once the user has approved the request.
	DevicePoll(ctx context.Context, deviceCode string) (*UserInfo, error)
}

// ErrAuthorizationPending is returned by DevicePoll when the device code is
// valid but the user has not yet completed the authorisation.
var ErrAuthorizationPending = &authorizationPendingError{}

type authorizationPendingError struct{}

func (e *authorizationPendingError) Error() string {
	return "authorization_pending"
}
