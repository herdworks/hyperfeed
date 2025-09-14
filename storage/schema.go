package storage

const schema = `
CREATE TABLE IF NOT EXISTS subscription_lists (
    id TEXT PRIMARY KEY,
    uri TEXT,
    title TEXT,
    owner_name TEXT,
    owner_email TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS feeds (
    id TEXT PRIMARY KEY,
    subscription_id TEXT REFERENCES subscription_lists(id),
    url TEXT UNIQUE,
    domain TEXT,
    title TEXT,
    description TEXT,
    language TEXT,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    feed_id TEXT REFERENCES feeds(id),
    guid TEXT,
    url TEXT,
    title TEXT,
    author TEXT,
    published TIMESTAMP,
    updated TIMESTAMP,
    summary TEXT,
    content TEXT,
    image_url TEXT
);

CREATE TABLE IF NOT EXISTS metadata (
    id INTEGER PRIMARY KEY,
    namespace TEXT,
    name TEXT,
    value TEXT
);

CREATE TABLE IF NOT EXISTS feed_metadata (
    feed_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(feed_id, metadata_id),
    FOREIGN KEY(feed_id) REFERENCES feeds(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);

CREATE TABLE IF NOT EXISTS item_metadata (
    item_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(item_id, metadata_id),
    FOREIGN KEY(item_id) REFERENCES items(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);

CREATE TABLE IF NOT EXISTS subscription_metadata (
    subscription_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(subscription_id, metadata_id),
    FOREIGN KEY(subscription_id) REFERENCES subscription_lists(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);
`
