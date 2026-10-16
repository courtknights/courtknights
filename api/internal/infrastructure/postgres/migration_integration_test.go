//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// applyMigrationRange runs, in order, the .up.sql files whose version is
// between minVersion and maxVersion inclusive. Versions are the zero-padded
// numeric file prefixes ("003").
func applyMigrationRange(ctx context.Context, t *testing.T, conn *pgxpool.Conn, minVersion, maxVersion string) {
	t.Helper()
	migrationsDir := filepath.Join("..", "..", "..", "..", "db", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var upFiles []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		if version := name[:len(maxVersion)]; version >= minVersion && version <= maxVersion {
			upFiles = append(upFiles, name)
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

// #136: migration 004 links each PAT to the user that could resolve it under
// the old model (users.provider_id = PAT id) and removes the orphan PATs.
func TestMigration004_BackfillsUserIDAndRemovesOrphans(t *testing.T) {
	ctx := context.Background()

	// Run the migrations in a dedicated schema so the shared test schema,
	// already at the latest version, is untouched.
	conn, err := testDB.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	for _, stmt := range []string{
		`DROP SCHEMA IF EXISTS migration004 CASCADE`,
		`CREATE SCHEMA migration004`,
		`SET search_path TO migration004`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	defer func() {
		_, _ = conn.Exec(ctx, `RESET search_path`)
		_, _ = conn.Exec(ctx, `DROP SCHEMA IF EXISTS migration004 CASCADE`)
	}()

	applyMigrationRange(ctx, t, conn, "001", "003")

	const (
		adminPAT  = "11111111-1111-1111-1111-111111111111"
		orphanPAT = "22222222-2222-2222-2222-222222222222"
		adminID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	)
	// Old model: the bootstrap admin's provider_id is its PAT id. The second
	// PAT was left behind when CreatePAT overwrote that provider_id.
	if _, err := conn.Exec(ctx, `
		INSERT INTO personal_access_tokens (id, key_hash, salt) VALUES ($1, 'h1', 's1'), ($2, 'h2', 's2');
	`, adminPAT, orphanPAT); err != nil {
		t.Fatalf("seed PATs: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (id, email, name, role, provider, provider_id) VALUES
			($1, 'admin@example.com', 'Admin', 'admin', 'pat', $2),
			(gen_random_uuid(), 'bob@example.com', 'Bob', 'user', 'google', 'g-123');
	`, adminID, adminPAT); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	applyMigrationRange(ctx, t, conn, "004", "004")

	rows, err := conn.Query(ctx, `SELECT id::text, user_id::text FROM personal_access_tokens`)
	if err != nil {
		t.Fatalf("query PATs: %v", err)
	}
	got := map[string]string{}
	for rows.Next() {
		var id, userID string
		if err := rows.Scan(&id, &userID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = userID
	}
	rows.Close()

	if len(got) != 1 {
		t.Fatalf("expected only the linked PAT to remain, got %v", got)
	}
	if got[adminPAT] != adminID {
		t.Errorf("admin PAT user_id: got %q, want %q", got[adminPAT], adminID)
	}
}

// #121: migration 005 accepts existing consistent profiles and then rejects a
// region without a country or from another country.
func TestMigration005_RegionCountryCheck(t *testing.T) {
	ctx := context.Background()

	conn, err := testDB.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	for _, stmt := range []string{
		`DROP SCHEMA IF EXISTS migration005 CASCADE`,
		`CREATE SCHEMA migration005`,
		`SET search_path TO migration005`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	defer func() {
		_, _ = conn.Exec(ctx, `RESET search_path`)
		_, _ = conn.Exec(ctx, `DROP SCHEMA IF EXISTS migration005 CASCADE`)
	}()

	applyMigrationRange(ctx, t, conn, "001", "004")

	// Valid combinations written before the constraint existed.
	if _, err := conn.Exec(ctx, `
		WITH u AS (
			INSERT INTO users (email, name, role, provider, provider_id) VALUES
				('a@example.com', 'A', 'user', 'google', 'a'),
				('b@example.com', 'B', 'user', 'google', 'b'),
				('c@example.com', 'C', 'user', 'google', 'c')
			RETURNING id, email
		)
		INSERT INTO user_profiles (user_id, display_name, country, region)
		SELECT id, email,
		       CASE email WHEN 'a@example.com' THEN 'ES' WHEN 'b@example.com' THEN 'FR' END,
		       CASE email WHEN 'a@example.com' THEN 'ES-MD' END
		FROM u`); err != nil {
		t.Fatalf("seed profiles: %v", err)
	}

	applyMigrationRange(ctx, t, conn, "005", "005")

	tests := []struct {
		name    string
		update  string
		wantErr bool
	}{
		{name: "region of the same country", update: `UPDATE user_profiles SET region = 'FR-IDF' WHERE country = 'FR'`},
		{name: "clear the region", update: `UPDATE user_profiles SET region = NULL WHERE country = 'ES'`},
		{name: "region without country", update: `UPDATE user_profiles SET region = 'ES-MD' WHERE country IS NULL`, wantErr: true},
		{name: "region of another country", update: `UPDATE user_profiles SET region = 'ES-MD' WHERE country = 'FR'`, wantErr: true},
		{name: "clear the country but keep the region", update: `UPDATE user_profiles SET country = NULL WHERE country = 'FR'`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conn.Exec(ctx, tt.update)
			if tt.wantErr && !isRegionCountryViolation(err) {
				t.Errorf("expected a region/country check violation, got: %v", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
