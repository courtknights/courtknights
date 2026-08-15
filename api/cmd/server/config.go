package main

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for the server.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Google    OAuthConfig
	GitHub    OAuthConfig
	Bootstrap BootstrapConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port string
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string
}

// JWTConfig holds JWT signing settings.
type JWTConfig struct {
	Secret string
	Expiry time.Duration
}

// OAuthConfig holds OAuth2 provider credentials.
// The endpoint fields (AuthURL, TokenURL, DeviceAuthURL) are optional: when
// empty the provider's production URLs are used. Set them to point at a local
// mock server during development or integration testing.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// Optional endpoint overrides — leave empty to use the provider's defaults.
	AuthURL       string
	TokenURL      string
	DeviceAuthURL string
	UserInfoURL   string
}

// BootstrapConfig holds the one-time admin bootstrap parameters.
type BootstrapConfig struct {
	Email string
	Name  string
	PAT   string
}

// loadConfig reads all values from the provided Viper instance and returns a Config.
func loadConfig(v *viper.Viper) Config {
	return Config{
		Server: ServerConfig{
			Port: v.GetString("server.port"),
		},
		Database: DatabaseConfig{
			URL: v.GetString("database.url"),
		},
		JWT: JWTConfig{
			Secret: v.GetString("jwt.secret"),
			Expiry: v.GetDuration("jwt.expiry"),
		},
		Google: OAuthConfig{
			ClientID:      v.GetString("google.client_id"),
			ClientSecret:  v.GetString("google.client_secret"),
			RedirectURL:   v.GetString("google.redirect_url"),
			AuthURL:       v.GetString("google.auth_url"),
			TokenURL:      v.GetString("google.token_url"),
			DeviceAuthURL: v.GetString("google.device_auth_url"),
			UserInfoURL:   v.GetString("google.userinfo_url"),
		},
		GitHub: OAuthConfig{
			ClientID:      v.GetString("github.client_id"),
			ClientSecret:  v.GetString("github.client_secret"),
			RedirectURL:   v.GetString("github.redirect_url"),
			AuthURL:       v.GetString("github.auth_url"),
			TokenURL:      v.GetString("github.token_url"),
			DeviceAuthURL: v.GetString("github.device_auth_url"),
			UserInfoURL:   v.GetString("github.userinfo_url"),
		},
		Bootstrap: BootstrapConfig{
			Email: v.GetString("bootstrap.email"),
			Name:  v.GetString("bootstrap.name"),
			PAT:   v.GetString("bootstrap.pat"),
		},
	}
}
