-- HyperFeed Database Configuration
PRAGMA journal_mode = WAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

-- HyperFeed table (versioning info)
CREATE TABLE IF NOT EXISTS hyperfeed (
    [lib_version] TEXT UNIQUE,
    [lib_schema_version] INTEGER,
    [lib_release_date] TIMESTAMP,
    [added_at] TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY ([lib_version])
);

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
    [website_id] TEXT REFERENCES websites([id]),

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS feeds_id_index ON feeds([id]);
CREATE INDEX IF NOT EXISTS feeds_kind_index ON feeds([kind]);
CREATE INDEX IF NOT EXISTS feeds_url_index ON feeds([url]);
CREATE INDEX IF NOT EXISTS feeds_html_url_index ON feeds([html_url]);
CREATE INDEX IF NOT EXISTS feeds_updated_at_index ON feeds([updated_at] DESC);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    [website_id] TEXT REFERENCES websites([id]),
    [feed_id] TEXT REFERENCES feeds([id]) ON DELETE CASCADE,
    [id] TEXT,
    [published_at] TIMESTAMP,
    [modified_at] TIMESTAMP,
    [added_at] TIMESTAMP,
    [updated_at] TIMESTAMP,
    [feed_at] TIMESTAMP,
    [url] TEXT,
    [external_hrefs] TEXT NOT NULL DEFAULT '[]', -- JSON array of external URL strings
    [title] TEXT,
    [summary] TEXT,
    [content] TEXT,

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS items_id_index ON items([id]);
CREATE INDEX IF NOT EXISTS items_url_index ON items([url]);
CREATE INDEX IF NOT EXISTS items_external_hrefs_index ON items([external_hrefs]);
CREATE INDEX IF NOT EXISTS items_published_at_index ON items([published_at] DESC);
CREATE INDEX IF NOT EXISTS items_modified_at_index ON items([modified_at] DESC);
CREATE INDEX IF NOT EXISTS items_added_at_index ON items([added_at] DESC);
CREATE INDEX IF NOT EXISTS items_updated_at_index ON items([updated_at] DESC);
CREATE INDEX IF NOT EXISTS items_feed_at_index_asc ON items([feed_at] ASC);
CREATE INDEX IF NOT EXISTS items_feed_at_index_desc ON items([feed_at] DESC);
CREATE INDEX IF NOT EXISTS items_view_feed_at_index_asc ON items(feed_at DESC, published_at DESC, added_at DESC);

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

-- Images table
CREATE TABLE IF NOT EXISTS images (
    [id] TEXT UNIQUE DEFAULT( lower(hex(randomblob(4))) ),
    [href] TEXT NOT NULL,
    [sources] TEXT NOT NULL DEFAULT '[]', -- JSON array of ImageSource objects
    [title] TEXT,
    [alt] TEXT,
    [caption] TEXT,
    [width] INTEGER,
    [height] INTEGER,

    [item_id] TEXT NOT NULL REFERENCES items([id]),
    [feed_id] TEXT NOT NULL REFERENCES feeds([id]),
    [website_id] TEXT NOT NULL REFERENCES websites([id]),

    PRIMARY KEY ([id])
);
CREATE INDEX IF NOT EXISTS images_href_index ON images([href]);
CREATE INDEX IF NOT EXISTS images_item_id_index ON images([item_id]);
CREATE INDEX IF NOT EXISTS images_feed_id_index ON images([feed_id]);
CREATE INDEX IF NOT EXISTS images_website_id_index ON images([website_id]);

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    [id] TEXT NOT NULL DEFAULT( lower(hex(randomblob(4))) ),
    [url] TEXT,
    [size] INTEGER,
    [duration] INTEGER,
    [mimetype] TEXT,
    [title] TEXT,
    [labels] TEXT,

    [item_id] TEXT NOT NULL REFERENCES items([id]),
    [feed_id] TEXT NOT NULL REFERENCES feeds([id]),
    [website_id] TEXT NOT NULL REFERENCES websites([id]),

    PRIMARY KEY ([url],[item_id]) -- Composite PK
);
CREATE INDEX IF NOT EXISTS attachments_id_index ON attachments([id]);
CREATE INDEX IF NOT EXISTS attachments_url_index ON attachments([url]);
CREATE INDEX IF NOT EXISTS attachments_item_id_index ON attachments([item_id]);
CREATE INDEX IF NOT EXISTS attachments_feed_id_index ON attachments([feed_id]);
CREATE INDEX IF NOT EXISTS attachments_website_id_index ON attachments([website_id]);

-- Cache table
CREATE TABLE IF NOT EXISTS cache (
    [url] TEXT PRIMARY KEY, -- source url
    [uri] TEXT, -- cache file uri
    [type] TEXT, -- mimetype
    [size_in_bytes] INTEGER, -- size of cached data
    [updated_at] TIMESTAMP DEFAULT CURRENT_TIMESTAMP, -- last update time
    [fetched_at] TIMESTAMP, -- last fetch time
    [http_etag] TEXT, -- HTTP ETag
    [http_last_modified] TEXT, -- HTTP Last-Modified
    [http_expires_at] TIMESTAMP, -- HTTP Expires
    [http_status_code] INTEGER, -- HTTP status code of last fetch
    [bytes] BLOB -- cached data
);
CREATE INDEX IF NOT EXISTS cache_url_index ON cache ([url]);
CREATE INDEX IF NOT EXISTS cache_type_index ON cache ([type]);
CREATE INDEX IF NOT EXISTS cache_http_expires_at_index ON cache ([http_expires_at]);

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

-- Author-Relation join table
CREATE TABLE IF NOT EXISTS item_bylines (
    [feed_id] TEXT REFERENCES feeds([id]),
    [item_id] TEXT REFERENCES items([id]),
    [byline_id] TEXT NOT NULL REFERENCES bylines([id]),

    PRIMARY KEY([item_id], [byline_id])
);
CREATE INDEX IF NOT EXISTS item_bylines_feed_id_index ON item_bylines([feed_id]);
CREATE INDEX IF NOT EXISTS item_bylines_item_id_index ON item_bylines([item_id]);
CREATE INDEX IF NOT EXISTS item_bylines_byline_id_index ON item_bylines([byline_id]);

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
        'html_url', [feeds].[html_url],
        'title', [feeds].[title],
        'description', [feeds].[description],
        'language', [feeds].[language],
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
            json('{}')
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
            SELECT
                json([feeds_view].[feed])
            FROM [feeds_view]
            WHERE [feeds_view].[id] = [items].[feed_id]
        ), json('{}')),
        'title', [items].[title],
        'bylines', COALESCE((
            SELECT json_group_array(json([bylines_view].[byline]))
            FROM [item_bylines]
            INNER JOIN [bylines_view] ON [bylines_view].[id] = [item_bylines].[byline_id]
            WHERE [item_bylines].[item_id] = [items].[id]
        ), json('[]')),
        'summary', [items].[summary],
        'content', [items].[content],
        'images', COALESCE((
            SELECT json_group_array(json([images_view].[image]))
            FROM [images_view]
            WHERE [images_view].[item_id] = [items].[id]
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
    [items].[published_at] DESC,
    [items].[added_at] DESC
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
            'feed', json('{}'),
            'title', [items].[title],
            'bylines', COALESCE((
                SELECT json_group_array(json([bylines_view].[byline]))
                FROM [item_bylines]
                INNER JOIN [bylines_view] ON [bylines_view].[id] = [item_bylines].[byline_id]
                WHERE [item_bylines].[item_id] = [items].[id]
            ), json('[]')),
            'summary', [items].[summary],
            'content', [items].[content],
            'images', COALESCE((
                SELECT json_group_array(json([images_view].[image]))
                FROM [images_view]
                WHERE [images_view].[item_id] = [items].[id]
            ), json('[]'))
        ),
        'feed', COALESCE((
            SELECT
                json([feeds_view].[feed])
            FROM [feeds_view]
            WHERE [feeds_view].[id] = [items].[feed_id]
        ), json('{}'))
    ) AS [post],
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
    [items].[published_at] DESC,
    [items].[added_at] DESC
;

-- BylinesView
CREATE VIEW IF NOT EXISTS bylines_view AS
SELECT
    json_object(
        'id', [bylines].[id],
        'href', NULLIF([bylines].[href], ''),
        'name', [bylines].[name],
        'email', [bylines].[email]
    ) as 'byline',
    [bylines].[id],
    [bylines].[href],
    [bylines].[name],
    [bylines].[email]
FROM
    [bylines]
;

-- ImagesView
CREATE VIEW IF NOT EXISTS images_view AS
SELECT
    json_object(
        'id', [images].[id],
        'href', [images].[href],
        'sources', json([images].[sources]),
        'title', [images].[title],
        'alt', [images].[alt],
        'caption', [images].[caption],
        'width', [images].[width],
        'height', [images].[height],
        'item_id', [images].[item_id],
        'feed_id', [images].[feed_id]
    ) as 'image',
    [images].[id],
    [images].[href],
    [images].[item_id],
    [images].[feed_id]
FROM
    [images]
;

-- AttachmentsView
CREATE VIEW IF NOT EXISTS attachments_view AS
SELECT
    json_object(
        'id', [attachments].[id],
        'url', [attachments].[url],
        'size', [attachments].[size],
        'duration', [attachments].[duration],
        'mimetype', [attachments].[mimetype],
        'title', [attachments].[title],
        'labels', [attachments].[labels],
        'item_id', [attachments].[item_id]
    ) as 'attachment',
    [attachments].[id],
    [attachments].[url],
    [attachments].[item_id]
FROM
    [attachments]
;
