package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

// GoogleConfig holds the configuration for the Google OAuth2 provider.
type GoogleConfig struct {
	// ClientID is the Google OAuth2 client ID.
	ClientID string
	// ClientSecret is the Google OAuth2 client secret.
	ClientSecret string
	// RedirectURL is the callback URL registered in Google Cloud Console.
	RedirectURL string
	// AuthURL overrides Google's authorization endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	AuthURL string
	// TokenURL overrides Google's token endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	TokenURL string
	// DeviceAuthURL overrides Google's device authorization endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	DeviceAuthURL string
	// UserInfoURL overrides Google's userinfo endpoint.
	// Leave empty to use the production URL. Set for local mock servers.
	UserInfoURL string
}

// GoogleProvider implements Provider for Google OAuth2.
type GoogleProvider struct {
	cfg         *oauth2.Config
	httpClient  *http.Client
	userInfoURL string
}

// NewGoogle returns a GoogleProvider configured with the given credentials.
// httpClient is optional; if nil, http.DefaultClient is used.
func NewGoogle(cfg GoogleConfig, httpClient *http.Client) *GoogleProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	endpoint := oauth2.Endpoint{
		AuthURL:       google.Endpoint.AuthURL,
		TokenURL:      google.Endpoint.TokenURL,
		DeviceAuthURL: google.Endpoint.DeviceAuthURL,
		AuthStyle:     google.Endpoint.AuthStyle,
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

	userInfoURL := googleUserInfoURL
	if cfg.UserInfoURL != "" {
		userInfoURL = cfg.UserInfoURL
	}

	return &GoogleProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     endpoint,
		},
		httpClient:  httpClient,
		userInfoURL: userInfoURL,
	}
}

// ctxWithClient injects httpClient into the context so the oauth2 library uses
// it for all HTTP calls (token exchange, device auth, etc.).
func (g *GoogleProvider) ctxWithClient(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, g.httpClient)
}

// AuthCodeURL implements Provider.
func (g *GoogleProvider) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange implements Provider.
func (g *GoogleProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	ctx = g.ctxWithClient(ctx)
	token, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google: exchange: %w", err)
	}
	return g.fetchUserInfo(ctx, token.AccessToken)
}

// DeviceAuth implements Provider using Google's device authorisation endpoint.
func (g *GoogleProvider) DeviceAuth(ctx context.Context) (*DeviceAuthResponse, error) {
	ctx = g.ctxWithClient(ctx)
	resp, err := g.cfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("google: device auth: %w", err)
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
func (g *GoogleProvider) DevicePoll(ctx context.Context, deviceCode string) (*UserInfo, error) {
	ctx = g.ctxWithClient(ctx)
	deviceResp := &oauth2.DeviceAuthResponse{DeviceCode: deviceCode}
	token, err := g.cfg.DeviceAccessToken(ctx, deviceResp)
	if err != nil {
		return nil, fmt.Errorf("google: device poll: %w: %w", ErrAuthorizationPending, err)
	}
	return g.fetchUserInfo(ctx, token.AccessToken)
}

// fetchUserInfo retrieves the authenticated user's profile from Google's userinfo endpoint.
func (g *GoogleProvider) fetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google: userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google: read userinfo: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: userinfo: unexpected status %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("google: parse userinfo: %w", err)
	}

	return &UserInfo{
		ProviderID: info.Sub,
		Email:      info.Email,
		Name:       info.Name,
	}, nil
}
