-- Migration from version 4 to version 5.
-- Introduce soft-delete: a `following` flag on websites and feeds, plus a
-- `sponsored` flag on items (for generated sponsored-post feeds).
--
-- Add the v5 columns. This migration runs only against real v4-shaped
-- databases; fresh databases are created directly at the latest schema version.

ALTER TABLE [websites]
ADD COLUMN [following] INTEGER NOT NULL DEFAULT 1 CHECK ([following] IN (0, 1));

ALTER TABLE [feeds]
ADD COLUMN [following] INTEGER NOT NULL DEFAULT 1 CHECK ([following] IN (0, 1));

ALTER TABLE [items]
ADD COLUMN [sponsored] INTEGER NOT NULL DEFAULT 0 CHECK ([sponsored] IN (0, 1));

-- Backfill: a website is "followed" iff it has at least one followed feed.
-- (Existing feeds default to following=1, so existing websites with feeds become
-- followed; orphan websites with no feeds become unfollowed.)
UPDATE [websites]
SET [following] = (
    CASE WHEN EXISTS (
        SELECT 1 FROM [feeds]
        WHERE [feeds].[website_id] = [websites].[id] AND [feeds].[following] = 1
    ) THEN 1 ELSE 0 END
);

-- Indexes for following / sponsored filtering.
CREATE INDEX IF NOT EXISTS websites_following_index ON websites([following]);
CREATE INDEX IF NOT EXISTS feeds_following_index ON feeds([following]);
CREATE INDEX IF NOT EXISTS feeds_website_id_following_index ON feeds([website_id], [following]);
CREATE INDEX IF NOT EXISTS items_sponsored_index ON items([sponsored]);

-- Rebuild the views that surface `following` / `sponsored`. Dropped in reverse
-- dependency order; recreated in dependency order (items_view reads feeds_view).
DROP VIEW IF EXISTS posts_view;
DROP VIEW IF EXISTS items_view;
DROP VIEW IF EXISTS feeds_view;
DROP VIEW IF EXISTS websites_view;

CREATE VIEW websites_view AS
SELECT
    json_object(
        'id', [websites].[id],
        'html_url', NULLIF([websites].[html_url], ''),
        'favicon_url', NULLIF([websites].[favicon_url], ''),
        'cover_url', NULLIF([websites].[cover_url], ''),
        'mentions_url', NULLIF([websites].[mentions_url], ''),
        'following_url', NULLIF([websites].[following_url], ''),
        'email', [websites].[email],
        'name', [websites].[name],
        'username', [websites].[username],
        'description', [websites].[description],
        'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END,
        'following', CASE WHEN [websites].[following] = 1 THEN json('true') ELSE json('false') END
    ) AS 'website',
    [websites].[id],
    [websites].[html_url],
    [websites].[username],
    [websites].[email],
    [websites].[following]
FROM
    [websites]
;

CREATE VIEW feeds_view AS
SELECT
    json_object(
        'id', [feeds].[id],
        'kind', [feeds].[kind],
        'url', [feeds].[url],
        'html_url', NULLIF([feeds].[html_url], ''),
        'email', [feeds].[email],
        'title', [feeds].[title],
        'description', [feeds].[description],
        'language', [feeds].[language],
        'is_podcast', CASE WHEN [feeds].[is_podcast] = 1 THEN json('true') ELSE json('false') END,
        'following', CASE WHEN [feeds].[following] = 1 THEN json('true') ELSE json('false') END,
        'website', COALESCE(
            (
                SELECT json_object(
                    'id', [websites].[id],
                    'html_url', NULLIF([websites].[html_url], ''),
                    'favicon_url', NULLIF([websites].[favicon_url], ''),
                    'cover_url', NULLIF([websites].[cover_url], ''),
                    'mentions_url', NULLIF([websites].[mentions_url], ''),
                    'following_url', NULLIF([websites].[following_url], ''),
                    'email', [websites].[email],
                    'name', [websites].[name],
                    'username', [websites].[username],
                    'description', [websites].[description],
                    'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END,
                    'following', CASE WHEN [websites].[following] = 1 THEN json('true') ELSE json('false') END
                )
                FROM [websites]
                WHERE [websites].[id] = [feeds].[website_id]
                LIMIT 1
            ),
            json(NULL)
        ),
        'status', json([feeds].[status])
    ) AS 'feed',
    [feeds].[id],
    [feeds].[url],
    [feeds].[updated_at],
    [feeds].[website_id],
    [feeds].[following]
FROM
    [feeds]
;

CREATE VIEW items_view AS
SELECT
    json_object(
        'id', [items].[id],
        'url', [items].[url],
        'comments_url', NULLIF([items].[comments_url], ''),
        'published_at', [items].[published_at],
        'modified_at', [items].[modified_at],
        'added_at', [items].[added_at],
        'updated_at', [items].[updated_at],
        'feed_at', [items].[feed_at],
        'feed', COALESCE((
            SELECT json([feeds_view].[feed])
            FROM [feeds_view]
            WHERE [feeds_view].[id] = [items].[feed_id]
        ), json('{}')),
        'title', [items].[title],
        'bylines', COALESCE((
            SELECT json_group_array(json([bylines_view].[byline]))
            FROM [bylines_view]
            WHERE [bylines_view].[item_id] = [items].[id]
        ), json('[]')),
        'summary', [items].[summary],
        'content', [items].[content],
        'is_podcast_episode', CASE WHEN [items].[is_podcast_episode] = 1 THEN json('true') ELSE json('false') END,
        'sponsored', CASE WHEN [items].[sponsored] = 1 THEN json('true') ELSE json('false') END,
        'link_id', [items].[link_id],
        'attachments', COALESCE((
            SELECT json_group_array(json([attachments_view].[attachment]))
            FROM [attachments_view]
            WHERE [attachments_view].[item_id] = [items].[id]
        ), json('[]'))
    ) AS [item],
    [items].[id],
    [items].[url],
    [items].[published_at],
    [items].[modified_at],
    [items].[added_at],
    [items].[updated_at],
    [items].[feed_at]
FROM items
ORDER BY
    [items].[feed_at] DESC,
    [items].[id] DESC
;

CREATE VIEW posts_view AS
SELECT
    json_object(
        'item', json_object(
            'id', [items].[id],
            'url', [items].[url],
            'comments_url', NULLIF([items].[comments_url], ''),
            'published_at', [items].[published_at],
            'modified_at', [items].[modified_at],
            'added_at', [items].[added_at],
            'updated_at', [items].[updated_at],
            'feed_at', [items].[feed_at],
            'feed', json(NULL),
            'title', [items].[title],
            'bylines', COALESCE((
                SELECT json_group_array(json([bylines_view].[byline]))
                FROM [bylines_view]
                WHERE [bylines_view].[item_id] = [items].[id]
            ), json('[]')),
            'summary', [items].[summary],
            'content', [items].[content],
            'is_podcast_episode', CASE WHEN [items].[is_podcast_episode] = 1 THEN json('true') ELSE json('false') END,
            'sponsored', CASE WHEN [items].[sponsored] = 1 THEN json('true') ELSE json('false') END,
            'link_id', [items].[link_id],
            'attachments', COALESCE((
                SELECT json_group_array(json([an].[attachment]))
                FROM (
                    SELECT [attachments_view].[attachment]
                    FROM [attachments_view]
                    WHERE
                        [attachments_view].[item_id] = [items].[id] AND
                        [attachments_view].[display] = 1
                    ORDER BY
                        [attachments_view].[enclosure] DESC,
                        [attachments_view].[priority] ASC,
                        [attachments_view].[rank] ASC
                ) AS [an]
            ), json('[]'))
        ),
        'feed', CASE
            WHEN [feeds].[id] IS NULL THEN json('{}')
            ELSE json_object(
                'id', [feeds].[id],
                'kind', [feeds].[kind],
                'url', [feeds].[url],
                'html_url', NULLIF([feeds].[html_url], ''),
                'email', [feeds].[email],
                'title', [feeds].[title],
                'description', [feeds].[description],
                'language', [feeds].[language],
                'is_podcast', CASE WHEN [feeds].[is_podcast] = 1 THEN json('true') ELSE json('false') END,
                'following', CASE WHEN [feeds].[following] = 1 THEN json('true') ELSE json('false') END,
                'website', json(NULL),
                'status', json([feeds].[status])
            )
        END,
        'website', CASE
            WHEN [websites].[id] IS NULL THEN json('{}')
            ELSE json_object(
                'id', [websites].[id],
                'html_url', NULLIF([websites].[html_url], ''),
                'favicon_url', NULLIF([websites].[favicon_url], ''),
                'cover_url', NULLIF([websites].[cover_url], ''),
                'mentions_url', NULLIF([websites].[mentions_url], ''),
                'following_url', NULLIF([websites].[following_url], ''),
                'email', [websites].[email],
                'name', [websites].[name],
                'username', [websites].[username],
                'description', [websites].[description],
                'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END,
                'following', CASE WHEN [websites].[following] = 1 THEN json('true') ELSE json('false') END
            )
        END
    ) AS [post],
    [items].[id],
    [items].[url],
    [items].[published_at],
    [items].[modified_at],
    [items].[added_at],
    [items].[updated_at],
    [items].[feed_at],
    [items].[feed_id],
    [items].[website_id]
FROM items
LEFT JOIN [feeds] ON [feeds].[id] = [items].[feed_id]
LEFT JOIN [websites] ON [websites].[id] = [items].[website_id]
ORDER BY
    [items].[feed_at] DESC,
    [items].[id] DESC
;

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = 5, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 4;
