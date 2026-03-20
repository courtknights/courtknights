package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// AuthManager is the root manager for the authentication feature.
// It is the single entry point for all auth Handlers — no Handler may call
// a sub-manager directly.
//
// Rule: AuthManager calls other managers only — never repositories or
// infrastructure adapters directly.
type AuthManager interface {
	// OAuth2 redirect flow
	OAuthRedirectURL(provider user.Provider, state string) (string, error)
	OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error)

	// Device Authorization Grant (RFC 8628)
	DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error)
	DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error)

	// PAT exchange and JWT refresh
	ExchangePAT(ctx context.Context, rawPAT string) (string, error)
	RefreshJWT(ctx context.Context, tokenStr string) (string, error)

	// PAT lifecycle (used by PAT management handler)
	CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error)
	RevokePAT(ctx context.Context, id uuid.UUID) error
	ListPATs(ctx context.Context) ([]*pat.PAT, error)

	// User management
	UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error

	// Bootstrap
	BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error)
}

type authManager struct {
	user  UserManager
	jwt   JWTManager
	oauth OAuthManager
}

// NewAuthManager returns an AuthManager composed of the three sub-managers.
func NewAuthManager(u UserManager, j JWTManager, o OAuthManager) AuthManager {
	return &authManager{user: u, jwt: j, oauth: o}
}

func (m *authManager) OAuthRedirectURL(provider user.Provider, state string) (string, error) {
	return m.oauth.RedirectURL(provider, state)
}

func (m *authManager) OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error) {
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

func (m *authManager) DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error) {
	return m.oauth.DeviceAuth(ctx, provider)
}

func (m *authManager) DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error) {
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

func (m *authManager) ExchangePAT(ctx context.Context, rawPAT string) (string, error) {
	u, err := m.user.ResolveByPAT(ctx, rawPAT)
	if err != nil {
		return "", fmt.Errorf("auth manager: ExchangePAT: %w", err)
	}
	return m.jwt.Sign(u)
}

func (m *authManager) RefreshJWT(ctx context.Context, tokenStr string) (string, error) {
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

func (m *authManager) CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error) {
	return m.user.CreatePAT(ctx, userID, expiresAt)
}

func (m *authManager) RevokePAT(ctx context.Context, id uuid.UUID) error {
	return m.user.RevokePAT(ctx, id)
}

func (m *authManager) ListPATs(ctx context.Context) ([]*pat.PAT, error) {
	return m.user.ListPATs(ctx)
}

func (m *authManager) UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error {
	return m.user.UpdateUserRole(ctx, userID, role)
}

func (m *authManager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error) {
	return m.user.BootstrapAdmin(ctx, email, name, rawPAT)
}
