package auth

import (
	"context"
	"fmt"

	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// AuthManager is the root manager for the authentication feature.
// It composes UserManager, JWTManager, and OAuthManager to implement
// all authentication use cases. It is the only auth component the Handler calls.
//
// Rule: AuthManager calls other managers only — never repositories or
// infrastructure adapters directly.
type AuthManager struct {
	user  *UserManager
	jwt   *JWTManager
	oauth *OAuthManager
}

// NewAuthManager returns an AuthManager with the three required leaf managers.
func NewAuthManager(user *UserManager, jwt *JWTManager, oauth *OAuthManager) *AuthManager {
	return &AuthManager{user: user, jwt: jwt, oauth: oauth}
}

// OAuthRedirectURL returns the provider's authorisation URL for the redirect flow.
func (m *AuthManager) OAuthRedirectURL(provider user.Provider, state string) (string, error) {
	return m.oauth.RedirectURL(provider, state)
}

// OAuthCallback exchanges a provider authorisation code for a signed JWT.
func (m *AuthManager) OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error) {
	info, err := m.oauth.Exchange(ctx, provider, code)
	if err != nil {
		return "", fmt.Errorf("auth manager: OAuthCallback: %w", err)
	}
	u, err := m.user.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("auth manager: OAuthCallback: %w", err)
	}
	return m.jwt.Sign(u)
}

// DeviceInit initiates the Device Authorization Grant for the given provider.
func (m *AuthManager) DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	return m.oauth.DeviceAuth(ctx, provider)
}

// DevicePoll polls for an authorised device token and returns a signed JWT.
func (m *AuthManager) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error) {
	info, err := m.oauth.DevicePoll(ctx, provider, deviceCode)
	if err != nil {
		return "", fmt.Errorf("auth manager: DevicePoll: %w", err)
	}
	u, err := m.user.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("auth manager: DevicePoll: %w", err)
	}
	return m.jwt.Sign(u)
}

// ExchangePAT validates a raw PAT and returns a signed JWT.
func (m *AuthManager) ExchangePAT(ctx context.Context, rawPAT string) (string, error) {
	u, err := m.user.ResolveByPAT(ctx, rawPAT)
	if err != nil {
		return "", fmt.Errorf("auth manager: ExchangePAT: %w", err)
	}
	return m.jwt.Sign(u)
}

// RefreshJWT validates the current JWT and issues a new one with a fresh expiry.
func (m *AuthManager) RefreshJWT(ctx context.Context, tokenStr string) (string, error) {
	claims, err := m.jwt.Validate(tokenStr)
	if err != nil {
		return "", fmt.Errorf("auth manager: RefreshJWT: %w", err)
	}
	u := &user.User{Email: claims.Email, Role: claims.Role}
	if id, err := parseUUID(claims.Subject); err == nil {
		u.ID = id
	}
	return m.jwt.Sign(u)
}
