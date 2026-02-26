-- Migration from version 0 to version 1
BEGIN;

    -- Bump version
    -- NOTE: this will only succeed if the version is currently 0, which should be impossible
    UPDATE [hyperfeed_schema] SET [version] = 1, [updated_at] = CURRENT_TIMESTAMP WHERE version = 0;

COMMIT;
