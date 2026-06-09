package database

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var viewWhitespace = regexp.MustCompile(`\s+`)

// canonicalViewDDL normalizes a stored CREATE VIEW statement for comparison. It
// drops the `IF NOT EXISTS` that schema.sql carries (the migration scripts omit
// it) and collapses whitespace. Nothing else is normalized — including
// identifier quoting — so schema.sql and the migration chain must stay in
// lockstep down to the bracketing.
func canonicalViewDDL(sql string) string {
	normalized := strings.ReplaceAll(sql, "IF NOT EXISTS ", "")
	normalized = viewWhitespace.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func viewDefinitions(ctx context.Context, t *testing.T, db *Database) map[string]string {
	t.Helper()
	rows, err := db.DB.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type = 'view' ORDER BY name`)
	if err != nil {
		t.Fatalf("query view definitions: %v", err)
	}
	defer rows.Close()
	defs := map[string]string{}
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatalf("scan view definition: %v", err)
		}
		defs[name] = canonicalViewDDL(sql)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate view definitions: %v", err)
	}
	return defs
}

// TestFreshVsMigratedViewParity guards a hard invariant of the migration design:
// schema.sql (applied ONLY to genuinely fresh databases, stamped at the latest
// version) and the migration chain (applied to existing databases, never
// touching schema.sql) must produce IDENTICAL view definitions. A fresh install
// runs schema.sql and stops; an upgrading install reaches the latest version
// purely through the N_from_(N-1).sql scripts. If the two sources drift, fresh
// and existing users get different view behavior from the same release (e.g. the
// historical comments_url incident). Every view is compared, so a view added to
// only one source is also caught.
func TestFreshVsMigratedViewParity(t *testing.T) {
	ctx := context.Background()

	fresh := &Database{Path: filepath.Join("testdata", "parity_fresh.db")}
	if err := fresh.Destroy(); err != nil {
		t.Fatalf("destroy fresh: %v", err)
	}
	if err := fresh.Open(ctx); err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	t.Cleanup(func() { fresh.Close(); fresh.Destroy() })
	if err := fresh.Initialize(ctx); err != nil {
		t.Fatalf("initialize fresh: %v", err)
	}

	migrated := &Database{Path: filepath.Join("testdata", "parity_migrated.db")}
	if err := migrated.Destroy(); err != nil {
		t.Fatalf("destroy migrated: %v", err)
	}
	if err := migrated.Open(ctx); err != nil {
		t.Fatalf("open migrated: %v", err)
	}
	t.Cleanup(func() { migrated.Close(); migrated.Destroy() })
	// Simulate the oldest supported on-disk version, then run the real migration
	// chain to the latest version — exactly what an upgrading install does.
	prepareLegacySchema(ctx, t, migrated, 2)
	if err := migrated.Migrate(ctx); err != nil {
		t.Fatalf("migrate v2->latest: %v", err)
	}

	freshViews := viewDefinitions(ctx, t, fresh)
	migratedViews := viewDefinitions(ctx, t, migrated)
	if len(freshViews) == 0 {
		t.Fatal("fresh database has no views")
	}

	names := map[string]struct{}{}
	for name := range freshViews {
		names[name] = struct{}{}
	}
	for name := range migratedViews {
		names[name] = struct{}{}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	for _, name := range sorted {
		freshDDL, inFresh := freshViews[name]
		migratedDDL, inMigrated := migratedViews[name]
		switch {
		case !inFresh:
			t.Errorf("view %q exists only in the MIGRATED database; add it to schema.sql", name)
		case !inMigrated:
			t.Errorf("view %q exists only in the FRESH database; add it to a migration", name)
		case freshDDL != migratedDDL:
			t.Errorf("view %q DRIFTED between schema.sql and the migration chain:\n  fresh    = %s\n  migrated = %s", name, freshDDL, migratedDDL)
		}
	}
}
