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
	return githubTransport(tokenPayload, userPayload, verifiedEmails("unused@example.com"))
}

// githubTransport is tokenTransport plus GitHub's GET /user/emails, served for
// URLs ending in "/emails".
func githubTransport(tokenPayload, userPayload, emailsPayload any) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.String(), "token"):
			return jsonResp(http.StatusOK, tokenPayload), nil
		case strings.HasSuffix(r.URL.Path, "/emails"):
			return jsonResp(http.StatusOK, emailsPayload), nil
		default:
			return jsonResp(http.StatusOK, userPayload), nil
		}
	}
}

// verifiedEmails is a GitHub /user/emails payload with a single primary, verified address.
func verifiedEmails(email string) []map[string]any {
	return []map[string]any{{"email": email, "primary": true, "verified": true, "visibility": "private"}}
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
	userPayload := map[string]any{"sub": "google-123", "email": "alice@example.com", "email_verified": true, "name": "Alice"}

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
	userPayload := map[string]any{"id": int64(42), "login": "bob", "name": "Bob Builder", "email": nil}

	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
		&http.Client{Transport: githubTransport(tokenPayload, userPayload, verifiedEmails("bob@example.com"))})
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
	userPayload := map[string]any{"id": int64(7), "login": "charlie", "name": ""}

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
	userPayload := map[string]any{"sub": "google-123", "email": "alice@example.com", "email_verified": true, "name": "Alice"}

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
	userPayload := map[string]any{"id": int64(42), "login": "bob", "name": "Bob Builder"}

	p := NewGitHub(GitHubConfig{
		ClientID: "id", ClientSecret: "secret",
		UserInfoURL: "http://mock/github/userinfo",
	}, &http.Client{Transport: githubTransport(tokenPayload, userPayload, verifiedEmails("bob@example.com"))})
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

// ---- verified emails (#131) ----

func TestGoogleProvider_Exchange_RejectsUnverifiedOrMissingEmail(t *testing.T) {
	tests := map[string]map[string]any{
		"email_verified false":   {"sub": "g", "email": "a@example.com", "email_verified": false, "name": "A"},
		"email_verified missing": {"sub": "g", "email": "a@example.com", "name": "A"},
		"email missing":          {"sub": "g", "email_verified": true, "name": "A"},
	}
	for name, userPayload := range tests {
		t.Run(name, func(t *testing.T) {
			p := NewGoogle(GoogleConfig{ClientID: "id", ClientSecret: "secret"},
				&http.Client{Transport: tokenTransport(map[string]any{"access_token": "t", "token_type": "Bearer"}, userPayload)})
			p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

			info, err := p.Exchange(context.Background(), "code")
			assert.Nil(t, info)
			require.ErrorIs(t, err, ErrEmailNotVerified)
		})
	}
}

func TestGitHubProvider_Exchange_UsesPrimaryVerifiedEmail(t *testing.T) {
	emails := []map[string]any{
		{"email": "old@example.com", "primary": false, "verified": true},
		{"email": "main@example.com", "primary": true, "verified": true},
	}
	// The public profile email must be ignored in favour of /user/emails.
	userPayload := map[string]any{"id": 1, "login": "dana", "name": "Dana", "email": "public@example.com"}

	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
		&http.Client{Transport: githubTransport(map[string]any{"access_token": "t", "token_type": "bearer"}, userPayload, emails)})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	info, err := p.Exchange(context.Background(), "code")
	require.NoError(t, err)
	assert.Equal(t, "main@example.com", info.Email)
}

func TestGitHubProvider_Exchange_RejectsWhenNoPrimaryVerifiedEmail(t *testing.T) {
	tests := map[string][]map[string]any{
		"primary not verified": {
			{"email": "main@example.com", "primary": true, "verified": false},
			{"email": "other@example.com", "primary": false, "verified": true},
		},
		"no emails": {},
	}
	for name, emails := range tests {
		t.Run(name, func(t *testing.T) {
			p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"},
				&http.Client{Transport: githubTransport(map[string]any{"access_token": "t", "token_type": "bearer"},
					map[string]any{"id": 1, "login": "dana"}, emails)})
			p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

			info, err := p.Exchange(context.Background(), "code")
			assert.Nil(t, info)
			require.ErrorIs(t, err, ErrEmailNotVerified)
		})
	}
}

func TestGitHubProvider_Exchange_EmailsEndpointFailureIsNotUnverified(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.String(), "token"):
			return jsonResp(http.StatusOK, map[string]any{"access_token": "t", "token_type": "bearer"}), nil
		case strings.HasSuffix(r.URL.Path, "/emails"):
			return jsonResp(http.StatusNotFound, map[string]string{"message": "Not Found"}), nil
		default:
			return jsonResp(http.StatusOK, map[string]any{"id": 1, "login": "dana"}), nil
		}
	})
	p := NewGitHub(GitHubConfig{ClientID: "id", ClientSecret: "secret"}, &http.Client{Transport: transport})
	p.cfg.Endpoint = xoauth2.Endpoint{TokenURL: "http://fake/token"}

	_, err := p.Exchange(context.Background(), "code")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrEmailNotVerified)
}

func TestGitHubProvider_EmailsURLDerivedFromUserURL(t *testing.T) {
	assert.Equal(t, "https://api.github.com/user/emails", NewGitHub(GitHubConfig{}, nil).emailsURL())
	assert.Equal(t, "http://localhost:8090/github/user/emails",
		NewGitHub(GitHubConfig{UserInfoURL: "http://localhost:8090/github/user/"}, nil).emailsURL())
}
