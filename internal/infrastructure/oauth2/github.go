package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
}

// GitHubProvider implements Provider for GitHub OAuth2.
type GitHubProvider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
}

// NewGitHub returns a GitHubProvider configured with the given credentials.
// httpClient is optional; if nil, http.DefaultClient is used.
func NewGitHub(cfg GitHubConfig, httpClient *http.Client) *GitHubProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GitHubProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		},
		httpClient: httpClient,
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
	return &DeviceAuthResponse{
		DeviceCode:      resp.DeviceCode,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		ExpiresIn:       int(resp.Expiry.Unix()),
		Interval:        int(resp.Interval),
	}, nil
}

// DevicePoll implements Provider.
// Returns ErrAuthorizationPending while the user has not yet approved the request.
func (g *GitHubProvider) DevicePoll(ctx context.Context, deviceCode string) (*UserInfo, error) {
	ctx = g.ctxWithClient(ctx)
	deviceResp := &oauth2.DeviceAuthResponse{DeviceCode: deviceCode}
	token, err := g.cfg.DeviceAccessToken(ctx, deviceResp)
	if err != nil {
		return nil, fmt.Errorf("github: device poll: %w: %w", ErrAuthorizationPending, err)
	}
	return g.fetchUserInfo(ctx, token.AccessToken)
}

// fetchUserInfo retrieves the authenticated user's profile from the GitHub API.
func (g *GitHubProvider) fetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: userinfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github: read userinfo: %w", err)
	}

	var info struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("github: parse userinfo: %w", err)
	}

	name := info.Name
	if name == "" {
		name = info.Login
	}

	return &UserInfo{
		ProviderID: fmt.Sprintf("%d", info.ID),
		Email:      info.Email,
		Name:       name,
	}, nil
}
