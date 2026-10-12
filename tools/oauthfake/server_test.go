package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const redirectURI = "http://localhost:8080/auth/cb"

// fakeClock is a manually advanced clock for expiry and slow_down tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestServer(t *testing.T) (*httptest.Server, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	ts := httptest.NewServer(newServer(newStore(clock.Now, 10*time.Minute, time.Second), "http://fake.test"))
	t.Cleanup(ts.Close)
	return ts, clock
}

// noRedirect is an HTTP client that returns redirects instead of following them.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func authorizeURL(ts *httptest.Server, provider string, extra url.Values) string {
	v := url.Values{
		"client_id":     {"any-client"},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"state":         {"xyz"},
	}
	for k, vals := range extra {
		v[k] = vals
	}
	return ts.URL + "/" + provider + "/authorize?" + v.Encode()
}

// redirectQuery returns a function that checks a non-followed 302 response
// and returns the query of its Location header. It is curried so that it can
// take a (response, error) pair directly: redirectQuery(t)(client.Get(u)).
func redirectQuery(t *testing.T) func(*http.Response, error) url.Values {
	return func(resp *http.Response, err error) url.Values {
		t.Helper()
		return checkRedirect(t, resp, err)
	}
}

func checkRedirect(t *testing.T, resp *http.Response, err error) url.Values {
	t.Helper()
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 302 (%s)", resp.StatusCode, body)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != redirectURI {
		t.Fatalf("redirect target: got %q, want %q", got, redirectURI)
	}
	return loc.Query()
}

func postForm(t *testing.T, target string, form url.Values) (int, map[string]any) {
	t.Helper()
	resp, err := http.PostForm(target, form)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode %s: %v", target, err)
		}
	}
	return resp.StatusCode, body
}

func exchangeCode(t *testing.T, ts *httptest.Server, provider, code string) (int, map[string]any) {
	t.Helper()
	return postForm(t, ts.URL+"/"+provider+"/token", url.Values{
		"grant_type":   {grantAuthorizationCode},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

func userInfo(t *testing.T, ts *httptest.Server, path, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	if resp.StatusCode == http.StatusOK {
		_ = json.NewDecoder(resp.Body).Decode(&body)
	}
	return resp.StatusCode, body
}

// loginScripted runs the redirect flow with fake_user and returns an access token.
func loginScripted(t *testing.T, ts *httptest.Server, provider, login string) string {
	t.Helper()
	q := redirectQuery(t)(noRedirectGet(authorizeURL(ts, provider, url.Values{"fake_user": {login}})))
	if q.Get("state") != "xyz" {
		t.Fatalf("state: got %q, want xyz", q.Get("state"))
	}
	status, body := exchangeCode(t, ts, provider, q.Get("code"))
	if status != http.StatusOK {
		t.Fatalf("token exchange: status %d, body %v", status, body)
	}
	return body["access_token"].(string)
}

func noRedirectGet(target string) (*http.Response, error) { return noRedirect.Get(target) }

// ---- redirect flow ----

func TestRedirectFlow_Google_UserInfoShape(t *testing.T) {
	ts, _ := newTestServer(t)
	token := loginScripted(t, ts, providerGoogle, "alice")

	status, body := userInfo(t, ts, "/google/userinfo", token)
	if status != http.StatusOK {
		t.Fatalf("userinfo status: %d", status)
	}
	if body["sub"] != "fake-google-alice" || body["email"] != "alice@example.com" || body["name"] != "alice" {
		t.Errorf("userinfo: got %v", body)
	}
}

func TestRedirectFlow_GitHub_UserShapeWithNumericStableID(t *testing.T) {
	ts, _ := newTestServer(t)

	_, alice1 := userInfo(t, ts, "/github/user", loginScripted(t, ts, providerGitHub, "alice"))
	_, alice2 := userInfo(t, ts, "/github/user", loginScripted(t, ts, providerGitHub, "alice"))
	_, bob := userInfo(t, ts, "/github/user", loginScripted(t, ts, providerGitHub, "bob"))

	id, ok := alice1["id"].(float64)
	if !ok || id <= 0 || id != float64(int64(id)) {
		t.Fatalf("id must be a positive integer, got %v", alice1["id"])
	}
	if alice1["login"] != "alice" {
		t.Errorf("login: got %v", alice1["login"])
	}
	if alice1["id"] != alice2["id"] {
		t.Errorf("same login must keep the same id: %v vs %v", alice1["id"], alice2["id"])
	}
	if alice1["id"] == bob["id"] {
		t.Errorf("different logins must get different ids: both %v", bob["id"])
	}
}

func TestRedirectFlow_InteractiveForm(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, err := http.Get(authorizeURL(ts, providerGitHub, url.Values{"scope": {"read:user user:email"}}))
	if err != nil {
		t.Fatalf("GET authorize: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(page), `name="login"`) {
		t.Fatalf("expected login form, got %d: %s", resp.StatusCode, page)
	}

	form := url.Values{
		"client_id": {"any-client"}, "redirect_uri": {redirectURI}, "state": {"xyz"},
		"login": {"carol"}, "email": {"carol@club.test"}, "name": {"Carol C"}, "action": {"approve"},
	}
	q := redirectQuery(t)(noRedirect.PostForm(ts.URL+"/github/authorize", form))
	_, tok := exchangeCode(t, ts, providerGitHub, q.Get("code"))
	_, user := userInfo(t, ts, "/github/user", tok["access_token"].(string))
	if user["email"] != "carol@club.test" || user["name"] != "Carol C" {
		t.Errorf("user: got %v", user)
	}
}

func TestRedirectFlow_Deny_ReturnsAccessDenied(t *testing.T) {
	ts, _ := newTestServer(t)
	form := url.Values{"redirect_uri": {redirectURI}, "state": {"xyz"}, "action": {"deny"}}
	q := redirectQuery(t)(noRedirect.PostForm(ts.URL+"/google/authorize", form))
	if q.Get("error") != "access_denied" || q.Get("state") != "xyz" || q.Has("code") {
		t.Errorf("deny redirect query: got %v", q)
	}
}

func TestAuthorize_InvalidRequest_Returns400(t *testing.T) {
	ts, _ := newTestServer(t)
	tests := map[string]url.Values{
		"missing client_id":     {"client_id": {""}},
		"relative redirect_uri": {"redirect_uri": {"/cb"}},
		"wrong response_type":   {"response_type": {"token"}},
	}
	for name, override := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := http.Get(authorizeURL(ts, providerGoogle, override))
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestUnknownProvider_Returns404(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Get(authorizeURL(ts, "facebook", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestTokenExchange_InvalidGrants(t *testing.T) {
	ts, clock := newTestServer(t)
	issue := func(provider string) string {
		q := redirectQuery(t)(noRedirectGet(authorizeURL(ts, provider, url.Values{"fake_user": {"alice"}})))
		return q.Get("code")
	}

	t.Run("code is single-use", func(t *testing.T) {
		code := issue(providerGoogle)
		if s, _ := exchangeCode(t, ts, providerGoogle, code); s != http.StatusOK {
			t.Fatalf("first exchange: %d", s)
		}
		if s, body := exchangeCode(t, ts, providerGoogle, code); s != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Errorf("second exchange: %d %v", s, body)
		}
	})
	t.Run("code from another provider", func(t *testing.T) {
		code := issue(providerGoogle)
		if s, body := exchangeCode(t, ts, providerGitHub, code); s != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Errorf("got %d %v", s, body)
		}
	})
	t.Run("redirect_uri mismatch", func(t *testing.T) {
		code := issue(providerGoogle)
		s, body := postForm(t, ts.URL+"/google/token", url.Values{
			"grant_type": {grantAuthorizationCode}, "code": {code}, "redirect_uri": {"http://evil.test/cb"},
		})
		if s != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Errorf("got %d %v", s, body)
		}
	})
	t.Run("expired code", func(t *testing.T) {
		code := issue(providerGoogle)
		clock.Advance(11 * time.Minute)
		if s, body := exchangeCode(t, ts, providerGoogle, code); s != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Errorf("got %d %v", s, body)
		}
	})
	t.Run("unsupported grant type", func(t *testing.T) {
		s, body := postForm(t, ts.URL+"/google/token", url.Values{"grant_type": {"password"}})
		if s != http.StatusBadRequest || body["error"] != "unsupported_grant_type" {
			t.Errorf("got %d %v", s, body)
		}
	})
}

func TestUserInfo_RejectsMissingOrForeignTokens(t *testing.T) {
	ts, _ := newTestServer(t)
	googleToken := loginScripted(t, ts, providerGoogle, "alice")

	if s, _ := userInfo(t, ts, "/google/userinfo", ""); s != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", s)
	}
	if s, _ := userInfo(t, ts, "/google/userinfo", "bogus"); s != http.StatusUnauthorized {
		t.Errorf("bogus token: got %d, want 401", s)
	}
	if s, _ := userInfo(t, ts, "/github/user", googleToken); s != http.StatusUnauthorized {
		t.Errorf("google token at github: got %d, want 401", s)
	}
}

// ---- device flow ----

func startDevice(t *testing.T, ts *httptest.Server, provider string) map[string]any {
	t.Helper()
	s, body := postForm(t, ts.URL+"/"+provider+"/device/code", url.Values{"client_id": {"any-client"}})
	if s != http.StatusOK {
		t.Fatalf("device/code: %d %v", s, body)
	}
	return body
}

func pollDevice(t *testing.T, ts *httptest.Server, provider string, deviceCode any) (int, map[string]any) {
	t.Helper()
	return postForm(t, ts.URL+"/"+provider+"/token", url.Values{
		"grant_type": {grantDeviceCode}, "device_code": {deviceCode.(string)}, "client_id": {"any-client"},
	})
}

func TestDeviceCode_ResponseShapePerProvider(t *testing.T) {
	ts, _ := newTestServer(t)

	g := startDevice(t, ts, providerGoogle)
	if g["verification_url"] != "http://fake.test/google/device" || g["verification_uri"] != nil {
		t.Errorf("google must use verification_url: %v", g)
	}
	gh := startDevice(t, ts, providerGitHub)
	if gh["verification_uri"] != "http://fake.test/github/device" || gh["verification_url"] != nil {
		t.Errorf("github must use verification_uri: %v", gh)
	}
	if gh["expires_in"] != float64(600) || gh["interval"] != float64(1) {
		t.Errorf("expires_in/interval: %v", gh)
	}
	if code, _ := gh["user_code"].(string); len(code) != 9 || code[4] != '-' {
		t.Errorf("user_code format: %q", code)
	}
}

func TestDeviceFlow_ApproveThenToken(t *testing.T) {
	ts, clock := newTestServer(t)
	d := startDevice(t, ts, providerGitHub)

	if s, body := pollDevice(t, ts, providerGitHub, d["device_code"]); s != http.StatusBadRequest || body["error"] != "authorization_pending" {
		t.Fatalf("before approval: %d %v", s, body)
	}

	// Users may type the code in lower case and without the dash.
	typed := strings.ToLower(strings.ReplaceAll(d["user_code"].(string), "-", ""))
	s, _ := postForm(t, ts.URL+"/github/device/approve", url.Values{"user_code": {typed}, "login": {"dave"}})
	if s != http.StatusNoContent {
		t.Fatalf("approve: status %d", s)
	}

	clock.Advance(2 * time.Second)
	s, body := pollDevice(t, ts, providerGitHub, d["device_code"])
	if s != http.StatusOK {
		t.Fatalf("after approval: %d %v", s, body)
	}
	_, user := userInfo(t, ts, "/github/user", body["access_token"].(string))
	if user["login"] != "dave" {
		t.Errorf("user: got %v", user)
	}

	clock.Advance(2 * time.Second)
	if s, body := pollDevice(t, ts, providerGitHub, d["device_code"]); s != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("device code reused: %d %v", s, body)
	}
}

func TestDeviceFlow_ErrorResponses(t *testing.T) {
	ts, clock := newTestServer(t)

	t.Run("access_denied", func(t *testing.T) {
		d := startDevice(t, ts, providerGoogle)
		if s, _ := postForm(t, ts.URL+"/google/device/deny", url.Values{"user_code": {d["user_code"].(string)}}); s != http.StatusNoContent {
			t.Fatalf("deny: %d", s)
		}
		if s, body := pollDevice(t, ts, providerGoogle, d["device_code"]); s != http.StatusBadRequest || body["error"] != "access_denied" {
			t.Errorf("got %d %v", s, body)
		}
	})
	t.Run("slow_down", func(t *testing.T) {
		d := startDevice(t, ts, providerGoogle)
		pollDevice(t, ts, providerGoogle, d["device_code"])
		if s, body := pollDevice(t, ts, providerGoogle, d["device_code"]); s != http.StatusBadRequest || body["error"] != "slow_down" {
			t.Errorf("immediate second poll: %d %v", s, body)
		}
		// The interval grew by 5 s, so polling after 2 s is still too fast.
		clock.Advance(2 * time.Second)
		if _, body := pollDevice(t, ts, providerGoogle, d["device_code"]); body["error"] != "slow_down" {
			t.Errorf("poll within enlarged interval: %v", body)
		}
		clock.Advance(20 * time.Second)
		if _, body := pollDevice(t, ts, providerGoogle, d["device_code"]); body["error"] != "authorization_pending" {
			t.Errorf("poll after waiting: %v", body)
		}
	})
	t.Run("expired_token", func(t *testing.T) {
		d := startDevice(t, ts, providerGoogle)
		clock.Advance(11 * time.Minute)
		if s, body := pollDevice(t, ts, providerGoogle, d["device_code"]); s != http.StatusBadRequest || body["error"] != "expired_token" {
			t.Errorf("got %d %v", s, body)
		}
		if s, _ := postForm(t, ts.URL+"/google/device/approve", url.Values{"user_code": {d["user_code"].(string)}, "login": {"x"}}); s != http.StatusConflict {
			t.Errorf("approve expired: got %d, want 409", s)
		}
	})
	t.Run("unknown device code", func(t *testing.T) {
		if s, body := pollDevice(t, ts, providerGoogle, "nope"); s != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Errorf("got %d %v", s, body)
		}
	})
	t.Run("approve unknown or foreign user code", func(t *testing.T) {
		d := startDevice(t, ts, providerGoogle)
		if s, _ := postForm(t, ts.URL+"/github/device/approve", url.Values{"user_code": {d["user_code"].(string)}, "login": {"x"}}); s != http.StatusNotFound {
			t.Errorf("foreign provider: got %d, want 404", s)
		}
		if s, _ := postForm(t, ts.URL+"/google/device/approve", url.Values{"user_code": {"ZZZZ-ZZZZ"}, "login": {"x"}}); s != http.StatusNotFound {
			t.Errorf("unknown code: got %d, want 404", s)
		}
	})
	t.Run("approve requires login", func(t *testing.T) {
		d := startDevice(t, ts, providerGoogle)
		if s, _ := postForm(t, ts.URL+"/google/device/approve", url.Values{"user_code": {d["user_code"].(string)}}); s != http.StatusBadRequest {
			t.Errorf("got %d, want 400", s)
		}
	})
}

func TestDeviceFlow_InteractivePage(t *testing.T) {
	ts, clock := newTestServer(t)
	d := startDevice(t, ts, providerGoogle)

	resp, err := http.Get(ts.URL + "/google/device?user_code=" + url.QueryEscape(d["user_code"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(page), d["user_code"].(string)) {
		t.Errorf("device page should prefill the user code")
	}

	resp, err = http.PostForm(ts.URL+"/google/device", url.Values{"user_code": {d["user_code"].(string)}, "login": {"erin"}, "action": {"approve"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve via page: %d", resp.StatusCode)
	}
	clock.Advance(2 * time.Second)
	if s, body := pollDevice(t, ts, providerGoogle, d["device_code"]); s != http.StatusOK {
		t.Errorf("poll after page approval: %d %v", s, body)
	}
}

func TestHealthz(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", resp.StatusCode)
	}
}

func TestIdentity_Defaults(t *testing.T) {
	if _, ok := newIdentity("  ", "", ""); ok {
		t.Error("blank login must be rejected")
	}
	id, _ := newIdentity(" frank ", "", "")
	if id.Login != "frank" || id.Email != "frank@example.com" || id.Name != "frank" {
		t.Errorf("defaults: got %+v", id)
	}
	if id.githubID() >= 1<<53 {
		t.Errorf("githubID must fit in 53 bits: %d", id.githubID())
	}
}
