package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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

// ---- OAuth2 provider mock ----

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

// ---- JWT signer / validator mocks ----

type mockJWTSigner struct{ mock.Mock }

func (m *mockJWTSigner) Sign(u *user.User) (string, error) {
	args := m.Called(u)
	return args.String(0), args.Error(1)
}

type mockJWTValidator struct{ mock.Mock }

func (m *mockJWTValidator) Validate(tokenStr string) (*jwtinfra.Claims, error) {
	args := m.Called(tokenStr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwtinfra.Claims), args.Error(1)
}

// ---- helpers ----

func newTestService(
	userRepo *mockUserRepository,
	patRepo *mockPATRepository,
	providers map[user.Provider]oauth2infra.Provider,
	signer *mockJWTSigner,
	validator *mockJWTValidator,
) *Service {
	mgr := NewManager(userRepo, patRepo)
	return NewService(mgr, providers, signer, validator)
}

// ---- OAuthRedirectURL ----

func TestService_OAuthRedirectURL_ReturnsURL(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("AuthCodeURL", "state-123").Return("https://accounts.google.com/auth?state=state-123")

	svc := newTestService(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov},
		&mockJWTSigner{}, &mockJWTValidator{},
	)

	url, err := svc.OAuthRedirectURL(user.ProviderGoogle, "state-123")
	require.NoError(t, err)
	assert.Contains(t, url, "state-123")
}

func TestService_OAuthRedirectURL_ReturnsErrWhenProviderNotConfigured(t *testing.T) {
	svc := newTestService(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{},
		&mockJWTSigner{}, &mockJWTValidator{},
	)

	_, err := svc.OAuthRedirectURL(user.ProviderGoogle, "state")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderNotConfigured)
}

// ---- OAuthCallback ----

func TestService_OAuthCallback_ValidCode_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	prov := &mockOAuth2Provider{}
	signer := &mockJWTSigner{}

	resolvedUser := &user.User{ID: uuid.New(), Email: "alice@example.com", Role: user.RoleUser}
	prov.On("Exchange", mock.Anything, "valid-code").
		Return(&oauth2infra.UserInfo{ProviderID: "g-1", Email: "alice@example.com", Name: "Alice"}, nil)
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*user.User")).Return(resolvedUser, nil)
	signer.On("Sign", resolvedUser).Return("signed-jwt", nil)

	svc := newTestService(
		userRepo, patRepo,
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov},
		signer, &mockJWTValidator{},
	)

	token, err := svc.OAuthCallback(context.Background(), user.ProviderGoogle, "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "signed-jwt", token)
}

func TestService_OAuthCallback_InvalidCode_ReturnsError(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("Exchange", mock.Anything, "bad-code").Return(nil, errors.New("invalid_grant"))

	svc := newTestService(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov},
		&mockJWTSigner{}, &mockJWTValidator{},
	)

	_, err := svc.OAuthCallback(context.Background(), user.ProviderGoogle, "bad-code")
	require.Error(t, err)
}

// ---- DeviceInit ----

func TestService_DeviceInit_ReturnsDeviceAuthResponse(t *testing.T) {
	prov := &mockOAuth2Provider{}
	expected := &oauth2infra.DeviceAuthResponse{
		DeviceCode:      "dev-code",
		UserCode:        "ABCD-1234",
		VerificationURI: "https://github.com/login/device",
	}
	prov.On("DeviceAuth", mock.Anything).Return(expected, nil)

	svc := newTestService(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov},
		&mockJWTSigner{}, &mockJWTValidator{},
	)

	resp, err := svc.DeviceInit(context.Background(), user.ProviderGitHub)
	require.NoError(t, err)
	assert.Equal(t, "ABCD-1234", resp.UserCode)
}

// ---- DevicePoll ----

func TestService_DevicePoll_Pending_ReturnsErrAuthorizationPending(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("DevicePoll", mock.Anything, "dev-code").
		Return(nil, fmt.Errorf("poll: %w", oauth2infra.ErrAuthorizationPending))

	svc := newTestService(
		&mockUserRepository{}, &mockPATRepository{},
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov},
		&mockJWTSigner{}, &mockJWTValidator{},
	)

	_, err := svc.DevicePoll(context.Background(), user.ProviderGitHub, "dev-code")
	require.Error(t, err)
	assert.ErrorIs(t, err, oauth2infra.ErrAuthorizationPending)
}

func TestService_DevicePoll_Authorised_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	prov := &mockOAuth2Provider{}
	signer := &mockJWTSigner{}

	resolvedUser := &user.User{ID: uuid.New(), Email: "bob@example.com", Role: user.RoleUser}
	prov.On("DevicePoll", mock.Anything, "dev-code").
		Return(&oauth2infra.UserInfo{ProviderID: "gh-42", Email: "bob@example.com", Name: "Bob"}, nil)
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*user.User")).Return(resolvedUser, nil)
	signer.On("Sign", resolvedUser).Return("device-jwt", nil)

	svc := newTestService(
		userRepo, patRepo,
		map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov},
		signer, &mockJWTValidator{},
	)

	token, err := svc.DevicePoll(context.Background(), user.ProviderGitHub, "dev-code")
	require.NoError(t, err)
	assert.Equal(t, "device-jwt", token)
}

// ---- ExchangePAT ----

func TestService_ExchangePAT_ValidPAT_ReturnsJWT(t *testing.T) {
	userRepo := &mockUserRepository{}
	patRepo := &mockPATRepository{}
	signer := &mockJWTSigner{}

	rawKey := "valid-pat-key"
	salt, _ := generateSalt()
	hash := hashKey(rawKey, salt)
	patID := uuid.New()
	linkedUser := &user.User{ID: uuid.New(), Email: "carol@example.com", Role: user.RoleUser}

	patRepo.On("FindAll", mock.Anything).Return([]*pat.PAT{{ID: patID, KeyHash: hash, Salt: salt}}, nil)
	userRepo.On("FindByProvider", mock.Anything, user.ProviderPAT, patID.String()).Return(linkedUser, nil)
	signer.On("Sign", linkedUser).Return("pat-jwt", nil)

	svc := newTestService(userRepo, patRepo, map[user.Provider]oauth2infra.Provider{}, signer, &mockJWTValidator{})

	token, err := svc.ExchangePAT(context.Background(), rawKey)
	require.NoError(t, err)
	assert.Equal(t, "pat-jwt", token)
}

func TestService_ExchangePAT_InvalidPAT_ReturnsError(t *testing.T) {
	patRepo := &mockPATRepository{}
	patRepo.On("FindAll", mock.Anything).Return([]*pat.PAT{}, nil)

	svc := newTestService(&mockUserRepository{}, patRepo, map[user.Provider]oauth2infra.Provider{}, &mockJWTSigner{}, &mockJWTValidator{})

	_, err := svc.ExchangePAT(context.Background(), "wrong-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrInvalidPAT)
}

// ---- RefreshJWT ----

func TestService_RefreshJWT_ValidToken_ReturnsNewJWT(t *testing.T) {
	signer := &mockJWTSigner{}
	validator := &mockJWTValidator{}

	userID := uuid.New()
	claims := &jwtinfra.Claims{}
	claims.Subject = userID.String()
	claims.Email = "dave@example.com"
	claims.Role = user.RoleUser

	validator.On("Validate", "old-token").Return(claims, nil)
	signer.On("Sign", mock.AnythingOfType("*user.User")).Return("new-jwt", nil)

	svc := newTestService(&mockUserRepository{}, &mockPATRepository{}, map[user.Provider]oauth2infra.Provider{}, signer, validator)

	token, err := svc.RefreshJWT(context.Background(), "old-token")
	require.NoError(t, err)
	assert.Equal(t, "new-jwt", token)
}

func TestService_RefreshJWT_ExpiredToken_ReturnsError(t *testing.T) {
	validator := &mockJWTValidator{}
	validator.On("Validate", "expired-token").Return(nil, fmt.Errorf("%w", ckerrors.ErrUnauthorized))

	svc := newTestService(&mockUserRepository{}, &mockPATRepository{}, map[user.Provider]oauth2infra.Provider{}, &mockJWTSigner{}, validator)

	_, err := svc.RefreshJWT(context.Background(), "expired-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrUnauthorized)
}

// avoid unused import for time
var _ = time.Now
