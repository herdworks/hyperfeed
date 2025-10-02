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

    [status] TEXT, -- Feeder.Status JSON string

    [subscriptionlist_id] TEXT REFERENCES subscriptionlists(id),

    PRIMARY KEY (id)
);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    [id] TEXT,
    [guid] TEXT,
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
    [name] TEXT,
    [url] TEXT,
    [favicon] TEXT,
    [email] TEXT,

    PRIMARY KEY (id)
);

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    [id] TEXT DEFAULT( lower(hex(randomblob(4))) ),
    [title] TEXT,
    [url] TEXT,
    [size] INTEGER,
    [duration] INTEGER,
    [mimetype] TEXT,
    [labels] TEXT,

    [item_id] TEXT REFERENCES items(id),

    PRIMARY KEY (id)
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

-- Item-Metadata join table
CREATE TABLE IF NOT EXISTS item_metadata (
    [item_id] TEXT REFERENCES items(id),
    [metadata_id] INTEGER REFERENCES metadata(id),
    
    PRIMARY KEY(item_id, metadata_id)
);
