package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
	jwtinfra "github.com/courtknights/courtknights/internal/infrastructure/jwt"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

// ---- mock OAuth2 provider (infrastructure level) ----

type mockOAuth2Provider struct{ mock.Mock }

func (m *mockOAuth2Provider) AuthCodeURL(state string) string {
	return m.Called(state).String(0)
}
func (m *mockOAuth2Provider) Exchange(ctx context.Context, code string) (*oauth2infra.UserInfo, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2infra.UserInfo), args.Error(1)
}
func (m *mockOAuth2Provider) DeviceAuth(ctx context.Context) (*oauth2infra.DeviceAuthResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2infra.DeviceAuthResponse), args.Error(1)
}
func (m *mockOAuth2Provider) DevicePoll(ctx context.Context, deviceCode string) (*oauth2infra.UserInfo, error) {
	args := m.Called(ctx, deviceCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2infra.UserInfo), args.Error(1)
}

// ---- mock JWT adapter (infrastructure level) ----

type mockJWTAdapter struct{ mock.Mock }

func (m *mockJWTAdapter) Sign(u *user.User) (string, error) {
	args := m.Called(u)
	return args.String(0), args.Error(1)
}
func (m *mockJWTAdapter) Validate(tokenStr string) (*jwtinfra.Claims, error) {
	args := m.Called(tokenStr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwtinfra.Claims), args.Error(1)
}

// ---- builder: returns AuthManager interface ----

func newTestAuthManager(
	userRepo *mockUserRepository,
	patRepo *mockPATRepository,
	providers map[user.Provider]oauth2infra.Provider,
	jwtAdapt *mockJWTAdapter,
) AuthManager {
	return NewAuthManager(
		NewUserManager(userRepo, patRepo),
		NewJWTManager(jwtAdapt),
		NewOAuthManager(providers),
	)
}

// ---- OAuthRedirectURL ----

func TestAuthManager_OAuthRedirectURL_ReturnsURL(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("AuthCodeURL", "state-123").Return("https://accounts.google.com/auth?state=state-123")

	mgr := newTestAuthManager(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov},
		&mockJWTAdapter{},
	)

	url, err := mgr.OAuthRedirectURL(user.ProviderGoogle, "state-123")
	require.NoError(t, err)
	assert.Contains(t, url, "state-123")
}

func TestAuthManager_OAuthRedirectURL_ErrWhenProviderNotConfigured(t *testing.T) {
	mgr := newTestAuthManager(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{},
		&mockJWTAdapter{},
	)
	_, err := mgr.OAuthRedirectURL(user.ProviderGoogle, "state")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderNotConfigured)
}

// ---- OAuthCallback ----

func TestAuthManager_OAuthCallback_ValidCode_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	prov := &mockOAuth2Provider{}
	jwtAdapt := &mockJWTAdapter{}

	resolved := &user.User{ID: uuid.New(), Email: "alice@example.com", Role: user.RoleUser}
	prov.On("Exchange", mock.Anything, "valid-code").
		Return(&oauth2infra.UserInfo{ProviderID: "g-1", Email: "alice@example.com", Name: "Alice"}, nil)
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*user.User")).Return(resolved, nil)
	jwtAdapt.On("Sign", resolved).Return("signed-jwt", nil)

	mgr := newTestAuthManager(userRepo, patRepo,
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov}, jwtAdapt)

	token, err := mgr.OAuthCallback(context.Background(), user.ProviderGoogle, "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "signed-jwt", token)
}

func TestAuthManager_OAuthCallback_InvalidCode_ReturnsError(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("Exchange", mock.Anything, "bad-code").Return(nil, errors.New("invalid_grant"))

	mgr := newTestAuthManager(&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov}, &mockJWTAdapter{})

	_, err := mgr.OAuthCallback(context.Background(), user.ProviderGoogle, "bad-code")
	require.Error(t, err)
}

// ---- DeviceInit ----

func TestAuthManager_DeviceInit_ReturnsDeviceAuthResponse(t *testing.T) {
	prov := &mockOAuth2Provider{}
	expected := &oauth2infra.DeviceAuthResponse{
		DeviceCode: "dev-code", UserCode: "ABCD-1234", VerificationURI: "https://github.com/login/device",
	}
	prov.On("DeviceAuth", mock.Anything).Return(expected, nil)

	mgr := newTestAuthManager(&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov}, &mockJWTAdapter{})

	resp, err := mgr.DeviceInit(context.Background(), user.ProviderGitHub)
	require.NoError(t, err)
	assert.Equal(t, "ABCD-1234", resp.UserCode)
}

// ---- DevicePoll ----

func TestAuthManager_DevicePoll_Pending_ReturnsErrAuthorizationPending(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("DevicePoll", mock.Anything, "dev-code").
		Return(nil, fmt.Errorf("poll: %w", oauth2infra.ErrAuthorizationPending))

	mgr := newTestAuthManager(&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov}, &mockJWTAdapter{})

	_, err := mgr.DevicePoll(context.Background(), user.ProviderGitHub, "dev-code")
	require.Error(t, err)
	assert.ErrorIs(t, err, oauth2infra.ErrAuthorizationPending)
}

func TestAuthManager_DevicePoll_Authorised_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	prov := &mockOAuth2Provider{}
	jwtAdapt := &mockJWTAdapter{}

	resolved := &user.User{ID: uuid.New(), Email: "bob@example.com", Role: user.RoleUser}
	prov.On("DevicePoll", mock.Anything, "dev-code").
		Return(&oauth2infra.UserInfo{ProviderID: "gh-42", Email: "bob@example.com", Name: "Bob"}, nil)
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*user.User")).Return(resolved, nil)
	jwtAdapt.On("Sign", resolved).Return("device-jwt", nil)

	mgr := newTestAuthManager(userRepo, patRepo,
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov}, jwtAdapt)

	token, err := mgr.DevicePoll(context.Background(), user.ProviderGitHub, "dev-code")
	require.NoError(t, err)
	assert.Equal(t, "device-jwt", token)
}

// ---- ExchangePAT ----

func TestAuthManager_ExchangePAT_ValidPAT_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	jwtAdapt := &mockJWTAdapter{}

	rawKey := "valid-pat-key"
	salt, _ := generateSalt()
	hash := hashKey(rawKey, salt)
	patID := uuid.New()
	linked := &user.User{ID: uuid.New(), Email: "carol@example.com", Role: user.RoleUser}

	patRepo.On("FindAll", mock.Anything).Return([]*pat.PAT{{ID: patID, KeyHash: hash, Salt: salt}}, nil)
	userRepo.On("FindByProvider", mock.Anything, user.ProviderPAT, patID.String()).Return(linked, nil)
	jwtAdapt.On("Sign", linked).Return("pat-jwt", nil)

	mgr := newTestAuthManager(userRepo, patRepo, map[user.Provider]oauth2infra.Provider{}, jwtAdapt)

	token, err := mgr.ExchangePAT(context.Background(), rawKey)
	require.NoError(t, err)
	assert.Equal(t, "pat-jwt", token)
}

func TestAuthManager_ExchangePAT_InvalidPAT_ReturnsErrInvalidPAT(t *testing.T) {
	patRepo := &mockPATRepository{}
	patRepo.On("FindAll", mock.Anything).Return([]*pat.PAT{}, nil)

	mgr := newTestAuthManager(&mockUserRepository{}, patRepo, map[user.Provider]oauth2infra.Provider{}, &mockJWTAdapter{})

	_, err := mgr.ExchangePAT(context.Background(), "wrong-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrInvalidPAT)
}

// ---- RefreshJWT ----

func TestAuthManager_RefreshJWT_ValidToken_ReturnsNewJWT(t *testing.T) {
	jwtAdapt := &mockJWTAdapter{}

	claims := &jwtinfra.Claims{}
	claims.Subject = uuid.New().String()
	claims.Email = "dave@example.com"
	claims.Role = user.RoleUser

	jwtAdapt.On("Validate", "old-token").Return(claims, nil)
	jwtAdapt.On("Sign", mock.AnythingOfType("*user.User")).Return("new-jwt", nil)

	mgr := newTestAuthManager(&mockUserRepository{}, &mockPATRepository{}, map[user.Provider]oauth2infra.Provider{}, jwtAdapt)

	token, err := mgr.RefreshJWT(context.Background(), "old-token")
	require.NoError(t, err)
	assert.Equal(t, "new-jwt", token)
}

func TestAuthManager_RefreshJWT_ExpiredToken_ReturnsErrUnauthorized(t *testing.T) {
	jwtAdapt := &mockJWTAdapter{}
	jwtAdapt.On("Validate", "expired-token").Return(nil, fmt.Errorf("%w", ckerrors.ErrUnauthorized))

	mgr := newTestAuthManager(&mockUserRepository{}, &mockPATRepository{}, map[user.Provider]oauth2infra.Provider{}, jwtAdapt)

	_, err := mgr.RefreshJWT(context.Background(), "expired-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrUnauthorized)
}
