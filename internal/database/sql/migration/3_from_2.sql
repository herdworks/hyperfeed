-- Migration from version 2 to version 3.
-- Inaugural migration after the maintenance-script cutover: re-creates all SQL
-- views from the canonical definitions in schema.sql. Idempotent by design --
-- fresh databases initialize at version 2 and immediately migrate to version 3,
-- continuously exercising the migration pipeline.

DROP VIEW IF EXISTS activities_view;
DROP VIEW IF EXISTS discovery_view;
DROP VIEW IF EXISTS attachments_view;
DROP VIEW IF EXISTS bylines_view;
DROP VIEW IF EXISTS responses_view;
DROP VIEW IF EXISTS hot_hyperlinks_view;
DROP VIEW IF EXISTS links_view;
DROP VIEW IF EXISTS posts_view;
DROP VIEW IF EXISTS items_view;
DROP VIEW IF EXISTS feeds_view;
DROP VIEW IF EXISTS websites_view;
DROP VIEW IF EXISTS subscriptionlists_view;

CREATE VIEW subscriptionlists_view AS
SELECT
    json_object(
        'id', [id],
        'uri', [uri],
        'title', [title],
        'owner_name', [owner_name],
        'owner_email', [owner_email],
        'created_at', [created_at],
        'updated_at', [updated_at]
    ) AS 'subscriptionlist',
    [id],
    [uri],
    [created_at],
    [updated_at]
FROM
    [subscriptionlists]
;

-- WebsitesView
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
        'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END
    ) AS 'website',
    [websites].[id],
    [websites].[html_url],
    [websites].[username],
    [websites].[email]
FROM
    [websites]
;

-- FeedsView
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
                    'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END
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
    [feeds].[website_id]
FROM
    [feeds]
;

-- ItemsView
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

-- PostsView
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
                'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END
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

-- LinksView
CREATE VIEW links_view AS
SELECT
    json_object(
        'website_id', [links].[website_id],
        'id', [links].[id],
        'href', [links].[href],
        'cover_href', NULLIF([links].[cover_href], ''),
        'title', [links].[title],
        'description', [links].[description]
    ) as 'link',
    [links].[id],
    [links].[href],
    [links].[website_id]
FROM
    [links]
;

-- HotHyperLinksView
CREATE VIEW hot_hyperlinks_view AS
SELECT
    json_object(
        'website_id', [links].[website_id],
        'id', [links].[id],
        'href', [links].[href],
        'cover_href', NULLIF([links].[cover_href], ''),
        'title', [links].[title],
        'description', [links].[description],
        'website_count', [link_counts].[website_count],
        'item_count', [link_counts].[item_count],
        'score', [link_counts].[score]
    ) as 'link',
    [links].[website_id],
    [links].[id],
    [links].[href],
    [link_counts].[website_count],
    [link_counts].[item_count],
    [link_counts].[score]
FROM
    [links]
    JOIN (
        SELECT
            [item_links].[link_id],
            COUNT(DISTINCT [item_links].[website_id]) AS [website_count],
            COUNT(*) AS [item_count],
            SUM(COALESCE([item_links].[weight], 0)) AS [score],
            MAX([items].[feed_at]) AS [last_feed_at]
        FROM
            [item_links]
            JOIN [items] ON [items].[id] = [item_links].[item_id]
        WHERE
            [items].[feed_at] >= datetime('now', '-48 hours')
        GROUP BY
            [item_links].[link_id]
        HAVING
            COUNT(DISTINCT [item_links].[website_id]) > 1 AND
            SUM(COALESCE([item_links].[weight], 0)) > 0.0
    ) AS [link_counts] ON [link_counts].[link_id] = [links].[id]
ORDER BY
    [link_counts].[website_count] DESC,
    [link_counts].[score] DESC,
    [link_counts].[item_count] DESC,
    [link_counts].[last_feed_at] DESC
LIMIT 100
;

-- ResponsesView
CREATE VIEW responses_view AS
SELECT
    json([posts_view].[post]) AS 'post',
    [item_links].[link_id] AS 'link_id',
    [posts_view].[id] AS 'item_id',
    [posts_view].[feed_at] AS 'feed_at'
FROM
    posts_view JOIN item_links ON posts_view.id = item_links.item_id
;

-- BylinesView
CREATE VIEW bylines_view AS
SELECT
    json_object(
        'id', [bylines].[id],
        'href', NULLIF([bylines].[href], ''),
        'name', [bylines].[name],
        'email', [bylines].[email],
        'item_id', [item_bylines].[item_id],
        'feed_id', [item_bylines].[feed_id],
        'website_id', [item_bylines].[website_id]
    ) as 'byline',
    [bylines].[id],
    [bylines].[href],
    [bylines].[name],
    [bylines].[email],
    [item_bylines].[item_id],
    [item_bylines].[feed_id],
    [item_bylines].[website_id]
FROM
    [bylines]
    INNER JOIN [item_bylines] ON [item_bylines].[byline_id] = [bylines].[id]
;

-- AttachmentsView
CREATE VIEW attachments_view AS
SELECT
    json_object(
        'id', [attachments].[id],
        'href', [attachments].[href],
        'sources', json([attachments].[sources]),
        'poster', NULLIF([attachments].[poster], ''),
        'mimetype', [attachments].[mimetype],
        'title', [attachments].[title],
        'description', [attachments].[description],
        'enclosure', CASE WHEN [attachments].[enclosure] = 1 THEN json('true') ELSE json('false') END,
        'download', CASE WHEN [attachments].[download] = 1 THEN json('true') ELSE json('false') END,
        'rank', [attachments].[rank],
        'size', [attachments].[size],
        'duration', [attachments].[duration],
        'playback_position', [attachments].[playback_position],
        'width', [attachments].[width],
        'height', [attachments].[height],
        'item_id', [item_attachments].[item_id],
        'feed_id', [item_attachments].[feed_id],
        'website_id', [item_attachments].[website_id]
    ) as 'attachment',
    [attachments].[id],
    [attachments].[href],
    [attachments].[mimetype],
    [attachments].[width],
    [attachments].[height],
    [attachments].[display],
    [attachments].[download],
    [item_attachments].[item_id],
    [item_attachments].[feed_id],
    [item_attachments].[website_id],
    [item_attachments].[enclosure],
    [item_attachments].[priority],
    [item_attachments].[rank]
FROM
    [attachments]
    INNER JOIN [item_attachments] ON [item_attachments].[attachment_id] = [attachments].[id]
;

CREATE VIEW discovery_view AS
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

CREATE VIEW activities_view AS
SELECT
    json_object(
        'id', [activities].[id],
        'created_at', [activities].[created_at],
        'updated_at', [activities].[updated_at],
        'kind', [activities].[kind],
        'uri', [activities].[uri],
        'seen', [activities].[seen],
        'data', json([activities].[data])
    ) AS 'activity',
    [activities].[id],
    [activities].[created_at],
    [activities].[updated_at],
    [activities].[kind],
    [activities].[uri],
    [activities].[seen]
FROM
    [activities]
ORDER BY
    [activities].[created_at] DESC
;

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = 3, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 2;
