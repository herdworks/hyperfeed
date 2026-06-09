-- Migration from version 3 to version 4.
-- Rebuild discovery at the canonical table shape and expose occurrence/snooze
-- columns directly on discovery_view for efficient filtering.

DROP VIEW IF EXISTS discovery_view;

ALTER TABLE [discovery] RENAME TO [discovery_v3];

CREATE TABLE [discovery] (
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

INSERT OR IGNORE INTO [discovery] (
    [id],
    [created_at],
    [updated_at],
    [origin],
    [url],
    [html_url],
    [favicon_url],
    [cover_url],
    [mentions_url],
    [following_url],
    [email],
    [name],
    [title],
    [description],
    [username],
    [language],
    [occurrences],
    [snooze]
)
SELECT
    COALESCE(NULLIF([id], ''), lower(hex(randomblob(4)))),
    COALESCE(NULLIF([created_at], ''), CURRENT_TIMESTAMP),
    COALESCE(NULLIF([updated_at], ''), CURRENT_TIMESTAMP),
    [origin],
    [url],
    [html_url],
    [favicon_url],
    [cover_url],
    [mentions_url],
    [following_url],
    [email],
    [name],
    [title],
    [description],
    [username],
    [language],
    COALESCE([occurrences], 1),
    COALESCE([snooze], 0)
FROM
    [discovery_v3]
;

DROP TABLE [discovery_v3];

CREATE VIEW discovery_view AS
SELECT
    json_object(
        'id', [discovery].[id],
        'origin', [discovery].[origin],
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
    [discovery].[occurrences],
    [discovery].[snooze],
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

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = 4, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 3;
