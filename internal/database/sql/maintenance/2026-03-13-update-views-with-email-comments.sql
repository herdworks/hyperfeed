-- [feeds] table
-- Abort maintenance if [feeds].[email] does not exist.
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE NOT EXISTS (
    SELECT * FROM pragma_table_info('feeds') WHERE [name] = 'email'
);

-- [items] table
-- Abort maintenance if [items].[comments_url] does not exist.
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE NOT EXISTS (
    SELECT * FROM pragma_table_info('items') WHERE [name] = 'comments_url'
);

-- [feeds_view] view
DROP VIEW feeds_view;
CREATE VIEW IF NOT EXISTS feeds_view AS
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

-- [items_view] view
DROP VIEW items_view;
CREATE VIEW IF NOT EXISTS items_view AS
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
DROP VIEW posts_view;
CREATE VIEW IF NOT EXISTS posts_view AS
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

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-13-update-views-with-email-comments.sql', 'update [feeds_view], [items_view], and [posts_view] with [feeds].[email] and [items].[comments_url] columns.')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
