package auth

import (
	"context"
	"fmt"

	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// Service is the application-layer orchestrator for authentication flows.
// It coordinates AuthManager, OAuthManager, and JWTManager.
// It never calls repositories or infrastructure adapters directly.
type Service struct {
	auth  *Manager
	oauth *OAuthManager
	jwt   *JWTManager
}

// NewService returns a Service with the three required managers.
func NewService(auth *Manager, oauth *OAuthManager, jwt *JWTManager) *Service {
	return &Service{auth: auth, oauth: oauth, jwt: jwt}
}

// OAuthRedirectURL returns the provider's authorisation URL for the redirect flow.
func (s *Service) OAuthRedirectURL(provider user.Provider, state string) (string, error) {
	return s.oauth.RedirectURL(provider, state)
}

// OAuthCallback exchanges a provider code for a signed JWT.
func (s *Service) OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error) {
	info, err := s.oauth.Exchange(ctx, provider, code)
	if err != nil {
		return "", fmt.Errorf("service: OAuthCallback: %w", err)
	}
	u, err := s.auth.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("service: OAuthCallback: %w", err)
	}
	return s.jwt.Sign(u)
}

// DeviceInit initiates the Device Authorization Grant for the given provider.
func (s *Service) DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	return s.oauth.DeviceAuth(ctx, provider)
}

// DevicePoll polls for an authorised device token and returns a signed JWT.
func (s *Service) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error) {
	info, err := s.oauth.DevicePoll(ctx, provider, deviceCode)
	if err != nil {
		return "", fmt.Errorf("service: DevicePoll: %w", err)
	}
	u, err := s.auth.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("service: DevicePoll: %w", err)
	}
	return s.jwt.Sign(u)
}

// ExchangePAT validates a raw PAT and returns a signed JWT.
func (s *Service) ExchangePAT(ctx context.Context, rawPAT string) (string, error) {
	u, err := s.auth.ResolveByPAT(ctx, rawPAT)
	if err != nil {
		return "", fmt.Errorf("service: ExchangePAT: %w", err)
	}
	return s.jwt.Sign(u)
}

// RefreshJWT validates the current JWT and issues a new one with a fresh expiry.
func (s *Service) RefreshJWT(ctx context.Context, tokenStr string) (string, error) {
	claims, err := s.jwt.Validate(tokenStr)
	if err != nil {
		return "", fmt.Errorf("service: RefreshJWT: %w", err)
	}
	u := &user.User{Email: claims.Email, Role: claims.Role}
	if id, err := parseUUID(claims.Subject); err == nil {
		u.ID = id
	}
	return s.jwt.Sign(u)
}
