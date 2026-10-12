package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// deviceCodeGrantType is the grant type for the Device Authorization Grant (RFC 8628 §3.4).
const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// Device-flow errors returned by DevicePoll, one per RFC 8628 §3.5 response.
// ErrAuthorizationPending and ErrSlowDown mean "poll again"; the others are terminal.
var (
	// ErrSlowDown is returned when the client polled too fast. It must add 5 seconds
	// to its polling interval before polling again.
	ErrSlowDown = errors.New("slow_down")
	// ErrAccessDenied is returned when the user denied the authorization request.
	ErrAccessDenied = errors.New("access_denied")
	// ErrDeviceCodeExpired is returned when the device code expired before the user
	// completed the authorization.
	ErrDeviceCodeExpired = errors.New("expired_token")
	// ErrInvalidDeviceCode is returned when the provider does not recognise the device code.
	ErrInvalidDeviceCode = errors.New("invalid device code")
)

// deviceTokenResponse covers both success and error token responses.
type deviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

// pollDeviceToken performs exactly one device-code token request and returns the
// access token, or the error matching the provider's RFC 8628 response.
//
// golang.org/x/oauth2's DeviceAccessToken is not used because it polls in a loop
// until the user approves, which would block the calling HTTP request (#128).
func pollDeviceToken(ctx context.Context, client *http.Client, cfg *oauth2.Config, deviceCode string) (string, error) {
	form := url.Values{
		"grant_type":  {deviceCodeGrantType},
		"device_code": {deviceCode},
		"client_id":   {cfg.ClientID},
	}
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("device token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// GitHub answers form-encoded unless JSON is requested explicitly.
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("device token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read device token response: %w", err)
	}

	tok, err := parseDeviceTokenResponse(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return "", fmt.Errorf("device token: status %d: %w", resp.StatusCode, err)
	}

	// Providers disagree on status codes: GitHub reports errors with 200, Google
	// uses 4xx (428 for authorization_pending). The error field is authoritative.
	switch tok.Error {
	case "":
		if tok.AccessToken == "" {
			return "", fmt.Errorf("device token: status %d: response has neither access_token nor error", resp.StatusCode)
		}
		return tok.AccessToken, nil
	case "authorization_pending":
		return "", ErrAuthorizationPending
	case "slow_down":
		return "", ErrSlowDown
	case "access_denied":
		return "", ErrAccessDenied
	case "expired_token":
		return "", ErrDeviceCodeExpired
	case "invalid_grant", "bad_verification_code", "incorrect_device_code":
		// bad_verification_code and incorrect_device_code are GitHub's codes.
		return "", ErrInvalidDeviceCode
	default:
		return "", fmt.Errorf("device token: status %d: provider error %q", resp.StatusCode, tok.Error)
	}
}

func parseDeviceTokenResponse(contentType string, body []byte) (deviceTokenResponse, error) {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "application/x-www-form-urlencoded" || mediaType == "text/plain" {
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return deviceTokenResponse{}, fmt.Errorf("parse form response: %w", err)
		}
		return deviceTokenResponse{AccessToken: vals.Get("access_token"), Error: vals.Get("error")}, nil
	}
	var tok deviceTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return deviceTokenResponse{}, fmt.Errorf("parse JSON response: %w", err)
	}
	return tok, nil
}

// toDeviceAuthResponse converts the library response, turning the absolute expiry
// into a lifetime in seconds.
func toDeviceAuthResponse(resp *oauth2.DeviceAuthResponse) *DeviceAuthResponse {
	expiresIn := 0
	if !resp.Expiry.IsZero() {
		expiresIn = int(time.Until(resp.Expiry).Round(time.Second).Seconds())
	}
	return &DeviceAuthResponse{
		DeviceCode:      resp.DeviceCode,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		ExpiresIn:       expiresIn,
		Interval:        int(resp.Interval),
	}
}
