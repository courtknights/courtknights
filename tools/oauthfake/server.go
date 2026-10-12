package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
)

const (
	providerGoogle = "google"
	providerGitHub = "github"

	grantAuthorizationCode = "authorization_code"
	grantDeviceCode        = "urn:ietf:params:oauth:grant-type:device_code"
)

// server is the HTTP front end of the fake. It serves every provider under its
// own path prefix (/google/..., /github/...).
type server struct {
	store   *store
	baseURL string
	mux     *http.ServeMux
}

// newServer wires all routes. baseURL is the externally reachable URL of the
// fake, used to build device verification URIs.
func newServer(st *store, baseURL string) *server {
	s := &server{store: st, baseURL: strings.TrimRight(baseURL, "/"), mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	s.mux.HandleFunc("GET /{provider}/authorize", s.withProvider(s.authorizeForm))
	s.mux.HandleFunc("POST /{provider}/authorize", s.withProvider(s.authorizeSubmit))
	s.mux.HandleFunc("POST /{provider}/token", s.withProvider(s.token))
	s.mux.HandleFunc("POST /{provider}/device/code", s.withProvider(s.deviceCode))
	s.mux.HandleFunc("GET /{provider}/device", s.withProvider(s.deviceForm))
	s.mux.HandleFunc("POST /{provider}/device", s.withProvider(s.deviceSubmit))
	s.mux.HandleFunc("POST /{provider}/device/approve", s.withProvider(s.deviceResolveAPI(true)))
	s.mux.HandleFunc("POST /{provider}/device/deny", s.withProvider(s.deviceResolveAPI(false)))
	s.mux.HandleFunc("GET /google/userinfo", s.googleUserInfo)
	s.mux.HandleFunc("GET /github/user", s.githubUser)
	s.mux.HandleFunc("GET /github/user/emails", s.githubUserEmails)
	return s
}

// ServeHTTP implements http.Handler and logs every request.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

type providerHandler func(w http.ResponseWriter, r *http.Request, provider string)

// withProvider rejects unknown providers with 404 and passes the provider on.
func (s *server) withProvider(h providerHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.PathValue("provider")
		if p != providerGoogle && p != providerGitHub {
			http.NotFound(w, r)
			return
		}
		h(w, r, p)
	}
}

// ---- redirect (authorization code) flow ----

// authorizeForm renders the login form, or approves immediately when the
// fake_user query parameter is set (scripted logins).
func (s *server) authorizeForm(w http.ResponseWriter, r *http.Request, provider string) {
	q := r.URL.Query()
	redirectURI, ok := validRedirect(q.Get("redirect_uri"))
	if !ok || q.Get("response_type") != "code" || q.Get("client_id") == "" {
		http.Error(w, "invalid authorization request: client_id, redirect_uri and response_type=code are required", http.StatusBadRequest)
		return
	}
	if login := q.Get("fake_user"); login != "" {
		id, _ := newIdentity(login, q.Get("fake_email"), q.Get("fake_name"))
		id = id.withEmailVerified(q.Get("fake_email_verified"))
		s.redirectWithCode(w, r, provider, redirectURI, q.Get("state"), id)
		return
	}
	render(w, authorizeTmpl, map[string]string{
		"Provider":    provider,
		"ClientID":    q.Get("client_id"),
		"RedirectURI": redirectURI.String(),
		"State":       q.Get("state"),
		"Scope":       q.Get("scope"),
	})
}

// authorizeSubmit handles the login form: approve issues a code, deny returns access_denied.
func (s *server) authorizeSubmit(w http.ResponseWriter, r *http.Request, provider string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	redirectURI, ok := validRedirect(r.PostForm.Get("redirect_uri"))
	if !ok {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	state := r.PostForm.Get("state")
	if r.PostForm.Get("action") == "deny" {
		v := url.Values{"error": {"access_denied"}}
		if state != "" {
			v.Set("state", state)
		}
		redirectURI.RawQuery = mergeQuery(redirectURI.RawQuery, v)
		http.Redirect(w, r, redirectURI.String(), http.StatusFound)
		return
	}
	id, ok := newIdentity(r.PostForm.Get("login"), r.PostForm.Get("email"), r.PostForm.Get("name"))
	if !ok {
		http.Error(w, "login is required", http.StatusBadRequest)
		return
	}
	id.EmailVerified = r.PostForm.Get("email_unverified") == ""
	s.redirectWithCode(w, r, provider, redirectURI, state, id)
}

func (s *server) redirectWithCode(w http.ResponseWriter, r *http.Request, provider string, redirectURI *url.URL, state string, id Identity) {
	code := s.store.issueCode(provider, redirectURI.String(), id)
	v := url.Values{"code": {code}}
	if state != "" {
		v.Set("state", state)
	}
	target := *redirectURI
	target.RawQuery = mergeQuery(target.RawQuery, v)
	http.Redirect(w, r, target.String(), http.StatusFound)
}

// ---- token endpoint ----

// token handles both the authorization_code and device_code grants.
// Client credentials are accepted without validation: this is a fake.
func (s *server) token(w http.ResponseWriter, r *http.Request, provider string) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, "invalid_request", "invalid form body")
		return
	}
	var (
		tok string
		err error
	)
	switch r.PostForm.Get("grant_type") {
	case grantAuthorizationCode:
		tok, err = s.store.redeemCode(provider, r.PostForm.Get("code"), r.PostForm.Get("redirect_uri"))
	case grantDeviceCode:
		tok, err = s.store.pollDevice(provider, r.PostForm.Get("device_code"))
	default:
		writeOAuthError(w, "unsupported_grant_type", "grant_type must be authorization_code or device_code")
		return
	}
	if err != nil {
		writeOAuthError(w, err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok,
		"token_type":   "bearer",
		"expires_in":   3600,
	})
}

// ---- device flow ----

// deviceCode starts a device authorization request (RFC 8628 §3.1–3.2).
// Google names the verification field "verification_url", GitHub "verification_uri";
// the fake mirrors each provider.
func (s *server) deviceCode(w http.ResponseWriter, _ *http.Request, provider string) {
	d := s.store.startDevice(provider)
	verificationField := "verification_uri"
	if provider == providerGoogle {
		verificationField = "verification_url"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":     d.deviceCode,
		"user_code":       d.userCode,
		verificationField: s.baseURL + "/" + provider + "/device",
		"expires_in":      int(d.expiresIn.Seconds()),
		"interval":        int(d.interval.Seconds()),
	})
}

// deviceForm renders the page where the user enters the user code.
func (s *server) deviceForm(w http.ResponseWriter, r *http.Request, provider string) {
	render(w, deviceTmpl, map[string]string{"Provider": provider, "UserCode": r.URL.Query().Get("user_code")})
}

// deviceSubmit handles the device page form.
func (s *server) deviceSubmit(w http.ResponseWriter, r *http.Request, provider string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	approve := r.PostForm.Get("action") != "deny"
	status, msg := s.resolveDevice(provider, r.PostForm, approve)
	w.WriteHeader(status)
	render(w, deviceResultTmpl, map[string]string{"Provider": provider, "Message": msg})
}

// deviceResolveAPI is the non-interactive approve/deny endpoint used by tests and scripts.
func (s *server) deviceResolveAPI(approve bool) providerHandler {
	return func(w http.ResponseWriter, r *http.Request, provider string) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		status, msg := s.resolveDevice(provider, r.PostForm, approve)
		if status == http.StatusOK {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, msg, status)
	}
}

func (s *server) resolveDevice(provider string, form url.Values, approve bool) (int, string) {
	var id Identity
	if approve {
		var ok bool
		id, ok = newIdentity(form.Get("login"), form.Get("email"), form.Get("name"))
		if !ok {
			return http.StatusBadRequest, "login is required to approve"
		}
		// The device page sends a checkbox; the test API sends email_verified=false.
		id = id.withEmailVerified(form.Get("email_verified"))
		if form.Get("email_unverified") != "" {
			id.EmailVerified = false
		}
	}
	err := s.store.resolveDevice(provider, form.Get("user_code"), approve, id)
	switch {
	case errors.Is(err, errUnknownCode):
		return http.StatusNotFound, "unknown user code"
	case errors.Is(err, errNotPending):
		return http.StatusConflict, "this code was already used or has expired"
	case approve:
		return http.StatusOK, "Device approved for " + id.Login + ". You can return to your terminal."
	default:
		return http.StatusOK, "Device request denied."
	}
}

// ---- user info ----

// googleUserInfo mirrors Google's OIDC userinfo response.
func (s *server) googleUserInfo(w http.ResponseWriter, r *http.Request) {
	id, ok := s.bearerIdentity(w, r, providerGoogle)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":            id.googleSub(),
		"email":          id.Email,
		"email_verified": id.EmailVerified,
		"name":           id.Name,
	})
}

// githubUser mirrors GitHub's GET /user response (the fields CourtKnights reads).
// email is always null, as for a user whose email is private (the GitHub
// default): clients must read GET /user/emails instead.
func (s *server) githubUser(w http.ResponseWriter, r *http.Request) {
	id, ok := s.bearerIdentity(w, r, providerGitHub)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    id.githubID(),
		"login": id.Login,
		"name":  id.Name,
		"email": nil,
	})
}

// githubUserEmails mirrors GitHub's GET /user/emails response.
func (s *server) githubUserEmails(w http.ResponseWriter, r *http.Request) {
	id, ok := s.bearerIdentity(w, r, providerGitHub)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{{
		"email":      id.Email,
		"primary":    true,
		"verified":   id.EmailVerified,
		"visibility": "private",
	}})
}

func (s *server) bearerIdentity(w http.ResponseWriter, r *http.Request, provider string) (Identity, bool) {
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return Identity{}, false
	}
	id, err := s.store.lookupToken(provider, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return Identity{}, false
	}
	return id, true
}

// ---- helpers ----

// validRedirect accepts only absolute http(s) URLs.
func validRedirect(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	return u, true
}

func mergeQuery(raw string, extra url.Values) string {
	v, _ := url.ParseQuery(raw)
	for k, vals := range extra {
		v[k] = vals
	}
	return v.Encode()
}

// writeOAuthError writes an RFC 6749 §5.2 error response.
func writeOAuthError(w http.ResponseWriter, code, description string) {
	body := map[string]string{"error": code}
	if description != "" {
		body["error_description"] = description
	}
	writeJSON(w, http.StatusBadRequest, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("render %s: %v", t.Name(), err)
	}
}
