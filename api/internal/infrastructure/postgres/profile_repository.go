package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// ProfileRepository implements profile.ProfileRepository using PostgreSQL.
type ProfileRepository struct {
	db *pgxpool.Pool
}

// NewProfileRepository returns a ProfileRepository backed by the given connection pool.
func NewProfileRepository(db *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// EnsureExists implements profile.ProfileRepository.
// It inserts a new profile row for the given user if one does not already exist.
// The call is idempotent: if a profile already exists for the user it is a no-op.
func (r *ProfileRepository) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error {
	const q = `
		INSERT INTO user_profiles (user_id, display_name)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`

	_, err := r.db.Exec(ctx, q, userID, displayName)
	if err != nil {
		return fmt.Errorf("postgres: EnsureExists profile: %w", err)
	}
	return nil
}

// FindByUserID implements profile.ProfileRepository.
// Returns ckerrors.ErrProfileNotFound when no profile exists for the given user.
func (r *ProfileRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error) {
	const q = `
		SELECT user_id, display_name, city, region, country, gender, date_of_birth,
		       category, preferences, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1`

	row := r.db.QueryRow(ctx, q, userID)
	p, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: FindByUserID profile: %w", ckerrors.ErrProfileNotFound)
		}
		return nil, fmt.Errorf("postgres: FindByUserID profile: %w", err)
	}
	return p, nil
}

// Update implements profile.ProfileRepository.
// It applies a JSON Merge Patch (ADR-015): a set field writes its value, a
// cleared field writes NULL, and preferences are merged into the stored JSONB
// inside the UPDATE so concurrent writes of different keys are not lost.
// An empty patch returns the current profile without writing.
func (r *ProfileRepository) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error) {
	if patch.IsEmpty() {
		return r.FindByUserID(ctx, userID)
	}

	var b updateBuilder
	if patch.DisplayName != nil {
		b.set("display_name", *patch.DisplayName)
	}
	setField(&b, "city", patch.City)
	setField(&b, "region", patch.Region)
	setField(&b, "country", patch.Country)
	setField(&b, "gender", patch.Gender)
	setField(&b, "date_of_birth", patch.DateOfBirth)
	setField(&b, "category", patch.Category)
	if err := b.mergePreferences(patch.Preferences); err != nil {
		return nil, fmt.Errorf("postgres: Update profile: %w", err)
	}
	b.set("updated_at", time.Now().UTC())

	b.args = append(b.args, userID)
	q := buildUpdateQuery(b.clauses, len(b.args))

	row := r.db.QueryRow(ctx, q, b.args...)
	p, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: Update profile: %w", ckerrors.ErrProfileNotFound)
		}
		if isRegionCountryViolation(err) {
			return nil, fmt.Errorf("postgres: Update profile: %w", ckerrors.ErrProfileConflict)
		}
		return nil, fmt.Errorf("postgres: Update profile: %w", err)
	}
	return p, nil
}

// updateBuilder collects the SET clauses of an UPDATE and their arguments.
type updateBuilder struct {
	clauses []string
	args    []any
}

// arg appends v to the arguments and returns its placeholder ($n).
func (b *updateBuilder) arg(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

// set adds "column = $n". A nil value writes NULL.
func (b *updateBuilder) set(column string, v any) {
	b.clauses = append(b.clauses, column+" = "+b.arg(v))
}

// setField adds a SET clause for a merge-patch field: nothing when the field
// is unchanged, NULL when it is cleared, its value otherwise.
func setField[T any](b *updateBuilder, column string, f profile.Field[T]) {
	if !f.Set {
		return
	}
	if f.Value == nil {
		b.set(column, nil)
		return
	}
	b.set(column, *f.Value)
}

// mergePreferences adds the SET clause that merges a preferences patch into
// the stored JSONB: start from {} when the patch clears all preferences, add
// the members with a value, then remove the members that are null.
func (b *updateBuilder) mergePreferences(p profile.PreferencesPatch) error {
	if p.IsEmpty() {
		return nil
	}
	set := map[string]any{}
	var remove []string
	mergePreference(set, &remove, "court_side", p.CourtSide)
	mergePreference(set, &remove, "handedness", p.Handedness)

	setJSON, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}
	if remove == nil {
		remove = []string{}
	}
	b.clauses = append(b.clauses, fmt.Sprintf(
		"preferences = ((CASE WHEN %s THEN '{}'::jsonb ELSE preferences END) || %s::jsonb) - %s::text[]",
		b.arg(p.Clear), b.arg(setJSON), b.arg(remove)))
	return nil
}

// mergePreference records one preference member under its JSON key (the same
// key as in profile.Preferences): in set when it has a value, in remove when
// it is cleared.
func mergePreference[T any](set map[string]any, remove *[]string, key string, f profile.Field[T]) {
	switch {
	case !f.Set:
	case f.Value == nil:
		*remove = append(*remove, key)
	default:
		set[key] = *f.Value
	}
}

// regionCountryConstraint is the CHECK constraint that keeps a profile's region
// within its country (db/migrations/005).
const regionCountryConstraint = "user_profiles_region_country_check"

// isRegionCountryViolation reports whether err is a violation of the
// region/country CHECK constraint.
func isRegionCountryViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23514" && // check_violation
		pgErr.ConstraintName == regionCountryConstraint
}

// buildUpdateQuery assembles the UPDATE statement from the pre-built SET clauses.
// idx is the next available parameter index (used for the WHERE clause).
func buildUpdateQuery(setClauses []string, whereIdx int) string {
	q := "UPDATE user_profiles SET "
	for i, clause := range setClauses {
		if i > 0 {
			q += ", "
		}
		q += clause
	}
	q += fmt.Sprintf(`
		WHERE user_id = $%d
		RETURNING user_id, display_name, city, region, country, gender, date_of_birth,
		          category, preferences, created_at, updated_at`, whereIdx)
	return q
}

// List implements profile.ProfileRepository.
// Returns a paginated list of profiles joined with the users table, ordered by display_name ASC.
// Ties are broken by user ID so that LIMIT/OFFSET pagination is stable across pages.
// The second return value is the total count of profiles across all pages.
func (r *ProfileRepository) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error) {
	offset := (params.Page - 1) * params.PageSize

	const countQ = `
		SELECT COUNT(*)
		FROM users u
		JOIN user_profiles p ON p.user_id = u.id`

	var totalCount int64
	if err := r.db.QueryRow(ctx, countQ).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("postgres: List profiles count: %w", err)
	}

	const q = `
		SELECT
			u.id,
			p.display_name,
			p.city,
			p.region,
			p.country,
			p.gender,
			p.category
		FROM users u
		JOIN user_profiles p ON p.user_id = u.id
		ORDER BY p.display_name ASC, u.id ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, params.PageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: List profiles: %w", err)
	}
	defer rows.Close()

	var items []*profile.ProfileListItem

	for rows.Next() {
		item, err := scanProfileListItem(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: List profiles scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: List profiles rows: %w", err)
	}

	if items == nil {
		items = []*profile.ProfileListItem{}
	}

	return items, totalCount, nil
}

// scanProfile reads a profile row from a pgx.Row.
func scanProfile(row pgx.Row) (*profile.Profile, error) {
	var p profile.Profile
	var gender *string
	var category *string
	var prefsJSON []byte
	var dateOfBirth *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&p.UserID,
		&p.DisplayName,
		&p.City,
		&p.Region,
		&p.Country,
		&gender,
		&dateOfBirth,
		&category,
		&prefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if gender != nil {
		g := profile.Gender(*gender)
		p.Gender = &g
	}
	if category != nil {
		c := profile.Category(*category)
		p.Category = &c
	}
	if dateOfBirth != nil {
		p.DateOfBirth = dateOfBirth
	}

	if len(prefsJSON) > 0 {
		if err := json.Unmarshal(prefsJSON, &p.Preferences); err != nil {
			return nil, fmt.Errorf("unmarshal preferences: %w", err)
		}
	}

	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt

	return &p, nil
}

// scanProfileListItem reads a profile list row from pgx.Rows.
func scanProfileListItem(rows pgx.Rows) (*profile.ProfileListItem, error) {
	var item profile.ProfileListItem
	var id uuid.UUID
	var displayName string
	var gender *string
	var category *string

	err := rows.Scan(
		&id,
		&displayName,
		&item.City,
		&item.Region,
		&item.Country,
		&gender,
		&category,
	)
	if err != nil {
		return nil, err
	}

	item.ID = &id
	item.DisplayName = &displayName

	if gender != nil {
		g := profile.Gender(*gender)
		item.Gender = &g
	}
	if category != nil {
		c := profile.Category(*category)
		item.Category = &c
	}

	return &item, nil
}
