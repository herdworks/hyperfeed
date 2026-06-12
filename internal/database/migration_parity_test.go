package database

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var viewWhitespace = regexp.MustCompile(`\s+`)

// canonicalDDL normalizes a stored CREATE VIEW/TRIGGER statement for
// comparison. It drops the `IF NOT EXISTS` that schema.sql carries (the
// migration scripts may omit it) and collapses whitespace. Nothing else is
// normalized — including identifier quoting — so schema.sql and the migration
// chain must stay in lockstep down to the bracketing.
func canonicalDDL(sql string) string {
	normalized := strings.ReplaceAll(sql, "IF NOT EXISTS ", "")
	normalized = viewWhitespace.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

// masterDefinitions returns name -> canonicalized DDL for sqlite_master rows
// of the given type (sqlite-internal objects like autoindexes are excluded).
func masterDefinitions(ctx context.Context, t *testing.T, db *Database, objectType string) map[string]string {
	t.Helper()
	rows, err := db.DB.QueryContext(ctx, `
		SELECT name, COALESCE(sql, '') FROM sqlite_master
		WHERE type = ? AND name NOT LIKE 'sqlite_%'
		ORDER BY name`, objectType)
	if err != nil {
		t.Fatalf("query %s definitions: %v", objectType, err)
	}
	defer rows.Close()
	defs := map[string]string{}
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatalf("scan %s definition: %v", objectType, err)
		}
		defs[name] = canonicalDDL(sql)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s definitions: %v", objectType, err)
	}
	return defs
}

func viewDefinitions(ctx context.Context, t *testing.T, db *Database) map[string]string {
	t.Helper()
	return masterDefinitions(ctx, t, db, "view")
}

func triggerDefinitions(ctx context.Context, t *testing.T, db *Database) map[string]string {
	t.Helper()
	return masterDefinitions(ctx, t, db, "trigger")
}

func indexDefinitions(ctx context.Context, t *testing.T, db *Database) map[string]string {
	t.Helper()
	return masterDefinitions(ctx, t, db, "index")
}

// tableColumnSignatures returns name -> sorted column signature for every
// non-internal table. Tables are compared by column SET, not DDL text or
// column order: an upgraded database acquires columns via ALTER TABLE ADD
// COLUMN, which appends them to the end and preserves the historical CREATE
// TABLE text, so DDL and ordering legitimately differ from schema.sql (e.g.
// websites.following, added by 5_from_4.sql). What must NOT drift is the set
// of columns and their definitions.
func tableColumnSignatures(ctx context.Context, t *testing.T, db *Database) map[string]string {
	t.Helper()
	names := masterDefinitions(ctx, t, db, "table")
	sigs := map[string]string{}
	for name := range names {
		rows, err := db.DB.QueryContext(ctx,
			`SELECT name, type, "notnull", COALESCE(dflt_value, ''), pk FROM pragma_table_info(?)`, name)
		if err != nil {
			t.Fatalf("pragma_table_info(%q): %v", name, err)
		}
		var columns []string
		for rows.Next() {
			var columnName, columnType, defaultValue string
			var notNull, pk int
			if err := rows.Scan(&columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
				rows.Close()
				t.Fatalf("scan table_info(%q): %v", name, err)
			}
			columns = append(columns, fmt.Sprintf("%s %s notnull=%d default=%q pk=%d", columnName, columnType, notNull, defaultValue, pk))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("iterate table_info(%q): %v", name, err)
		}
		rows.Close()
		sort.Strings(columns)
		sigs[name] = strings.Join(columns, "; ")
	}
	return sigs
}

func compareParity(t *testing.T, kind string, fresh, migrated map[string]string) {
	t.Helper()
	names := map[string]struct{}{}
	for name := range fresh {
		names[name] = struct{}{}
	}
	for name := range migrated {
		names[name] = struct{}{}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	for _, name := range sorted {
		freshDef, inFresh := fresh[name]
		migratedDef, inMigrated := migrated[name]
		switch {
		case !inFresh:
			t.Errorf("%s %q exists only in the MIGRATED database; add it to schema.sql", kind, name)
		case !inMigrated:
			t.Errorf("%s %q exists only in the FRESH database; add it to a migration", kind, name)
		case freshDef != migratedDef:
			t.Errorf("%s %q DRIFTED between schema.sql and the migration chain:\n  fresh    = %s\n  migrated = %s", kind, name, freshDef, migratedDef)
		}
	}
}

// TestFreshVsMigratedSchemaParity guards a hard invariant of the migration
// design: schema.sql (applied ONLY to genuinely fresh databases, stamped at
// the latest version) and the migration chain (applied to existing databases,
// never touching schema.sql) must produce the SAME schema. A fresh install
// runs schema.sql and stops; an upgrading install reaches the latest version
// purely through the N_from_(N-1).sql scripts. If the two sources drift,
// fresh and existing users get different behavior from the same release (e.g.
// the historical comments_url incident). Views, triggers, and indexes are
// compared by canonicalized DDL; tables by column signature (see
// tableColumnSignatures for why DDL text cannot be compared for tables). An
// object present in only one source is also caught.
//
// Note: the simulated legacy database starts from the current schema.sql
// (stripped back), so hyperfeed_schema_insert_trigger — whose version literal
// is frozen at creation time and never recreated by migrations — matches
// trivially here. On real upgraded devices its literal stays at the historical
// version, which is harmless: it only fires on the first insert into an empty
// hyperfeed_schema table.
func TestFreshVsMigratedSchemaParity(t *testing.T) {
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
	if len(freshViews) == 0 {
		t.Fatal("fresh database has no views")
	}
	compareParity(t, "view", freshViews, viewDefinitions(ctx, t, migrated))
	compareParity(t, "trigger", triggerDefinitions(ctx, t, fresh), triggerDefinitions(ctx, t, migrated))
	compareParity(t, "index", indexDefinitions(ctx, t, fresh), indexDefinitions(ctx, t, migrated))
	compareParity(t, "table", tableColumnSignatures(ctx, t, fresh), tableColumnSignatures(ctx, t, migrated))
}
