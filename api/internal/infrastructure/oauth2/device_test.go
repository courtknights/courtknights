package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deviceTestServer serves a token endpoint driven by respond, plus Google- and
// GitHub-shaped userinfo endpoints. It counts token requests.
func deviceTestServer(t *testing.T, respond func(w http.ResponseWriter)) (*httptest.Server, *atomic.Int32, *url.Values) {
	t.Helper()
	var calls atomic.Int32
	var lastForm url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.NoError(t, r.ParseForm())
		lastForm = r.PostForm
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		respond(w)
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "g-1", "email": "a@example.com", "name": "A"})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "login": "a", "name": "A", "email": "a@example.com"})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, &calls, &lastForm
}

func jsonBody(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func formBody(body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
}

func newDeviceProviders(baseURL string) map[string]Provider {
	return map[string]Provider{
		"google": NewGoogle(GoogleConfig{
			ClientID: "cid", ClientSecret: "csecret",
			TokenURL: baseURL + "/token", UserInfoURL: baseURL + "/userinfo",
		}, nil),
		"github": NewGitHub(GitHubConfig{
			ClientID: "cid", ClientSecret: "csecret",
			TokenURL: baseURL + "/token", UserInfoURL: baseURL + "/user",
		}, nil),
	}
}

func TestDevicePoll_MapsRFC8628Responses(t *testing.T) {
	tests := []struct {
		name    string
		respond func(http.ResponseWriter)
		wantErr error
	}{
		// Google answers authorization_pending with 428; GitHub with 200.
		{"pending (Google, 428)", jsonBody(http.StatusPreconditionRequired, `{"error":"authorization_pending"}`), ErrAuthorizationPending},
		{"pending (GitHub, 200)", jsonBody(http.StatusOK, `{"error":"authorization_pending"}`), ErrAuthorizationPending},
		{"pending (form-encoded)", formBody("error=authorization_pending"), ErrAuthorizationPending},
		{"slow_down", jsonBody(http.StatusBadRequest, `{"error":"slow_down"}`), ErrSlowDown},
		{"access_denied", jsonBody(http.StatusForbidden, `{"error":"access_denied"}`), ErrAccessDenied},
		{"expired_token", jsonBody(http.StatusBadRequest, `{"error":"expired_token"}`), ErrDeviceCodeExpired},
		{"invalid_grant", jsonBody(http.StatusBadRequest, `{"error":"invalid_grant"}`), ErrInvalidDeviceCode},
		{"GitHub bad_verification_code", jsonBody(http.StatusOK, `{"error":"bad_verification_code"}`), ErrInvalidDeviceCode},
	}
	for _, tc := range tests {
		for name, provider := range map[string]func(string) Provider{
			"google": func(u string) Provider { return newDeviceProviders(u)["google"] },
			"github": func(u string) Provider { return newDeviceProviders(u)["github"] },
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				ts, calls, _ := deviceTestServer(t, tc.respond)

				info, err := provider(ts.URL).DevicePoll(context.Background(), "dev-code")

				assert.Nil(t, info)
				require.ErrorIs(t, err, tc.wantErr)
				for _, other := range []error{ErrAuthorizationPending, ErrSlowDown, ErrAccessDenied, ErrDeviceCodeExpired, ErrInvalidDeviceCode} {
					if !errors.Is(tc.wantErr, other) {
						assert.NotErrorIs(t, err, other, "error must not also match %v", other)
					}
				}
				assert.Equal(t, int32(1), calls.Load(), "exactly one token request")
			})
		}
	}
}

func TestDevicePoll_UnknownOrMalformedResponse_IsNotAProtocolError(t *testing.T) {
	tests := map[string]func(http.ResponseWriter){
		"unknown error code": jsonBody(http.StatusBadRequest, `{"error":"server_error"}`),
		"no token, no error": jsonBody(http.StatusOK, `{}`),
		"invalid JSON":       jsonBody(http.StatusInternalServerError, `<html>oops</html>`),
	}
	for name, respond := range tests {
		t.Run(name, func(t *testing.T) {
			ts, _, _ := deviceTestServer(t, respond)
			_, err := newDeviceProviders(ts.URL)["google"].DevicePoll(context.Background(), "dev-code")
			require.Error(t, err)
			for _, sentinel := range []error{ErrAuthorizationPending, ErrSlowDown, ErrAccessDenied, ErrDeviceCodeExpired, ErrInvalidDeviceCode} {
				assert.NotErrorIs(t, err, sentinel)
			}
		})
	}
}

// Regression for #128: DevicePoll must return immediately instead of looping
// until the user approves.
func TestDevicePoll_PendingReturnsImmediately(t *testing.T) {
	ts, calls, _ := deviceTestServer(t, jsonBody(http.StatusOK, `{"error":"authorization_pending"}`))

	start := time.Now()
	_, err := newDeviceProviders(ts.URL)["github"].DevicePoll(context.Background(), "dev-code")

	require.ErrorIs(t, err, ErrAuthorizationPending)
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, int32(1), calls.Load())
}

func TestDevicePoll_Approved_SendsDeviceGrantAndMapsUserInfo(t *testing.T) {
	ts, _, form := deviceTestServer(t, jsonBody(http.StatusOK, `{"access_token":"at","token_type":"bearer"}`))
	providers := newDeviceProviders(ts.URL)

	g, err := providers["google"].DevicePoll(context.Background(), "dev-code")
	require.NoError(t, err)
	assert.Equal(t, &UserInfo{ProviderID: "g-1", Email: "a@example.com", Name: "A"}, g)
	assert.Equal(t, deviceCodeGrantType, form.Get("grant_type"))
	assert.Equal(t, "dev-code", form.Get("device_code"))
	assert.Equal(t, "cid", form.Get("client_id"))
	assert.Equal(t, "csecret", form.Get("client_secret"))

	gh, err := providers["github"].DevicePoll(context.Background(), "dev-code")
	require.NoError(t, err)
	assert.Equal(t, &UserInfo{ProviderID: "42", Email: "a@example.com", Name: "A"}, gh)
}

// Regression for #128: ExpiresIn is a lifetime in seconds, not a Unix timestamp.
func TestDeviceAuth_ExpiresInIsSeconds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /device/code", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-EFGH","verification_uri":"https://example.test/device","expires_in":900,"interval":5}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	for name, p := range map[string]Provider{
		"google": NewGoogle(GoogleConfig{ClientID: "cid", DeviceAuthURL: ts.URL + "/device/code"}, nil),
		"github": NewGitHub(GitHubConfig{ClientID: "cid", DeviceAuthURL: ts.URL + "/device/code"}, nil),
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := p.DeviceAuth(context.Background())
			require.NoError(t, err)
			assert.InDelta(t, 900, resp.ExpiresIn, 1)
			assert.Equal(t, 5, resp.Interval)
			assert.Equal(t, "dc", resp.DeviceCode)
			assert.Equal(t, "ABCD-EFGH", resp.UserCode)
			assert.Equal(t, "https://example.test/device", resp.VerificationURI)
		})
	}
}
