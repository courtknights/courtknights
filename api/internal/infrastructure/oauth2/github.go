package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const githubUserURL = "https://api.github.com/user"

// GitHubConfig holds the configuration for the GitHub OAuth2 provider.
type GitHubConfig struct {
	// ClientID is the GitHub OAuth App client ID.
	ClientID string
	// ClientSecret is the GitHub OAuth App client secret.
	ClientSecret string
	// RedirectURL is the callback URL registered in the GitHub OAuth App.
	RedirectURL string
	// AuthURL overrides GitHub's authorization endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	AuthURL string
	// TokenURL overrides GitHub's token endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	TokenURL string
	// DeviceAuthURL overrides GitHub's device authorization endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	DeviceAuthURL string
	// UserInfoURL overrides GitHub's user endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	UserInfoURL string
}

// GitHubProvider implements Provider for GitHub OAuth2.
type GitHubProvider struct {
	cfg         *oauth2.Config
	httpClient  *http.Client
	userInfoURL string
}

// NewGitHub returns a GitHubProvider configured with the given credentials.
// httpClient is optional; if nil, http.DefaultClient is used.
func NewGitHub(cfg GitHubConfig, httpClient *http.Client) *GitHubProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	// Use AuthStyleInParams so client_id and client_secret are sent as form
	// body parameters on every request. AuthStyleAutoDetect (the default)
	// tries Basic-Auth first, which most mock servers and GitHub's own
	// device-flow endpoint do not accept.
	// DeviceAuthURL is not present in github.Endpoint, so we set it
	// explicitly following GitHub's device flow documentation.
	endpoint := oauth2.Endpoint{
		AuthURL:       github.Endpoint.AuthURL,
		TokenURL:      github.Endpoint.TokenURL,
		DeviceAuthURL: "https://github.com/login/device/code",
		AuthStyle:     oauth2.AuthStyleInParams,
	}
	if cfg.AuthURL != "" {
		endpoint.AuthURL = cfg.AuthURL
	}
	if cfg.TokenURL != "" {
		endpoint.TokenURL = cfg.TokenURL
	}
	if cfg.DeviceAuthURL != "" {
		endpoint.DeviceAuthURL = cfg.DeviceAuthURL
	}

	userInfoURL := githubUserURL
	if cfg.UserInfoURL != "" {
		userInfoURL = cfg.UserInfoURL
	}

	return &GitHubProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     endpoint,
		},
		httpClient:  httpClient,
		userInfoURL: userInfoURL,
	}
}

// ctxWithClient injects httpClient into the context so the oauth2 library uses
// it for all HTTP calls (token exchange, device auth, etc.).
func (g *GitHubProvider) ctxWithClient(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
}

// AuthCodeURL implements Provider.
func (g *GitHubProvider) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange implements Provider.
func (g *GitHubProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	ctx = g.ctxWithClient(ctx)
	token, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github: exchange: %w", err)
	}
	return g.fetchUserInfo(ctx, token.AccessToken)
}

// DeviceAuth implements Provider using GitHub's device authorisation flow.
func (g *GitHubProvider) DeviceAuth(ctx context.Context) (*DeviceAuthResponse, error) {
	ctx = g.ctxWithClient(ctx)
	resp, err := g.cfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: device auth: %w", err)
	}
	return toDeviceAuthResponse(resp), nil
}

// DevicePoll implements Provider. It makes a single token request and returns
// immediately; see Provider.DevicePoll for the errors it can return.
func (g *GitHubProvider) DevicePoll(ctx context.Context, deviceCode string) (*UserInfo, error) {
	accessToken, err := pollDeviceToken(ctx, g.httpClient, g.cfg, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("github: device poll: %w", err)
	}
	return g.fetchUserInfo(ctx, accessToken)
}

// fetchUserInfo retrieves the authenticated user's profile from the GitHub API.
//
// GET /user provides the stable numeric id, login and name. Its email field is
// only the user's public profile email (null when private), so the email is
// taken from GET /user/emails instead: the primary address, which must be
// verified. Otherwise it returns ErrEmailNotVerified.
func (g *GitHubProvider) fetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	var info struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := g.getJSON(ctx, g.userInfoURL, accessToken, &info); err != nil {
		return nil, fmt.Errorf("github: userinfo: %w", err)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := g.getJSON(ctx, g.emailsURL(), accessToken, &emails); err != nil {
		return nil, fmt.Errorf("github: emails: %w", err)
	}
	email := ""
	for _, e := range emails {
		if e.Primary && e.Verified {
			email = e.Email
			break
		}
	}
	if email == "" {
		return nil, fmt.Errorf("github: %w", ErrEmailNotVerified)
	}

	name := info.Name
	if name == "" {
		name = info.Login
	}

	return &UserInfo{
		ProviderID: fmt.Sprintf("%d", info.ID),
		Email:      email,
		Name:       name,
	}, nil
}

// emailsURL is the user-emails endpoint: GET /user/emails on GitHub, derived
// from the (possibly overridden) user endpoint so that local fakes work too.
func (g *GitHubProvider) emailsURL() string {
	return strings.TrimRight(g.userInfoURL, "/") + "/emails"
}

// getJSON performs an authenticated GET against the GitHub API and decodes the
// JSON body into out. Non-200 responses are errors.
func (g *GitHubProvider) getJSON(ctx context.Context, url, accessToken string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return nil
}
