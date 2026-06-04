-- Maintenance script: drop and recreate the discovery table and view at the
-- canonical schema.sql shape. Heals stale v2 databases created before the
-- discovery table reached its current iteration. Recorded in hyperfeed_scripts,
-- so databases that already ran it are skipped.
-- This script will be wrapped in a transaction; no need to wrap in BEGIN/COMMIT.

DROP VIEW IF EXISTS discovery_view;
DROP TABLE IF EXISTS discovery;

CREATE TABLE IF NOT EXISTS discovery (
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [created_at] TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    [updated_at] TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    [origin] TEXT,
    [url] TEXT,
    [html_url] TEXT,
    [favicon_url] TEXT,
    [cover_url] TEXT,
    [mentions_url] TEXT,
    [following_url] TEXT,
    [email] TEXT,
    [name] TEXT,
    [title] TEXT,
    [description] TEXT,
    [username] TEXT,
    [language] TEXT,
    [occurrences] INTEGER DEFAULT 1,
    [snooze] INTEGER DEFAULT 0,

    PRIMARY KEY ([id])
);

CREATE VIEW IF NOT EXISTS discovery_view AS
SELECT
    json_object(
        'id', [discovery].[id],
        'url', [discovery].[url],
        'html_url', [discovery].[html_url],
        'favicon_url', [discovery].[favicon_url],
        'cover_url', [discovery].[cover_url],
        'mentions_url', [discovery].[mentions_url],
        'following_url', [discovery].[following_url],
        'email', [discovery].[email],
        'name', [discovery].[name],
        'title', [discovery].[title],
        'description', [discovery].[description],
        'username', [discovery].[username],
        'language', [discovery].[language],
        'occurrences', [discovery].[occurrences],
        'snooze', [discovery].[snooze],
        'created_at', [discovery].[created_at],
        'updated_at', [discovery].[updated_at]
    ) AS 'discovery',
    [discovery].[id],
    [discovery].[url],
    [discovery].[html_url],
    [discovery].[username],
    [discovery].[created_at],
    [discovery].[updated_at]
FROM
    [discovery]
LEFT OUTER JOIN [feeds] ON [discovery].[id] = [feeds].[id]
WHERE
    [feeds].[id] IS NULL
ORDER BY
    [discovery].[updated_at] DESC,
    [discovery].[occurrences] DESC
;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/discovery-5.sql', 'drop and recreate the discovery table and view with the canonical schema')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
