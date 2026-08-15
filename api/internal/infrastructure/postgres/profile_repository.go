package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
// Only non-nil patch fields are applied. Returns the updated profile row.
func (r *ProfileRepository) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error) {
	setClauses := make([]string, 0, 8)
	args := make([]any, 0, 9)
	idx := 1

	if patch.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", idx))
		args = append(args, *patch.DisplayName)
		idx++
	}
	if patch.City != nil {
		setClauses = append(setClauses, fmt.Sprintf("city = $%d", idx))
		args = append(args, *patch.City)
		idx++
	}
	if patch.Region != nil {
		setClauses = append(setClauses, fmt.Sprintf("region = $%d", idx))
		args = append(args, *patch.Region)
		idx++
	}
	if patch.Country != nil {
		setClauses = append(setClauses, fmt.Sprintf("country = $%d", idx))
		args = append(args, *patch.Country)
		idx++
	}
	if patch.Gender != nil {
		setClauses = append(setClauses, fmt.Sprintf("gender = $%d", idx))
		args = append(args, *patch.Gender)
		idx++
	}
	if patch.DateOfBirth != nil {
		setClauses = append(setClauses, fmt.Sprintf("date_of_birth = $%d", idx))
		args = append(args, *patch.DateOfBirth)
		idx++
	}
	if patch.Category != nil {
		setClauses = append(setClauses, fmt.Sprintf("category = $%d", idx))
		args = append(args, *patch.Category)
		idx++
	}
	if patch.Preferences != nil {
		prefsJSON, err := json.Marshal(patch.Preferences)
		if err != nil {
			return nil, fmt.Errorf("postgres: Update profile marshal preferences: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("preferences = $%d", idx))
		args = append(args, prefsJSON)
		idx++
	}

	if len(setClauses) == 0 {
		// Nothing to update — return the current profile unchanged.
		return r.FindByUserID(ctx, userID)
	}

	// Always update updated_at.
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", idx))
	args = append(args, time.Now().UTC())
	idx++

	// Append userID as the final argument for the WHERE clause.
	args = append(args, userID)

	q := buildUpdateQuery(setClauses, idx)

	row := r.db.QueryRow(ctx, q, args...)
	p, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: Update profile: %w", ckerrors.ErrProfileNotFound)
		}
		return nil, fmt.Errorf("postgres: Update profile: %w", err)
	}
	return p, nil
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
		ORDER BY p.display_name ASC
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
