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

// OAuthManager encapsulates all OAuth2 operations.
// It owns the provider registry and is the only application-layer component
// that talks to OAuth2 infrastructure adapters.
type OAuthManager interface {
	RedirectURL(provider user.Provider, state string) (string, error)
	Exchange(ctx context.Context, provider user.Provider, code string) (*oauth2infra.UserInfo, error)
	DeviceAuth(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error)
	DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (*oauth2infra.UserInfo, error)
}

type oauthManager struct {
	providers map[user.Provider]oauth2infra.Provider
}

// NewOAuthManager returns an OAuthManager with the given provider registry.
// Providers absent from the map will cause ErrProviderNotConfigured to be returned.
func NewOAuthManager(providers map[user.Provider]oauth2infra.Provider) OAuthManager {
	return &oauthManager{providers: providers}
}

func (m *oauthManager) RedirectURL(provider user.Provider, state string) (string, error) {
	p, err := m.provider(provider)
	if err != nil {
		return "", err
	}
	return p.AuthCodeURL(state), nil
}

func (m *oauthManager) Exchange(ctx context.Context, provider user.Provider, code string) (*oauth2infra.UserInfo, error) {
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

func (m *oauthManager) DeviceAuth(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
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

func (m *oauthManager) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (*oauth2infra.UserInfo, error) {
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

func (m *oauthManager) provider(p user.Provider) (oauth2infra.Provider, error) {
	prov, ok := m.providers[p]
	if !ok {
		return nil, fmt.Errorf("oauth manager: %w: %q", ErrProviderNotConfigured, p)
	}
	return prov, nil
}
