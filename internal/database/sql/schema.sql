-- Configuration
PRAGMA journal_mode = WAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;

-- SubscriptionLists table
CREATE TABLE IF NOT EXISTS subscriptionlists (
    [id] TEXT,
    [uri] TEXT,
    [title] TEXT,
    [owner_name] TEXT,
    [owner_email] TEXT,
    [created_at] TIMESTAMP,
    [updated_at] TIMESTAMP,

    PRIMARY KEY (id)
);

-- Feeds table
CREATE TABLE IF NOT EXISTS feeds (
    [id] TEXT,
    [url] TEXT UNIQUE,
    [updated_at] TIMESTAMP,
    [title] TEXT,
    [description] TEXT,
    [language] TEXT,

    [status] TEXT, -- Feed.Status JSON string

    [subscriptionlist_id] TEXT REFERENCES subscriptionlists(id),

    PRIMARY KEY (id)
);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    [id] TEXT,
    [url] TEXT,
    [published] TIMESTAMP,
    [updated] TIMESTAMP,
    [added] TIMESTAMP,
    [author] TEXT,
    [title] TEXT,
    [summary] TEXT,
    [content] TEXT,
    [image_url] TEXT,

    [feed_id] TEXT REFERENCES feeds(id),

    PRIMARY KEY (id)
);

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

    -- indexes
    

    PRIMARY KEY(item_id, author_id)
);

