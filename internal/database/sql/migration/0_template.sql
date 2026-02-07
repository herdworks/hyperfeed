-- Migration template (copy and rename to <to_version>_<short_description>.sql).
-- Target version: <to_version>.
-- From version: <from_version>.
-- Notes:
-- * Do not include BEGIN, COMMIT, or ROLLBACK statements.
-- * The runner wraps migrations in a BEGIN IMMEDIATE transaction.
-- * Keep all statements idempotent or guarded for safe re-runs.
-- * Update hyperfeed_schema from <from_version> to <to_version> as the final statement.

-- Schema changes (idempotent).
-- Example:
-- CREATE TABLE IF NOT EXISTS example (
--     id TEXT PRIMARY KEY,
--     created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
-- );

-- Data backfill (guarded).
-- Example:
-- UPDATE example
-- SET created_at = CURRENT_TIMESTAMP
-- WHERE created_at IS NULL;

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = <to_version>, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = <from_version>;
