package auth

import (
	"context"
	"fmt"

	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// ErrProviderNotConfigured is returned when an operation targets an OAuth2
// provider that has not been registered.
var ErrProviderNotConfigured = fmt.Errorf("provider not configured")

// OAuthManager encapsulates all OAuth2 business logic.
// It owns the provider registry and is the only application-layer component
// that talks to OAuth2 infrastructure adapters.
type OAuthManager struct {
	providers map[user.Provider]oauth2infra.Provider
}

// NewOAuthManager returns an OAuthManager with the given provider registry.
// Providers absent from the map will cause ErrProviderNotConfigured to be returned.
func NewOAuthManager(providers map[user.Provider]oauth2infra.Provider) *OAuthManager {
	return &OAuthManager{providers: providers}
}

// RedirectURL returns the authorisation URL for the given provider and CSRF state.
func (m *OAuthManager) RedirectURL(provider user.Provider, state string) (string, error) {
	p, err := m.provider(provider)
	if err != nil {
		return "", err
	}
	return p.AuthCodeURL(state), nil
}

// Exchange trades an authorisation code for normalised user information.
func (m *OAuthManager) Exchange(ctx context.Context, provider user.Provider, code string) (*oauth2infra.UserInfo, error) {
	p, err := m.provider(provider)
	if err != nil {
		return nil, err
	}
	info, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth manager: exchange: %w", err)
	}
	return info, nil
}

// DeviceAuth initiates the Device Authorization Grant for the given provider.
func (m *OAuthManager) DeviceAuth(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	p, err := m.provider(provider)
	if err != nil {
		return nil, err
	}
	resp, err := p.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("oauth manager: device auth: %w", err)
	}
	return resp, nil
}

// DevicePoll polls the provider's token endpoint using the device code.
// Returns ErrAuthorizationPending while the user has not yet approved the request.
func (m *OAuthManager) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (*oauth2infra.UserInfo, error) {
	p, err := m.provider(provider)
	if err != nil {
		return nil, err
	}
	info, err := p.DevicePoll(ctx, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("oauth manager: device poll: %w", err)
	}
	return info, nil
}

// provider looks up the registered adapter for the given provider key.
func (m *OAuthManager) provider(p user.Provider) (oauth2infra.Provider, error) {
	prov, ok := m.providers[p]
	if !ok {
		return nil, fmt.Errorf("oauth manager: %w: %q", ErrProviderNotConfigured, p)
	}
	return prov, nil
}
