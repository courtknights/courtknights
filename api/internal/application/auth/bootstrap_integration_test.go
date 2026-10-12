//go:build integration

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/user"
	"github.com/courtknights/courtknights/internal/infrastructure/postgres"
)

var integrationDB *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("ck_auth_test"),
		tcpostgres.WithUsername("ck"),
		tcpostgres.WithPassword("ck"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get connection string: %v\n", err)
		os.Exit(1)
	}

	integrationDB, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open pool: %v\n", err)
		os.Exit(1)
	}
	defer integrationDB.Close()

	if err := applySchema(ctx, integrationDB); err != nil {
		fmt.Fprintf(os.Stderr, "apply schema: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// applySchema applies the real db/migrations so these tests run against the
// production schema (including the PAT → user foreign key, #136).
func applySchema(ctx context.Context, db *pgxpool.Pool) error {
	migrationsDir := filepath.Join("..", "..", "..", "..", "db", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)
	for _, name := range upFiles {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := integrationDB.Exec(context.Background(), "TRUNCATE users, personal_access_tokens CASCADE")
	require.NoError(t, err, "truncate tables")
}

func newUserManagerForIntegration() auth.UserManager {
	return auth.NewUserManager(
		postgres.NewUserRepository(integrationDB),
		postgres.NewPATRepository(integrationDB),
	)
}

// ---- bootstrap integration tests ----

func TestBootstrapAdmin_EmptyDB_CreatesAdminAndPAT(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()

	_, rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)
	assert.Equal(t, "bootstrap-secret", rawPAT, "raw PAT must be returned on first bootstrap")

	// Verify that exactly one user exists
	var count int64
	err = integrationDB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify the user has the admin role
	var role string
	err = integrationDB.QueryRow(context.Background(), "SELECT role FROM users WHERE email = $1", "admin@example.com").Scan(&role)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestBootstrapAdmin_NonEmptyDB_IsNoOp(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()

	// First bootstrap creates the admin
	_, _, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "secret-1")
	require.NoError(t, err)

	// Second bootstrap must be a no-op
	_, result, err := mgr.BootstrapAdmin(context.Background(), "other@example.com", "Other", "secret-2")
	require.NoError(t, err)
	assert.Empty(t, result, "second bootstrap must return empty string")

	// Still exactly one user
	var count int64
	err = integrationDB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "no duplicate users must be created")
}

func TestBootstrapAdmin_AdminCanAuthenticateWithBootstrapPAT(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()

	admin, rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)

	resolved, err := mgr.ResolveByPAT(context.Background(), rawPAT)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, resolved.ID)
	assert.Equal(t, user.RoleAdmin, resolved.Role)
}

// #136: a PAT created for an OAuth user resolves to that user, and the user's
// provider identity is untouched, so they can still sign in with Google.
func TestCreatePAT_OAuthUser_ResolvesToUserAndKeepsProviderIdentity(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()
	ctx := context.Background()

	googleUser, err := mgr.ResolveByOAuth(ctx, user.ProviderGoogle, "google-sub-1", "bob@example.com", "Bob")
	require.NoError(t, err)

	rawPAT, err := mgr.CreatePAT(ctx, googleUser.ID, nil)
	require.NoError(t, err)

	resolved, err := mgr.ResolveByPAT(ctx, rawPAT)
	require.NoError(t, err)
	assert.Equal(t, googleUser.ID, resolved.ID, "the PAT must resolve to the Google user")
	assert.Equal(t, user.ProviderGoogle, resolved.Provider)
	assert.Equal(t, "google-sub-1", resolved.ProviderID)

	again, err := mgr.ResolveByOAuth(ctx, user.ProviderGoogle, "google-sub-1", "bob@example.com", "Bob")
	require.NoError(t, err, "the user must still be able to sign in with Google")
	assert.Equal(t, googleUser.ID, again.ID)

	var users int64
	require.NoError(t, integrationDB.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&users))
	assert.Equal(t, int64(1), users, "no duplicate user may be created")
}

// #136: an unknown user is reported as ErrUserNotFound and leaves no orphan PAT.
func TestCreatePAT_UnknownUser_LeavesNoOrphan(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()
	ctx := context.Background()

	_, err := mgr.CreatePAT(ctx, uuid.New(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ckerrors.ErrUserNotFound), "got: %v", err)

	var pats int64
	require.NoError(t, integrationDB.QueryRow(ctx, "SELECT COUNT(*) FROM personal_access_tokens").Scan(&pats))
	assert.Equal(t, int64(0), pats)
}

// #132: signing in with GitHub using an email already registered through
// Google reports the existing provider and creates no second user.
func TestResolveByOAuth_EmailRegisteredWithOtherProvider_ReportsProvider(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()
	ctx := context.Background()

	_, err := mgr.ResolveByOAuth(ctx, user.ProviderGoogle, "google-sub-1", "hana@example.com", "Hana")
	require.NoError(t, err)

	_, err = mgr.ResolveByOAuth(ctx, user.ProviderGitHub, "github-1", "hana@example.com", "Hana")
	var conflict *ckerrors.EmailAlreadyRegisteredError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "google", conflict.Provider)

	var users int64
	require.NoError(t, integrationDB.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&users))
	assert.Equal(t, int64(1), users)
}

// #132: the bootstrap admin's provider ("pat") is not disclosed.
func TestResolveByOAuth_EmailOwnedByBootstrapAdmin_WithholdsProvider(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()
	ctx := context.Background()

	_, _, err := mgr.BootstrapAdmin(ctx, "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)

	_, err = mgr.ResolveByOAuth(ctx, user.ProviderGoogle, "google-admin", "admin@example.com", "Admin")
	var conflict *ckerrors.EmailAlreadyRegisteredError
	require.ErrorAs(t, err, &conflict)
	assert.Empty(t, conflict.Provider)
}
