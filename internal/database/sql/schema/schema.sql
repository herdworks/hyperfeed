-- HyperFeed Database Configuration
PRAGMA journal_mode = WAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

-- HyperFeed Schema version table (single-row)
CREATE TABLE IF NOT EXISTS hyperfeed_schema (
    [id] INTEGER PRIMARY KEY CHECK ([id] = 1),
    [version] INTEGER NOT NULL,
    [updated_at] TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- HyperFeed Schema triggers (enforce single-row, monotonic version increments)
CREATE TRIGGER IF NOT EXISTS hyperfeed_schema_insert_trigger
BEFORE INSERT ON hyperfeed_schema
FOR EACH ROW
BEGIN
    SELECT CASE
        WHEN NEW.version > 1 AND (SELECT COUNT(*) FROM hyperfeed_schema) = 0
            THEN RAISE(ABORT, 'hyperfeed_schema must start at version 1')
    END;
END;
CREATE TRIGGER IF NOT EXISTS hyperfeed_schema_update_trigger
BEFORE UPDATE ON hyperfeed_schema
FOR EACH ROW
BEGIN
    SELECT CASE
        WHEN NEW.version != OLD.version + 1
            THEN RAISE(ABORT, 'hyperfeed_schema must increment by 1')
    END;
END;

-- Initialize hyperfeed_schema with version 1 if not already set
INSERT OR IGNORE INTO hyperfeed_schema (id, version) VALUES (1, 1);

-- Scripts table
CREATE TABLE IF NOT EXISTS hyperfeed_scripts (
    [id] TEXT PRIMARY KEY,
    [description] TEXT,
    [completed_at] TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- HyperFeed scripts no-op guard (used by maintenance scripts).
CREATE TRIGGER IF NOT EXISTS hyperfeed_scripts_noop_trigger
BEFORE INSERT ON hyperfeed_scripts
WHEN NEW.id = 'noop'
BEGIN
    SELECT RAISE(ABORT, 'HYPERFEED_NOOP:');
END;

-- Initial version entry
-- INSERT OR IGNORE INTO hyperfeed (lib_version, lib_schema_version, lib_release_date)
-- VALUES ('v0.1.0', 1, datetime('2025-11-03T12:00:00-08:00'));

-- SubscriptionLists table
CREATE TABLE IF NOT EXISTS subscriptionlists (
    [id] TEXT,
    [uri] TEXT,
    [title] TEXT,
    [owner_name] TEXT,
    [owner_email] TEXT,
    [created_at] TIMESTAMP,
    [updated_at] TIMESTAMP,
    [status] TEXT NOT NULL DEFAULT '{}', -- SubscriptionList.Status JSON string

    PRIMARY KEY ([id])
);

-- Websites table
CREATE TABLE IF NOT EXISTS websites (
    [id] TEXT,
    [href] TEXT GENERATED ALWAYS AS ([html_url]) STORED,
    [html_url] TEXT,
    [favicon_url] TEXT,
    [cover_url] TEXT,
    [mentions_url] TEXT,
    [email] TEXT,
    [name] TEXT,
    [username] TEXT,
    [description] TEXT,
    [verified] INTEGER DEFAULT 0 CHECK ([verified] IN (0, 1)),
    [status] TEXT NOT NULL DEFAULT '{}', -- Website.Status JSON string

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS websites_id_index ON websites([id]);
CREATE INDEX IF NOT EXISTS websites_href_index ON websites([href]);
CREATE INDEX IF NOT EXISTS websites_html_url_index ON websites([html_url]);
CREATE INDEX IF NOT EXISTS websites_username_index ON websites([username]);
CREATE INDEX IF NOT EXISTS websites_email_index ON websites([email]);

-- Feeds table
CREATE TABLE IF NOT EXISTS feeds (
    [id] TEXT,
    [kind] TEXT,
    [url] TEXT UNIQUE,
    [html_url] TEXT,
    [title] TEXT,
    [description] TEXT,
    [language] TEXT,
    [is_podcast] INTEGER DEFAULT 0 CHECK ([is_podcast] IN (0, 1)),
    [interval] TEXT,
    [status] TEXT NOT NULL DEFAULT '{}', -- Feed.Status JSON string

    -- NOTE:
    --   [status_code] is an example computed column for indexing [status] JSON subfields.
    --   To create additional indexed JSON projections, use the same pattern:
    --     [<column>] [<type>] GENERATED ALWAYS AS (json_extract(status, '$.<json_path>')) STORED
    --   This allows efficient filtering/sorting on JSON subfields.
    --   Use VIRTUAL for unindexed columns or write-heavy workloads, STORED for indexed columns or read-heavy workloads
    [updated_at] TIMESTAMP GENERATED ALWAYS AS (json_extract(status, '$.updated_at')) STORED,

    [subscriptionlist_id] TEXT REFERENCES subscriptionlists([id]),
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS feeds_id_index ON feeds([id]);
CREATE INDEX IF NOT EXISTS feeds_kind_index ON feeds([kind]);
CREATE INDEX IF NOT EXISTS feeds_url_index ON feeds([url]);
CREATE INDEX IF NOT EXISTS feeds_html_url_index ON feeds([html_url]);
CREATE INDEX IF NOT EXISTS feeds_updated_at_index ON feeds([updated_at] DESC);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [id] TEXT,
    [link_id] TEXT, -- back-reference to primary link for the item
    [published_at] TIMESTAMP,
    [modified_at] TIMESTAMP,
    [added_at] TIMESTAMP,
    [updated_at] TIMESTAMP,
    [feed_at] TIMESTAMP,
    [guid] TEXT,
    [url] TEXT,
    [external_hrefs] TEXT NOT NULL DEFAULT '[]', -- JSON array of external URL strings
    [title] TEXT,
    [summary] TEXT,
    [content] TEXT,
    [is_podcast_episode] INTEGER DEFAULT 0 CHECK ([is_podcast_episode] IN (0, 1)),

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS items_id_index ON items([id]);
CREATE INDEX IF NOT EXISTS items_url_index ON items([url]);
CREATE INDEX IF NOT EXISTS items_external_hrefs_index ON items([external_hrefs]);
CREATE INDEX IF NOT EXISTS items_published_at_index ON items([published_at] DESC);
CREATE INDEX IF NOT EXISTS items_modified_at_index ON items([modified_at] DESC);
CREATE INDEX IF NOT EXISTS items_added_at_index ON items([added_at] DESC);
CREATE INDEX IF NOT EXISTS items_updated_at_index ON items([updated_at] DESC);
CREATE INDEX IF NOT EXISTS items_feed_id_feed_at_id_desc ON items(feed_id, feed_at DESC, id DESC); -- website feed index
CREATE INDEX IF NOT EXISTS items_feed_at_index_asc ON items([feed_at] ASC);
CREATE INDEX IF NOT EXISTS items_feed_at_index_desc ON items([feed_at] DESC);
CREATE INDEX IF NOT EXISTS items_feed_at_id_desc ON items([feed_at] DESC, [id] DESC);
CREATE INDEX IF NOT EXISTS items_view_feed_at_index_asc ON items([feed_at] DESC, [published_at] DESC, [added_at] DESC);
CREATE INDEX IF NOT EXISTS items_website_id_feed_at_desc ON items([website_id], [feed_at] DESC); -- feed frequency index
CREATE INDEX IF NOT EXISTS items_website_id_feed_at_id_desc ON items([website_id], [feed_at] DESC, [id] DESC);

--NOTE:  the following unique index had unintended side effects; next time we see duplicate posts,
--       we should check to see if the duplicates have been orphaned from their parent website.
--CREATE UNIQUE INDEX IF NOT EXISTS items_feed_id_guid_url_unique_index ON items([feed_id], [guid], [url]);

-- Bylines table
CREATE TABLE IF NOT EXISTS bylines (
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [href] TEXT,
    [name] TEXT,
    [email] TEXT,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS bylines_id_index ON bylines([id]);
CREATE INDEX IF NOT EXISTS bylines_href_index ON bylines([href]);
CREATE INDEX IF NOT EXISTS bylines_name_index ON bylines([name]);
CREATE INDEX IF NOT EXISTS bylines_email_index ON bylines([email]);

-- Links table
CREATE TABLE IF NOT EXISTS links (
    [website_id] TEXT,
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [href] TEXT NOT NULL,
    [cover_href] TEXT,
    [title] TEXT,
    [description] TEXT,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS links_id_index ON links([id]);
CREATE INDEX IF NOT EXISTS links_href_index ON links([href]);
CREATE INDEX IF NOT EXISTS links_website_id_index ON links([website_id]);

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    [id] TEXT NOT NULL DEFAULT( lower(hex(randomblob(4))) ),

    -- uris
    [href] TEXT,
    [sources] TEXT NOT NULL DEFAULT '[]', -- JSON array of source objects
    [poster] TEXT,

    -- metadata
    [mimetype] TEXT,
    [title] TEXT,
    [description] TEXT,
    [enclosure] INTEGER DEFAULT 0 CHECK ([enclosure] IN (0, 1)),
    [download] INTEGER DEFAULT 0 CHECK ([download] IN (0, 1)),

    -- details
    [rank] INTEGER,
    [width] INTEGER,
    [height] INTEGER,
    [size] INTEGER,
    [duration] INTEGER,
    [display] INTEGER DEFAULT 0 CHECK ([display] IN (0, 1)),

    -- playback
    [playback_position] INTEGER,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS attachments_id_index ON attachments([id]);
CREATE INDEX IF NOT EXISTS attachments_href_index ON attachments([href]);

-- Metadata table
CREATE TABLE IF NOT EXISTS metadata (
    [id] INTEGER,
    [namespace] TEXT,
    [name] TEXT,
    [value] TEXT,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS metadata_id_index ON metadata([id]);
CREATE INDEX IF NOT EXISTS metadata_namespace_index ON metadata([namespace]);
CREATE INDEX IF NOT EXISTS metadata_name_index ON metadata([name]);
CREATE INDEX IF NOT EXISTS metadata_ref_index ON metadata([namespace], [name]);

-- Discovery table
--
-- Used for storing discovered feeds & websites
CREATE TABLE IF NOT EXISTS discovery (
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [kind] TEXT,
    [url] TEXT,
    [feed_url] TEXT,
    [html_url] TEXT,
    [favicon_url] TEXT,
    [cover_url] TEXT,
    [mentions_url] TEXT,
    [email] TEXT,
    [name] TEXT,
    [title] TEXT,
    [description] TEXT,
    [username] TEXT,
    [language] TEXT,

    PRIMARY KEY ([id])
);

-- Metadata-Relation join table
CREATE TABLE IF NOT EXISTS metadata_relation (
    [subscription_id] TEXT REFERENCES subscriptionlists([id]),
    [feed_id] TEXT REFERENCES feeds([id]),
    [item_id] TEXT REFERENCES items([id]),
    [metadata_id] INTEGER NOT NULL REFERENCES metadata([id]),

    CHECK((subscription_id IS NOT NULL) OR (feed_id IS NOT NULL) OR (item_id IS NOT NULL)),

    UNIQUE (subscription_id, metadata_id),
    UNIQUE (feed_id, metadata_id),
    UNIQUE (item_id, metadata_id),

    PRIMARY KEY([subscription_id], [feed_id], [item_id])
);
CREATE INDEX IF NOT EXISTS metadata_relation_subscription_id_index ON metadata_relation([subscription_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_feed_id_index ON metadata_relation([feed_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_item_id_index ON metadata_relation([item_id]);
CREATE INDEX IF NOT EXISTS metadata_relation_metadata_id_index ON metadata_relation([metadata_id]);

-- Item-Bylines join table
CREATE TABLE IF NOT EXISTS item_bylines (
    [website_id] TEXT REFERENCES websites([id]),
    [feed_id] TEXT REFERENCES feeds([id]),
    [item_id] TEXT REFERENCES items([id]),
    [byline_id] TEXT NOT NULL REFERENCES bylines([id]),

    PRIMARY KEY([item_id], [byline_id])
);
CREATE INDEX IF NOT EXISTS item_bylines_website_id_index ON item_bylines([website_id]);
CREATE INDEX IF NOT EXISTS item_bylines_feed_id_index ON item_bylines([feed_id]);
CREATE INDEX IF NOT EXISTS item_bylines_byline_id_index ON item_bylines([byline_id]);

-- Item-Attachments join table
CREATE TABLE IF NOT EXISTS item_attachments (
    [website_id] TEXT REFERENCES websites([id]),
    [feed_id] TEXT REFERENCES feeds([id]),
    [item_id] TEXT REFERENCES items([id]),
    [attachment_id] TEXT NOT NULL REFERENCES attachments([id]),
    PRIMARY KEY([item_id], [attachment_id])
);
CREATE INDEX IF NOT EXISTS item_attachments_website_id_index ON item_attachments([website_id]);
CREATE INDEX IF NOT EXISTS item_attachments_feed_id_index ON item_attachments([feed_id]);
CREATE INDEX IF NOT EXISTS item_attachments_item_id_index ON item_attachments([item_id]);
CREATE INDEX IF NOT EXISTS item_attachments_attachment_id_index ON item_attachments([attachment_id]);

-- Item-Links join table
CREATE TABLE IF NOT EXISTS item_links (
    [website_id] TEXT REFERENCES websites([id]) ON DELETE CASCADE,
    [feed_id] TEXT REFERENCES feeds([id]),
    [item_id] TEXT REFERENCES items([id]),
    [link_id] TEXT REFERENCES links([id]),
    [weight] FLOAT,

    PRIMARY KEY([item_id], [link_id])
);
CREATE INDEX IF NOT EXISTS item_links_link_id_item_id_index ON item_links(link_id, item_id);

-- SubscriptionListsView
CREATE VIEW IF NOT EXISTS subscriptionlists_view AS
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
CREATE VIEW IF NOT EXISTS websites_view AS
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
    ) AS 'website',
    [websites].[id],
    [websites].[html_url],
    [websites].[username],
    [websites].[email]
FROM
    [websites]
;

-- FeedsView
CREATE VIEW IF NOT EXISTS feeds_view AS
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
        'status', json([feeds].[status]),
        'subscriptionlist_id', [feeds].[subscriptionlist_id]
    ) AS 'feed',
    [feeds].[id],
    [feeds].[url],
    [feeds].[updated_at],
    [feeds].[subscriptionlist_id],
    [feeds].[website_id]
FROM
    [feeds]
;

-- ItemsView
CREATE VIEW IF NOT EXISTS items_view AS
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

-- PostsView
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

-- LinksView
CREATE VIEW IF NOT EXISTS links_view AS
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
CREATE VIEW IF NOT EXISTS hot_hyperlinks_view AS
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

-- ResponsesView
CREATE VIEW IF NOT EXISTS responses_view AS
SELECT
    json([posts_view].[post]) AS 'post',
    [item_links].[link_id] AS 'link_id',
    [posts_view].[id] AS 'item_id',
    [posts_view].[feed_at] AS 'feed_at'
FROM
    posts_view JOIN item_links ON posts_view.id = item_links.item_id
;

-- BylinesView
CREATE VIEW IF NOT EXISTS bylines_view AS
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
