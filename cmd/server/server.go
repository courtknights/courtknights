package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/courtknights/courtknights/internal/api"
	apiauth "github.com/courtknights/courtknights/internal/api/auth"
	"github.com/courtknights/courtknights/internal/api/common"
	"github.com/courtknights/courtknights/internal/api/pats"
	"github.com/courtknights/courtknights/internal/api/users"
	appauth "github.com/courtknights/courtknights/internal/application/auth"
	domainuser "github.com/courtknights/courtknights/internal/domain/user"
	"github.com/courtknights/courtknights/internal/infrastructure/jwt"
	"github.com/courtknights/courtknights/internal/infrastructure/oauth2"
	"github.com/courtknights/courtknights/internal/infrastructure/postgres"
)

// runServer wires all dependencies and starts the HTTP server.
func runServer(ctx context.Context, cfg Config) error {
	// ---- database ----
	db, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	// ---- repositories ----
	userRepo := postgres.NewUserRepository(db)
	patRepo := postgres.NewPATRepository(db)

	// ---- jwt adapter ----
	jwtAdapter, err := jwt.NewWithConfig(cfg.JWT.Secret, cfg.JWT.Expiry)
	if err != nil {
		return fmt.Errorf("jwt: %w", err)
	}

	// ---- oauth2 providers ----
	providers := buildProviders(cfg)

	// ---- application managers ----
	userManager := appauth.NewUserManager(userRepo, patRepo)
	jwtManager := appauth.NewJWTManager(jwtAdapter)
	oauthManager := appauth.NewOAuthManager(providers)
	authManager := appauth.NewAuthManager(userManager, jwtManager, oauthManager)

	// ---- bootstrap admin ----
	if err := runBootstrap(ctx, cfg.Bootstrap, authManager); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// ---- router ----
	router := api.New()
	router.MountPublic("/auth", apiauth.NewRoutes(apiauth.NewHandler(authManager)))
	router.MountProtected("/api/v1", common.JWTMiddleware(jwtManager), pats.NewRoutes(pats.NewHandler(authManager)))
	router.MountProtected("/api/v1", common.JWTMiddleware(jwtManager), users.NewRoutes(users.NewHandler(authManager)))

	// ---- start server ----
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("starting server on %s", addr)
	if err := router.Echo().Start(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

// buildProviders constructs the OAuth2 provider map from the given configuration.
// A provider is only registered when its client ID is set.
func buildProviders(cfg Config) map[domainuser.Provider]oauth2.Provider {
	providers := make(map[domainuser.Provider]oauth2.Provider)

	if cfg.Google.ClientID != "" {
		providers[domainuser.ProviderGoogle] = oauth2.NewGoogle(oauth2.GoogleConfig{
			ClientID:      cfg.Google.ClientID,
			ClientSecret:  cfg.Google.ClientSecret,
			RedirectURL:   cfg.Google.RedirectURL,
			AuthURL:       cfg.Google.AuthURL,
			TokenURL:      cfg.Google.TokenURL,
			DeviceAuthURL: cfg.Google.DeviceAuthURL,
		}, nil)
	}

	if cfg.GitHub.ClientID != "" {
		providers[domainuser.ProviderGitHub] = oauth2.NewGitHub(oauth2.GitHubConfig{
			ClientID:      cfg.GitHub.ClientID,
			ClientSecret:  cfg.GitHub.ClientSecret,
			RedirectURL:   cfg.GitHub.RedirectURL,
			AuthURL:       cfg.GitHub.AuthURL,
			TokenURL:      cfg.GitHub.TokenURL,
			DeviceAuthURL: cfg.GitHub.DeviceAuthURL,
		}, nil)
	}

	return providers
}

// runBootstrap creates the initial admin user when bootstrap credentials are provided.
// It is a no-op when any of the three required fields is empty.
func runBootstrap(ctx context.Context, cfg BootstrapConfig, mgr appauth.AuthManager) error {
	if cfg.Email == "" || cfg.Name == "" || cfg.PAT == "" {
		return nil
	}

	result, err := mgr.BootstrapAdmin(ctx, cfg.Email, cfg.Name, cfg.PAT)
	if err != nil {
		return fmt.Errorf("BootstrapAdmin: %w", err)
	}

	if result == "" {
		log.Println("bootstrap: skipped — users already exist")
	} else {
		log.Println("bootstrap: admin user created")
	}
	return nil
}
