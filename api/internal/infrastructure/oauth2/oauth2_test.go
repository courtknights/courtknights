package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xoauth2 "golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc creates an http.RoundTripper from a function.
type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// jsonResp builds an *http.Response with a JSON body.
func jsonResp(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	rec.WriteHeader(status)
	_, _ = rec.Write(b)
	return rec.Result()
}

// tokenTransport returns a RoundTripper that serves tokenPayload for requests
// whose URL contains "token", and userPayload for all other requests.
// Since ctxWithClient injects the transport into the oauth2 library, both the
// token exchange and the userinfo calls go through this transport.
func tokenTransport(tokenPayload, userPayload any) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.String(), "token") {
			return jsonResp(http.StatusOK, tokenPayload), nil
		}
		return jsonResp(http.StatusOK, userPayload), nil
	}
}

// ---- Google ----

func TestGoogleProvider_AuthCodeURL_ContainsGoogleDomain(t *testing.T) {
	p := NewGoogle(GoogleConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/auth/google/callback",
	}, nil)

	url := p.AuthCodeURL("state-abc")
	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "state-abc")
}

func TestGoogleProvider_Exchange_MapsUserInfo(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "fake-google-token", "token_type": "Bearer"}
	userPayload := map[string]string{"sub": "google-123", "email": "alice@example.com", "name": "Alice"}

	p := NewGoogle(GoogleConfig{ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/cb"},
		&http.Client{Transport: tokenTransport(tokenPayload, userPayload)})
	// Point token URL at a fake path that contains "token" so our transport routes it correctly.
	p.cfg.Endpoint = xoauth2.Endpoint{
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		TokenURL: "http://fake/token",
	}

	info, err := p.Exchange(context.Background(), "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "google-123", info.ProviderID)
	assert.Equal(t, "alice@example.com", info.Email)
	assert.Equal(t, "Alice", info.Name)
}

// ---- GitHub ----

func TestGitHubProvider_AuthCodeURL_ContainsGitHubDomain(t *testing.T) {
	p := NewGitHub(GitHubConfig{
		ClientID:     "gh-client-id",
		ClientSecret: "gh-secret",
		RedirectURL:  "http://localhost/auth/github/callback",
	}, nil)

	url := p.AuthCodeURL("state-xyz")
	assert.Contains(t, url, "github.com")
	assert.Contains(t, url, "state-xyz")
}

func TestGitHubProvider_Exchange_MapsUserInfo(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "gh-token", "token_type": "bearer"}
	userPayload := map[string]any{"id": int64(42), "login": "bob", "name": "Bob Builder", "email": "bob@example.com"}

	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
		&http.Client{Transport: tokenTransport(tokenPayload, userPayload)})
	p.cfg.Endpoint = xoauth2.Endpoint{
		AuthURL:  "https://github.com/login/oauth/authorize",
		TokenURL: "http://fake/token",
	}

	info, err := p.Exchange(context.Background(), "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "42", info.ProviderID)
	assert.Equal(t, "bob@example.com", info.Email)
	assert.Equal(t, "Bob Builder", info.Name)
}

func TestGitHubProvider_Exchange_FallbackNameToLogin(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "gh-token", "token_type": "bearer"}
	userPayload := map[string]any{"id": int64(7), "login": "charlie", "name": "", "email": "charlie@example.com"}

	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
		&http.Client{Transport: tokenTransport(tokenPayload, userPayload)})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	info, err := p.Exchange(context.Background(), "code")
	require.NoError(t, err)
	// When name is empty, login is used as fallback.
	assert.Equal(t, "charlie", info.Name)
}

func TestErrAuthorizationPending_Error(t *testing.T) {
	assert.Equal(t, "authorization_pending", ErrAuthorizationPending.Error())
}

// ---- UserInfoURL override + non-200 handling ----

func TestGoogleProvider_Exchange_UsesUserInfoURLOverride(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "fake-google-token", "token_type": "Bearer"}
	userPayload := map[string]string{"sub": "google-123", "email": "alice@example.com", "name": "Alice"}

	p := NewGoogle(GoogleConfig{
		ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/cb",
		UserInfoURL: "http://mock/google/userinfo",
	}, &http.Client{Transport: tokenTransport(tokenPayload, userPayload)})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	info, err := p.Exchange(context.Background(), "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", info.Email)
	assert.Equal(t, "http://mock/google/userinfo", p.userInfoURL)
}

func TestGoogleProvider_Exchange_NonOKUserInfoStatusFails(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "fake-google-token", "token_type": "Bearer"}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.String(), "token") {
			return jsonResp(http.StatusOK, tokenPayload), nil
		}
		return jsonResp(http.StatusUnauthorized, map[string]string{"error": "invalid_token"}), nil
	})

	p := NewGoogle(GoogleConfig{ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/cb"},
		&http.Client{Transport: transport})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	_, err := p.Exchange(context.Background(), "valid-code")
	require.Error(t, err)
}

func TestGitHubProvider_Exchange_UsesUserInfoURLOverride(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "gh-token", "token_type": "bearer"}
	userPayload := map[string]any{"id": int64(42), "login": "bob", "name": "Bob Builder", "email": "bob@example.com"}

	p := NewGitHub(GitHubConfig{
		ClientID: "id", ClientSecret: "secret",
		UserInfoURL: "http://mock/github/userinfo",
	}, &http.Client{Transport: tokenTransport(tokenPayload, userPayload)})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	info, err := p.Exchange(context.Background(), "valid-code")
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", info.Email)
	assert.Equal(t, "http://mock/github/userinfo", p.userInfoURL)
}

func TestGitHubProvider_Exchange_NonOKUserInfoStatusFails(t *testing.T) {
	tokenPayload := map[string]any{"access_token": "gh-token", "token_type": "bearer"}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.String(), "token") {
			return jsonResp(http.StatusOK, tokenPayload), nil
		}
		return jsonResp(http.StatusUnauthorized, map[string]string{"message": "Bad credentials"}), nil
	})

	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
		&http.Client{Transport: transport})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	_, err := p.Exchange(context.Background(), "valid-code")
	require.Error(t, err)
}
