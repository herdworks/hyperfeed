-- SubscriptionLists table (OMPL data)
CREATE TABLE IF NOT EXISTS subscriptionlists (
    id TEXT PRIMARY KEY,
    uri TEXT,
    title TEXT,
    owner_name TEXT,
    owner_email TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Feeds table
--
-- status is a json object
CREATE TABLE IF NOT EXISTS feeds (
    id TEXT PRIMARY KEY,
    subscriptionlist_id TEXT REFERENCES subscriptionlists(id),
    url TEXT UNIQUE,
    domain TEXT,
    title TEXT,
    description TEXT,
    language TEXT,
    updated_at TIMESTAMP,
    status TEXT
);

-- Items table
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

-- Metadata table
CREATE TABLE IF NOT EXISTS metadata (
    id INTEGER PRIMARY KEY,
    namespace TEXT,
    name TEXT,
    value TEXT
);

-- SubscriptionList-Metadata join table
CREATE TABLE IF NOT EXISTS subscriptionlist_metadata (
    subscriptionlist_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(subscriptionlist_id, metadata_id),
    FOREIGN KEY(subscriptionlist_id) REFERENCES subscriptionlists(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);

-- Feed-Metadata join table
CREATE TABLE IF NOT EXISTS feed_metadata (
    feed_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(feed_id, metadata_id),
    FOREIGN KEY(feed_id) REFERENCES feeds(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);

-- Item-Metadata join table
CREATE TABLE IF NOT EXISTS item_metadata (
    item_id TEXT,
    metadata_id INTEGER,
    PRIMARY KEY(item_id, metadata_id),
    FOREIGN KEY(item_id) REFERENCES items(id),
    FOREIGN KEY(metadata_id) REFERENCES metadata(id)
);
