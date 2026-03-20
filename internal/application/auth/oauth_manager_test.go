package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/user"
	oauth2infra "github.com/courtknights/courtknights/internal/infrastructure/oauth2"
)

func TestOAuthManager_RedirectURL_ReturnsURL(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("AuthCodeURL", "state-abc").Return("https://accounts.google.com/auth?state=state-abc")

	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov})
	url, err := mgr.RedirectURL(user.ProviderGoogle, "state-abc")
	require.NoError(t, err)
	assert.Contains(t, url, "state-abc")
}

func TestOAuthManager_RedirectURL_ErrWhenProviderMissing(t *testing.T) {
	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{})
	_, err := mgr.RedirectURL(user.ProviderGoogle, "state")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProviderNotConfigured)
}

func TestOAuthManager_Exchange_ReturnsUserInfo(t *testing.T) {
	prov := &mockOAuth2Provider{}
	info := &oauth2infra.UserInfo{ProviderID: "g-1", Email: "alice@example.com", Name: "Alice"}
	prov.On("Exchange", mock.Anything, "code-123").Return(info, nil)

	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov})
	got, err := mgr.Exchange(context.Background(), user.ProviderGoogle, "code-123")
	require.NoError(t, err)
	assert.Equal(t, "g-1", got.ProviderID)
}

func TestOAuthManager_Exchange_PropagatesProviderError(t *testing.T) {
	prov := &mockOAuth2Provider{}
	prov.On("Exchange", mock.Anything, "bad").Return(nil, errors.New("invalid_grant"))

	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{user.ProviderGoogle: prov})
	_, err := mgr.Exchange(context.Background(), user.ProviderGoogle, "bad")
	require.Error(t, err)
}

func TestOAuthManager_DeviceAuth_ReturnsResponse(t *testing.T) {
	prov := &mockOAuth2Provider{}
	resp := &oauth2infra.DeviceAuthResponse{UserCode: "WXYZ-1234", VerificationURI: "https://github.com/login/device"}
	prov.On("DeviceAuth", mock.Anything).Return(resp, nil)

	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov})
	got, err := mgr.DeviceAuth(context.Background(), user.ProviderGitHub)
	require.NoError(t, err)
	assert.Equal(t, "WXYZ-1234", got.UserCode)
}

func TestOAuthManager_DevicePoll_ReturnsUserInfo(t *testing.T) {
	prov := &mockOAuth2Provider{}
	info := &oauth2infra.UserInfo{ProviderID: "gh-7", Email: "bob@example.com", Name: "Bob"}
	prov.On("DevicePoll", mock.Anything, "dev-code").Return(info, nil)

	mgr := NewOAuthManager(map[user.Provider]oauth2infra.Provider{user.ProviderGitHub: prov})
	got, err := mgr.DevicePoll(context.Background(), user.ProviderGitHub, "dev-code")
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", got.Email)
}
