package auth

import (
	"context"
	"fmt"

	"github.com/courtknights/courtknights/internal/domain/user"
	jwtinfra "github.com/courtknights/courtknights/internal/infrastructure/jwt"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// JWTSigner can sign a user into a JWT string.
type JWTSigner interface {
	Sign(u *user.User) (string, error)
}

// JWTValidator can validate a JWT string and return its claims.
type JWTValidator interface {
	Validate(tokenStr string) (*jwtinfra.Claims, error)
}

// Service orchestrates OAuth2 providers, the AuthManager, and the JWT adapter
// to implement all authentication flows exposed by the API layer.
type Service struct {
	manager   *Manager
	providers map[user.Provider]oauth2infra.Provider
	signer    JWTSigner
	validator JWTValidator
}

// NewService returns a Service. providers is a map of enabled OAuth2 providers;
// missing entries cause the corresponding endpoints to return 501.
func NewService(
	manager *Manager,
	providers map[user.Provider]oauth2infra.Provider,
	signer JWTSigner,
	validator JWTValidator,
) *Service {
	return &Service{
		manager:   manager,
		providers: providers,
		signer:    signer,
		validator: validator,
	}
}

// OAuthRedirectURL returns the provider's authorisation URL for the redirect flow.
// Returns ErrProviderNotConfigured if the provider is not enabled.
func (s *Service) OAuthRedirectURL(provider user.Provider, state string) (string, error) {
	p, err := s.provider(provider)
	if err != nil {
		return "", err
	}
	return p.AuthCodeURL(state), nil
}

// OAuthCallback exchanges an authorisation code for a signed JWT.
func (s *Service) OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error) {
	p, err := s.provider(provider)
	if err != nil {
		return "", err
	}

	info, err := p.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("service: OAuthCallback: exchange: %w", err)
	}

	u, err := s.manager.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("service: OAuthCallback: resolve: %w", err)
	}

	token, err := s.signer.Sign(u)
	if err != nil {
		return "", fmt.Errorf("service: OAuthCallback: sign: %w", err)
	}
	return token, nil
}

// DeviceInit initiates the Device Authorization Grant flow for the given provider.
func (s *Service) DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	p, err := s.provider(provider)
	if err != nil {
		return nil, err
	}
	resp, err := p.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: DeviceInit: %w", err)
	}
	return resp, nil
}

// DevicePoll polls the provider for an authorised token and returns a signed JWT.
// Returns oauth2infra.ErrAuthorizationPending while the user has not approved yet.
func (s *Service) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error) {
	p, err := s.provider(provider)
	if err != nil {
		return "", err
	}

	info, err := p.DevicePoll(ctx, deviceCode)
	if err != nil {
		return "", fmt.Errorf("service: DevicePoll: %w", err)
	}

	u, err := s.manager.ResolveByOAuth(ctx, provider, info.ProviderID, info.Email, info.Name)
	if err != nil {
		return "", fmt.Errorf("service: DevicePoll: resolve: %w", err)
	}

	token, err := s.signer.Sign(u)
	if err != nil {
		return "", fmt.Errorf("service: DevicePoll: sign: %w", err)
	}
	return token, nil
}

// ExchangePAT validates a raw PAT and returns a signed JWT.
func (s *Service) ExchangePAT(ctx context.Context, rawPAT string) (string, error) {
	u, err := s.manager.ResolveByPAT(ctx, rawPAT)
	if err != nil {
		return "", fmt.Errorf("service: ExchangePAT: %w", err)
	}

	token, err := s.signer.Sign(u)
	if err != nil {
		return "", fmt.Errorf("service: ExchangePAT: sign: %w", err)
	}
	return token, nil
}

// RefreshJWT validates the current JWT and issues a new one with a fresh expiry.
func (s *Service) RefreshJWT(ctx context.Context, tokenStr string) (string, error) {
	claims, err := s.validator.Validate(tokenStr)
	if err != nil {
		return "", fmt.Errorf("service: RefreshJWT: validate: %w", err)
	}

	// Re-build a minimal user from claims to sign a new token.
	u := &user.User{
		Email: claims.Email,
		Role:  claims.Role,
	}
	// Parse the subject UUID.
	if id, err := parseUUID(claims.Subject); err == nil {
		u.ID = id
	}

	token, err := s.signer.Sign(u)
	if err != nil {
		return "", fmt.Errorf("service: RefreshJWT: sign: %w", err)
	}
	return token, nil
}

// provider returns the OAuth2 provider for the given key, or an error if not configured.
func (s *Service) provider(p user.Provider) (oauth2infra.Provider, error) {
	prov, ok := s.providers[p]
	if !ok {
		return nil, fmt.Errorf("service: provider %q not configured: %w", p, ErrProviderNotConfigured)
	}
	return prov, nil
}

// ErrProviderNotConfigured is returned when an OAuth2 provider is not enabled.
var ErrProviderNotConfigured = fmt.Errorf("provider not configured")
