-- HyperFeed Database Configuration
PRAGMA journal_mode = WAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;

-- HyperFeed table (versioning info)
CREATE TABLE IF NOT EXISTS hyperfeed (
    [lib_version] TEXT,
    [schema_version] INTEGER,
    [created_at] TIMESTAMP,
    [added_at] TIMESTAMP,

    PRIMARY KEY (lib_version)
);

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

    PRIMARY KEY (id)
);

-- Feeds table
CREATE TABLE IF NOT EXISTS feeds (
    [id] TEXT,
    [url] TEXT UNIQUE,
    [html_url] TEXT,
    [updated_at] TIMESTAMP,
    [title] TEXT,
    [description] TEXT,
    [language] TEXT,

    [status] TEXT NOT NULL DEFAULT '{}', -- Feed.Status JSON string

    [subscriptionlist_id] TEXT REFERENCES subscriptionlists(id),

    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS feeds_id_index ON feeds(id);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    [id] TEXT,
    [url] TEXT,
    [published_at] TIMESTAMP,
    [modified_at] TIMESTAMP,
    [added_at] TIMESTAMP,
    [updated_at] TIMESTAMP,
    [feed_at] TIMESTAMP,
    [author] TEXT,
    [title] TEXT,
    [summary] TEXT,
    [content] TEXT,
    [image_url] TEXT,

    [feed_id] TEXT REFERENCES feeds(id),

    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS items_id_index ON items(id);

-- Authors table
CREATE TABLE IF NOT EXISTS authors (
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [href] TEXT,
    [username] TEXT,
    [name] TEXT,
    [favicon] TEXT,
    [email] TEXT,

    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS authors_id_index ON authors(id);

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    [id] TEXT NOT NULL DEFAULT( lower(hex(randomblob(4))) ),
    [url] TEXT,
    [size] INTEGER,
    [duration] INTEGER,
    [mimetype] TEXT,
    [title] TEXT,
    [labels] TEXT,

    [item_id] TEXT REFERENCES items(id),

    PRIMARY KEY (url,item_id) -- Composite PK
);

-- Metadata table
CREATE TABLE IF NOT EXISTS metadata (
    [id] INTEGER,
    [namespace] TEXT,
    [name] TEXT,
    [value] TEXT,

    PRIMARY KEY (id)
);

-- SubscriptionList-Metadata join table
CREATE TABLE IF NOT EXISTS subscriptionlist_metadata (
    [subscriptionlist_id] TEXT REFERENCES subscriptionlists(id),
    [metadata_id] INTEGER REFERENCES metadata(id),

    PRIMARY KEY(subscriptionlist_id, metadata_id)
);

-- Feed-Metadata join table
CREATE TABLE IF NOT EXISTS feed_metadata (
    [feed_id] TEXT REFERENCES feeds(id),
    [metadata_id] INTEGER REFERENCES metadata(id),

    PRIMARY KEY(feed_id, metadata_id)
);

-- Feed-Author join table
CREATE TABLE IF NOT EXISTS feed_authors (
    [feed_id] TEXT REFERENCES feeds(id),
    [author_id] TEXT REFERENCES authors(id),

    PRIMARY KEY(feed_id, author_id)
);
CREATE INDEX IF NOT EXISTS feed_authors_feed_id_index ON feed_authors(feed_id);
CREATE INDEX IF NOT EXISTS feed_authors_author_id_index ON feed_authors(author_id);

-- Item-Metadata join table
CREATE TABLE IF NOT EXISTS item_metadata (
    [item_id] TEXT REFERENCES items(id),
    [metadata_id] INTEGER REFERENCES metadata(id),

    PRIMARY KEY(item_id, metadata_id)
);

-- Item-Author join table
CREATE TABLE IF NOT EXISTS item_authors (
    [item_id] TEXT REFERENCES items(id),
    [author_id] TEXT REFERENCES authors(id),

    PRIMARY KEY(item_id, author_id)
);
CREATE INDEX IF NOT EXISTS item_authors_item_id_index ON item_authors(item_id);
CREATE INDEX IF NOT EXISTS item_authors_author_id_index ON item_authors(author_id);

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
    subscriptionlists
;

-- FeedsView
CREATE VIEW IF NOT EXISTS feeds_view AS
SELECT
    json_object(
        'id', [feeds].[id],
        'url', [feeds].[url],
        'html_url', [feeds].[html_url],
        'updated_at', [feeds].[updated_at],
        'title', [feeds].[title],
        'description', [feeds].[description],
        'authors', COALESCE(json_group_array(json_object(
            'href', [authors].[href],
            'name', [authors].[name],
            'username', [authors].[username],
            'favicon', [authors].[favicon]
        )) FILTER (WHERE [authors].[id] IS NOT NULL), json('[]')),
        'language', [feeds].[language],
        'status', json([feeds].[status]),
        'subscriptionlist_id', [feeds].[subscriptionlist_id]
    ) AS 'feed',
    [feeds].[id],
    [feeds].[url],
    [feeds].[updated_at]
FROM
    [feeds]
    LEFT OUTER JOIN [feed_authors] ON [feed_authors].[feed_id] = [feeds].[id]
    LEFT OUTER JOIN [authors] ON [authors].[id] = [feed_authors].[author_id]
GROUP BY
    [feeds].[id]
;

-- ItemsView
CREATE VIEW IF NOT EXISTS items_view AS
SELECT
    json_object(
        'id', [items].[id],
        'url', [items].[url],
        'published_at', [items].[published_at],
        'modified_at', [items].[modified_at],
        'added_at', [items].[added_at],
        'updated_at', [items].[updated_at],
        'feed_at', [items].[feed_at],
        'authors', COALESCE(json_group_array(json_object(
            'href', [authors].[href],
            'name', [authors].[name],
            'username', [authors].[username],
            'favicon', [authors].[favicon],
            'email', [authors].[email]
        )) FILTER (WHERE [authors].[id] IS NOT NULL), json('[]')),
        'title', [items].[title],
        'summary', [items].[summary],
        'content', [items].[content],
        'image_url', [items].[image_url],
        'feed_id', [items].[feed_id]
    ) as 'item',
    [items].[id],
    [items].[url],
    [items].[published_at],
    [items].[modified_at],
    [items].[added_at],
    [items].[updated_at],
    [items].[feed_at]
FROM
    [items]
    JOIN [item_authors] ON [item_authors].[item_id] = [items].[id]
    JOIN [authors] ON [authors].[id] = [item_authors].[author_id]
GROUP BY
    [items].[id]
;
-- NOTE: use views for the different item "algorithms" (sort orders)?

-- AuthorsView
CREATE VIEW IF NOT EXISTS authors_view AS
SELECT
    json_object(
        'href', [authors].[href],
        'username', [authors].[username],
        'name', [authors].[name],
        'favicon', [authors].[favicon],
        'email', [authors].[email]
    ) as 'author',
    [authors].[id],
    [authors].[href],
    [authors].[name],
    [authors].[username]
FROM
    [authors]
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

