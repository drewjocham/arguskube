// Package profiles persists per-user workspace profiles: named groups
// (e.g. "Daily", "Incident"), each containing variants that snapshot
// UI state (appearance, nav visibility, section tabs, UI prefs, saved
// filters). Frontend mirror lives at kube/view/src/stores/profiles.ts;
// the wire types here are kept structurally identical so JSON encoding
// passes straight through.
//
// Storage: SQLite tables on the shared argus.db handle. All rows are
// scoped by user_id so a SaaS instance can serve multiple users from
// one database. The opaque `snapshot_json` column holds the per-variant
// UI state — the backend never inspects its shape, only round-trips it,
// so adding fields on the frontend never requires a migration here.
//
// Active selection: one row per user in profile_active records the
// (group_id, variant_id) the user last applied. The frontend reads it
// on startup to restore the chosen profile across browser tabs and
// fresh installs.
package profiles

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// hashUserID returns the first 12 hex chars of SHA-256(userID). Used
// in structured logs so production grep/aggregation can correlate a
// user's profile activity without putting the raw ID in log files
// (which often end up in less-secure aggregators).
func hashUserID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:6])
}

// ErrNotFound is returned when a row is requested that does not exist
// for the given user. Callers use errors.Is so they can map it to a
// 404 / friendly empty response without parsing string messages.
var ErrNotFound = errors.New("profiles: not found")

// ErrQuotaExceeded is returned when a save would push the user past
// one of the per-user / per-group caps. Frontend surfaces the error
// message verbatim — keep it readable for end users.
var ErrQuotaExceeded = errors.New("profiles: quota exceeded")

// Per-user caps. Sized for the 50k-user production target where any
// one user's profile set is bounded so a buggy or hostile client
// can't fill the shared SQLite database.
//
// 100 groups × 50 variants × 64 KiB snapshot = ~320 MiB worst case
// per user. At 50k users that's 16 TiB worst-case — but the realistic
// median is two or three groups with a handful of variants and a
// snapshot under 4 KiB, putting the actual median per-user footprint
// in the single-MB range.
const (
	MaxGroupsPerUser    = 100
	MaxVariantsPerGroup = 50
	MaxSnapshotBytes    = 64 * 1024
)

// Group is a named collection of related variants. The frontend
// surfaces this as the top-level "Profile Groups" list — every
// variant always lives inside exactly one group.
type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Variants    []Variant `json:"variants"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Variant is one named snapshot within a group. The Snapshot field is
// a verbatim copy of the frontend's ProfileSnapshot — opaque to Go.
type Variant struct {
	ID          string          `json:"id"`
	ParentID    string          `json:"parentId"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	Snapshot    json.RawMessage `json:"snapshot"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// Active is the user's current selection. Both fields are empty
// strings when no profile has ever been applied.
type Active struct {
	GroupID   string    `json:"groupId"`
	VariantID string    `json:"variantId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store wraps the shared *sql.DB. Construction is a one-liner — the
// package never owns the connection.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewStore returns a Store that reads/writes against the supplied DB.
// A nil logger is replaced with slog.Default so callers don't have to
// pre-thread one through tests.
func NewStore(db *sql.DB, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{db: db, logger: logger}
}

// ListGroups returns every group + its variants for the user, ordered
// by name. Empty result is a valid response (zero-length slice, nil
// error) — callers should not treat it as a failure.
func (s *Store) ListGroups(ctx context.Context, userID string) ([]Group, error) {
	if userID == "" {
		return nil, errors.New("profiles: userID required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM profile_groups
		WHERE user_id = ?
		ORDER BY name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, fmt.Errorf("profiles: list groups: %w", err)
	}
	defer rows.Close()

	groups := make([]Group, 0)
	groupIdx := make(map[string]int) // id -> index in groups for variant attachment
	for rows.Next() {
		var g Group
		var created, updated int64
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &created, &updated); err != nil {
			return nil, fmt.Errorf("profiles: scan group: %w", err)
		}
		g.CreatedAt = time.Unix(created, 0).UTC()
		g.UpdatedAt = time.Unix(updated, 0).UTC()
		g.Variants = []Variant{}
		groupIdx[g.ID] = len(groups)
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profiles: iterate groups: %w", err)
	}

	if len(groups) == 0 {
		return groups, nil
	}

	// Pull every variant for this user in a single query, then attach.
	// One query rather than N+1; orders within each group by name.
	vrows, err := s.db.QueryContext(ctx, `
		SELECT v.id, v.group_id, v.name, v.description, v.version, v.snapshot_json,
		       v.created_at, v.updated_at
		FROM profile_variants v
		JOIN profile_groups g ON g.id = v.group_id
		WHERE g.user_id = ?
		ORDER BY v.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, fmt.Errorf("profiles: list variants: %w", err)
	}
	defer vrows.Close()

	for vrows.Next() {
		var v Variant
		var snapshot string
		var created, updated int64
		if err := vrows.Scan(&v.ID, &v.ParentID, &v.Name, &v.Description, &v.Version,
			&snapshot, &created, &updated); err != nil {
			return nil, fmt.Errorf("profiles: scan variant: %w", err)
		}
		v.Snapshot = json.RawMessage(snapshot)
		v.CreatedAt = time.Unix(created, 0).UTC()
		v.UpdatedAt = time.Unix(updated, 0).UTC()

		idx, ok := groupIdx[v.ParentID]
		if !ok {
			// Orphaned variant — would only happen if a delete races a
			// list. Drop it; the integrity of the user's view matters
			// more than completeness in that edge case.
			continue
		}
		groups[idx].Variants = append(groups[idx].Variants, v)
	}
	if err := vrows.Err(); err != nil {
		return nil, fmt.Errorf("profiles: iterate variants: %w", err)
	}

	return groups, nil
}

// SaveGroup is an upsert keyed by (user_id, id). When id is empty a
// new id is generated by the caller — the store treats it as opaque
// and only enforces uniqueness via PRIMARY KEY. The returned Group
// reflects post-save state (timestamps populated).
//
// SaveGroup never persists the Variants field — variant rows are
// owned by SaveVariant. Callers that want to bulk-replace variants
// must do so explicitly per variant; this mirrors the frontend store's
// fine-grained mutation API and avoids cascading delete surprises.
func (s *Store) SaveGroup(ctx context.Context, userID string, g Group) (Group, error) {
	if userID == "" {
		return Group{}, errors.New("profiles: userID required")
	}
	if g.ID == "" {
		return Group{}, errors.New("profiles: group ID required")
	}
	name := strings.TrimSpace(g.Name)
	if name == "" {
		return Group{}, errors.New("profiles: group name required")
	}

	// Quota check — only count rows the user doesn't already own.
	// An upsert of an existing row isn't a new row, so it must not
	// be rejected when the user is right at the cap.
	var (
		exists int
		count  int
	)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_groups WHERE user_id = ? AND id = ?`,
		userID, g.ID,
	).Scan(&exists); err != nil {
		return Group{}, fmt.Errorf("profiles: check existing: %w", err)
	}
	if exists == 0 {
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM profile_groups WHERE user_id = ?`, userID,
		).Scan(&count); err != nil {
			return Group{}, fmt.Errorf("profiles: count groups: %w", err)
		}
		if count >= MaxGroupsPerUser {
			return Group{}, fmt.Errorf("%w: max %d profile groups per user", ErrQuotaExceeded, MaxGroupsPerUser)
		}
	}

	now := time.Now().UTC()
	nowUnix := now.Unix()

	// UPSERT on the (user_id, id) primary key. created_at is preserved
	// across updates via excluded.created_at not being referenced; the
	// existing row's created_at stays put.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO profile_groups (user_id, id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			updated_at = excluded.updated_at`,
		userID, g.ID, name, strings.TrimSpace(g.Description), nowUnix, nowUnix,
	)
	if err != nil {
		return Group{}, fmt.Errorf("profiles: save group: %w", err)
	}

	s.logger.Info("profiles: save group",
		slog.String("user", hashUserID(userID)),
		slog.String("group", g.ID),
		slog.Bool("upsert", exists > 0),
	)

	// Re-read so the returned struct has the correct created_at when
	// this was an update (we asked SQLite to preserve it, but the
	// caller can't see the existing row's timestamp without a read).
	var (
		created, updated int64
		dbName, dbDesc   string
	)
	if err := s.db.QueryRowContext(ctx,
		`SELECT name, description, created_at, updated_at
		 FROM profile_groups WHERE user_id = ? AND id = ?`, userID, g.ID,
	).Scan(&dbName, &dbDesc, &created, &updated); err != nil {
		return Group{}, fmt.Errorf("profiles: read back group: %w", err)
	}

	g.Name = dbName
	g.Description = dbDesc
	g.CreatedAt = time.Unix(created, 0).UTC()
	g.UpdatedAt = time.Unix(updated, 0).UTC()
	g.Variants = nil
	return g, nil
}

// DeleteGroup removes a group and all its variants. SQLite foreign
// keys are not enabled in this codebase (PRAGMA foreign_keys is off
// by default and we don't set it), so the cascade is done explicitly
// in a transaction: variants first, then the group row, then the
// active-pointer scrub if it pointed here.
//
// Returns ErrNotFound if the group does not belong to this user.
func (s *Store) DeleteGroup(ctx context.Context, userID, groupID string) error {
	if userID == "" || groupID == "" {
		return errors.New("profiles: userID + groupID required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("profiles: begin tx: %w", err)
	}
	// Rollback is a no-op after Commit, so this is safe to defer.
	defer func() { _ = tx.Rollback() }()

	// Existence + ownership check first — return ErrNotFound without
	// touching variants when the group isn't ours.
	var owned int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_groups WHERE user_id = ? AND id = ?`,
		userID, groupID,
	).Scan(&owned); err != nil {
		return fmt.Errorf("profiles: check group ownership: %w", err)
	}
	if owned == 0 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_variants WHERE group_id = ?`, groupID,
	); err != nil {
		return fmt.Errorf("profiles: delete variants: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_groups WHERE user_id = ? AND id = ?`, userID, groupID,
	); err != nil {
		return fmt.Errorf("profiles: delete group: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_active WHERE user_id = ? AND group_id = ?`, userID, groupID,
	); err != nil {
		return fmt.Errorf("profiles: scrub active pointer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("profiles: commit delete: %w", err)
	}
	s.logger.Info("profiles: delete group",
		slog.String("user", hashUserID(userID)),
		slog.String("group", groupID),
	)
	return nil
}

// SaveVariant upserts a variant inside the named group. The group must
// already exist (FK constraint); callers should SaveGroup first. The
// snapshot is stored verbatim — empty or malformed JSON is the caller's
// responsibility to surface to its users.
func (s *Store) SaveVariant(ctx context.Context, userID, groupID string, v Variant) (Variant, error) {
	if userID == "" || groupID == "" {
		return Variant{}, errors.New("profiles: userID + groupID required")
	}
	if v.ID == "" {
		return Variant{}, errors.New("profiles: variant ID required")
	}
	name := strings.TrimSpace(v.Name)
	if name == "" {
		return Variant{}, errors.New("profiles: variant name required")
	}

	// Make sure the group exists AND belongs to this user. A foreign-
	// key check alone wouldn't catch the cross-user case (the group
	// might exist under a different user_id and the FK would happily
	// accept it). The explicit lookup keeps the multi-tenant boundary
	// tight.
	var owned int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_groups WHERE user_id = ? AND id = ?`,
		userID, groupID,
	).Scan(&owned); err != nil {
		return Variant{}, fmt.Errorf("profiles: check group ownership: %w", err)
	}
	if owned == 0 {
		return Variant{}, ErrNotFound
	}

	snapshot := string(v.Snapshot)
	if snapshot == "" {
		snapshot = "{}"
	}
	// Cap snapshot size — a 64 KiB payload covers every realistic UI
	// snapshot (the largest field, savedFilters, is a list of small
	// JSON entries the SavedFiltersStore itself caps at 50). The
	// limit protects the shared DB from a buggy client persisting
	// megabytes per variant.
	if len(snapshot) > MaxSnapshotBytes {
		return Variant{}, fmt.Errorf("%w: snapshot %d bytes exceeds %d-byte cap",
			ErrQuotaExceeded, len(snapshot), MaxSnapshotBytes)
	}
	version := strings.TrimSpace(v.Version)
	if version == "" {
		version = "1.0"
	}

	// Per-group variant quota check — only applies to net-new rows.
	var (
		variantExists int
		variantCount  int
	)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_variants WHERE id = ? AND group_id = ?`,
		v.ID, groupID,
	).Scan(&variantExists); err != nil {
		return Variant{}, fmt.Errorf("profiles: check existing variant: %w", err)
	}
	if variantExists == 0 {
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM profile_variants WHERE group_id = ?`, groupID,
		).Scan(&variantCount); err != nil {
			return Variant{}, fmt.Errorf("profiles: count variants: %w", err)
		}
		if variantCount >= MaxVariantsPerGroup {
			return Variant{}, fmt.Errorf("%w: max %d variants per group",
				ErrQuotaExceeded, MaxVariantsPerGroup)
		}
	}

	now := time.Now().UTC()
	nowUnix := now.Unix()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO profile_variants
			(id, group_id, name, description, version, snapshot_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			version = excluded.version,
			snapshot_json = excluded.snapshot_json,
			updated_at = excluded.updated_at`,
		v.ID, groupID, name, strings.TrimSpace(v.Description), version,
		snapshot, nowUnix, nowUnix,
	)
	if err != nil {
		return Variant{}, fmt.Errorf("profiles: save variant: %w", err)
	}

	s.logger.Info("profiles: save variant",
		slog.String("user", hashUserID(userID)),
		slog.String("group", groupID),
		slog.String("variant", v.ID),
		slog.Int("snapshot_bytes", len(snapshot)),
		slog.Bool("upsert", variantExists > 0),
	)

	var (
		created, updated   int64
		dbName, dbDesc, dv string
		dbSnap             string
	)
	if err := s.db.QueryRowContext(ctx,
		`SELECT name, description, version, snapshot_json, created_at, updated_at
		 FROM profile_variants WHERE id = ?`, v.ID,
	).Scan(&dbName, &dbDesc, &dv, &dbSnap, &created, &updated); err != nil {
		return Variant{}, fmt.Errorf("profiles: read back variant: %w", err)
	}

	v.ParentID = groupID
	v.Name = dbName
	v.Description = dbDesc
	v.Version = dv
	v.Snapshot = json.RawMessage(dbSnap)
	v.CreatedAt = time.Unix(created, 0).UTC()
	v.UpdatedAt = time.Unix(updated, 0).UTC()
	return v, nil
}

// DeleteVariant removes a variant. The variant must belong to a group
// owned by the user — cross-user deletes return ErrNotFound.
func (s *Store) DeleteVariant(ctx context.Context, userID, groupID, variantID string) error {
	if userID == "" || groupID == "" || variantID == "" {
		return errors.New("profiles: userID + groupID + variantID required")
	}
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM profile_variants
		WHERE id = ? AND group_id = ?
		  AND group_id IN (SELECT id FROM profile_groups WHERE user_id = ?)`,
		variantID, groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("profiles: delete variant: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// If this variant was the active one, clear the pointer.
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM profile_active WHERE user_id = ? AND variant_id = ?`, userID, variantID)
	s.logger.Info("profiles: delete variant",
		slog.String("user", hashUserID(userID)),
		slog.String("group", groupID),
		slog.String("variant", variantID),
	)
	return nil
}

// SetActive records the user's currently-applied profile. Both
// groupID and variantID may be empty to clear the selection.
func (s *Store) SetActive(ctx context.Context, userID, groupID, variantID string) error {
	if userID == "" {
		return errors.New("profiles: userID required")
	}
	nowUnix := time.Now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO profile_active (user_id, group_id, variant_id, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			group_id = excluded.group_id,
			variant_id = excluded.variant_id,
			updated_at = excluded.updated_at`,
		userID, groupID, variantID, nowUnix,
	)
	if err != nil {
		return fmt.Errorf("profiles: set active: %w", err)
	}
	return nil
}

// GetActive returns the user's active selection. Zero-value Active
// (empty group/variant IDs) when the user has never applied a profile;
// this is a normal state, not an error.
func (s *Store) GetActive(ctx context.Context, userID string) (Active, error) {
	if userID == "" {
		return Active{}, errors.New("profiles: userID required")
	}
	var (
		a       Active
		updated int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT group_id, variant_id, updated_at FROM profile_active WHERE user_id = ?`, userID,
	).Scan(&a.GroupID, &a.VariantID, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Active{}, nil
	}
	if err != nil {
		return Active{}, fmt.Errorf("profiles: get active: %w", err)
	}
	a.UpdatedAt = time.Unix(updated, 0).UTC()
	return a, nil
}
