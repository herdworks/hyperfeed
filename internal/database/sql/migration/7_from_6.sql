-- Migration from version 6 to version 7.
-- Add website identity to feed discovery rows so consumers can scope discovered
-- feeds by canonical website_id instead of host-name heuristics.

DROP VIEW IF EXISTS discovery_view;

ALTER TABLE [discovery]
ADD COLUMN [website_id] TEXT;

CREATE INDEX IF NOT EXISTS discovery_website_id_index ON discovery([website_id]);

CREATE VIEW discovery_view AS
SELECT
    json_object(
        'id', [discovery].[id],
        'website_id', [discovery].[website_id],
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
    [discovery].[website_id],
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
SET version = 7, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 6;
