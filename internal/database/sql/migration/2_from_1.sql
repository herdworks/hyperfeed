-- Migration from version 1 to version 2.
-- Drops deprecated subscription list fields, rebuilds relation tables with cascade rules,
-- and refreshes dependent views.

DROP VIEW IF EXISTS responses_view;
DROP VIEW IF EXISTS hot_hyperlinks_view;
DROP VIEW IF EXISTS attachments_view;
DROP VIEW IF EXISTS bylines_view;
DROP VIEW IF EXISTS posts_view;
DROP VIEW IF EXISTS items_view;
DROP VIEW IF EXISTS feeds_view;

-- Drop legacy subscriptionlist_id column from feeds.
ALTER TABLE feeds DROP COLUMN subscriptionlist_id;

-- Rebuild metadata_relation with website scope + cascades.
ALTER TABLE metadata_relation RENAME TO metadata_relation_old;
CREATE TABLE IF NOT EXISTS metadata_relation (
    [website_id] TEXT REFERENCES websites([id]),
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [item_id] TEXT REFERENCES items([id]) ON DELETE CASCADE,
    [metadata_id] INTEGER NOT NULL REFERENCES metadata([id]) ON DELETE CASCADE,

    CHECK((website_id IS NOT NULL) OR (feed_id IS NOT NULL) OR (item_id IS NOT NULL)),

    UNIQUE (website_id, metadata_id),
    UNIQUE (feed_id, metadata_id),
    UNIQUE (item_id, metadata_id),

    PRIMARY KEY([website_id], [feed_id], [item_id])
);
INSERT OR IGNORE INTO metadata_relation (website_id, feed_id, item_id, metadata_id)
SELECT
    COALESCE(
        (SELECT website_id FROM feeds WHERE feeds.id = metadata_relation_old.feed_id),
        (SELECT feeds.website_id FROM items JOIN feeds ON items.feed_id = feeds.id WHERE items.id = metadata_relation_old.item_id)
    ),
    metadata_relation_old.feed_id,
    metadata_relation_old.item_id,
    metadata_relation_old.metadata_id
FROM metadata_relation_old
WHERE metadata_relation_old.feed_id IS NOT NULL OR metadata_relation_old.item_id IS NOT NULL;
DROP TABLE metadata_relation_old;
CREATE INDEX IF NOT EXISTS metadata_relation_website_id_index ON metadata_relation([website_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_feed_id_index ON metadata_relation([feed_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_item_id_index ON metadata_relation([item_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_metadata_id_index ON metadata_relation([metadata_id]);

-- Rebuild item_bylines with cascades.
ALTER TABLE item_bylines RENAME TO item_bylines_old;
CREATE TABLE IF NOT EXISTS item_bylines (
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [item_id] TEXT REFERENCES items([id]) ON DELETE CASCADE,
    [byline_id] TEXT NOT NULL REFERENCES bylines([id]) ON DELETE CASCADE,

    PRIMARY KEY([item_id], [byline_id])
);
INSERT OR IGNORE INTO item_bylines (website_id, feed_id, item_id, byline_id)
SELECT website_id, feed_id, item_id, byline_id
FROM item_bylines_old;
DROP TABLE item_bylines_old;
CREATE INDEX IF NOT EXISTS item_bylines_website_id_index ON item_bylines([website_id]);
CREATE INDEX IF NOT EXISTS item_bylines_feed_id_index ON item_bylines([feed_id]);
CREATE INDEX IF NOT EXISTS item_bylines_byline_id_index ON item_bylines([byline_id]);

-- Rebuild item_attachments with cascades.
ALTER TABLE item_attachments RENAME TO item_attachments_old;
CREATE TABLE IF NOT EXISTS item_attachments (
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [item_id] TEXT REFERENCES items([id]) ON DELETE CASCADE,
    [attachment_id] TEXT NOT NULL REFERENCES attachments([id]) ON DELETE CASCADE,
    PRIMARY KEY([item_id], [attachment_id])
);
INSERT OR IGNORE INTO item_attachments (website_id, feed_id, item_id, attachment_id)
SELECT website_id, feed_id, item_id, attachment_id
FROM item_attachments_old;
DROP TABLE item_attachments_old;
CREATE INDEX IF NOT EXISTS item_attachments_website_id_index ON item_attachments([website_id]);
CREATE INDEX IF NOT EXISTS item_attachments_feed_id_index ON item_attachments([feed_id]);
CREATE INDEX IF NOT EXISTS item_attachments_item_id_index ON item_attachments([item_id]);
CREATE INDEX IF NOT EXISTS item_attachments_attachment_id_index ON item_attachments([attachment_id]);

-- Rebuild item_links with cascades and add missing indexes.
ALTER TABLE item_links RENAME TO item_links_old;
CREATE TABLE IF NOT EXISTS item_links (
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [item_id] TEXT REFERENCES items([id]) ON DELETE CASCADE,
    [link_id] TEXT REFERENCES links([id]) ON DELETE CASCADE,
    [weight] FLOAT,

    PRIMARY KEY([item_id], [link_id])
);
INSERT OR IGNORE INTO item_links (website_id, feed_id, item_id, link_id, weight)
SELECT website_id, feed_id, item_id, link_id, weight
FROM item_links_old;
DROP TABLE item_links_old;
CREATE INDEX IF NOT EXISTS item_links_link_id_item_id_index ON item_links(link_id, item_id);
CREATE INDEX IF NOT EXISTS item_links_website_id_index ON item_links([website_id]);
CREATE INDEX IF NOT EXISTS item_links_feed_id_index ON item_links([feed_id]);

-- Refresh feeds/items/posts views to match the updated schema.
CREATE VIEW feeds_view AS
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
        'website', COALESCE(
            (
                SELECT json_object(
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

CREATE VIEW items_view AS
SELECT
    json_object(
        'id', [items].[id],
        'url', [items].[url],
        'external_hrefs', json([items].[external_hrefs]),
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

CREATE VIEW posts_view AS
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
                    'status', json([feeds].[status])
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
    [items].[feed_at]
FROM [items]
;

CREATE VIEW responses_view AS
SELECT
    json([posts_view].[post]) AS 'post',
    [item_links].[link_id] AS 'link_id',
    [posts_view].[id] AS 'item_id',
    [posts_view].[feed_at] AS 'feed_at'
FROM
    posts_view JOIN item_links ON posts_view.id = item_links.item_id
;

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
    [item_attachments].[item_id],
    [item_attachments].[feed_id],
    [item_attachments].[website_id]
FROM
    [attachments]
    INNER JOIN [item_attachments] ON [item_attachments].[attachment_id] = [attachments].[id]
;

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
            [items].[feed_at] >= datetime('now', '-7 days')
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

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = 2, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 1;
