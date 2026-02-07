-- Replace attachments_view with a new definition that includes the display column.
DROP VIEW IF EXISTS attachments_view;
CREATE VIEW IF NOT EXISTS attachments_view AS
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
    [item_attachments].[item_id],
    [item_attachments].[feed_id],
    [item_attachments].[website_id]
FROM
    [attachments]
    INNER JOIN [item_attachments] ON [item_attachments].[attachment_id] = [attachments].[id]
;

-- Replace posts_view with a new definition that includes the display column from attachments_view.
DROP VIEW IF EXISTS posts_view;
CREATE VIEW IF NOT EXISTS posts_view AS
SELECT
    json_object(
        'item', json_object(
            'id', [items].[id],
            'url', [items].[url],
            'external_hrefs', json([items].[external_hrefs]),
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
                SELECT json_group_array(json([attachments_view].[attachment]))
                FROM [attachments_view] 
                WHERE 
                    [attachments_view].[item_id] = [items].[id] AND
                    [attachments_view].[display] = 1
            ), json('[]'))
        ),
        'feed', COALESCE((
            SELECT
                json_object(
                    'id', [feeds].[id],
                    'kind', [feeds].[kind],
                    'url', [feeds].[url],
                    'html_url', NULLIF([feeds].[html_url], ''),
                    'title', [feeds].[title],
                    'description', [feeds].[description],
                    'language', [feeds].[language],
                    'is_podcast', CASE WHEN [feeds].[is_podcast] = 1 THEN json('true') ELSE json('false') END,
                    'website', json(NULL),
                    'status', json([feeds].[status]),
                    'subscriptionlist_id', [feeds].[subscriptionlist_id]
                )
            FROM [feeds]
            WHERE [feeds].[id] = [items].[feed_id]
        ), json('{}')),
        'website', COALESCE((
            SELECT
                json_object(
                    'id', [websites].[id],
                    'html_url', NULLIF([websites].[html_url], ''),
                    'favicon_url', NULLIF([websites].[favicon_url], ''),
                    'cover_url', NULLIF([websites].[cover_url], ''),
                    'mentions_url', NULLIF([websites].[mentions_url], ''),
                    'email', [websites].[email],
                    'name', [websites].[name],
                    'username', [websites].[username],
                    'description', [websites].[description],
                    'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END
                )
            FROM [websites]
            WHERE [websites].[id] = [items].[website_id]
            LIMIT 1
        ), json('{}'))
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
ORDER BY
    [items].[feed_at] DESC,
    [items].[id] DESC
;

-- Record maintenance completion
INSERT OR IGNORE INTO hyperfeed_scripts (id, description, completed_at)
VALUES ('sql/maintenance/attachments_display_views.sql', 'update views to use attachments.display', CURRENT_TIMESTAMP);
