# HyperFeed Types

Core domain and wire types for HyperFeed. This package defines a normalized model used by higher‑level packages (discovery, subscriber, scheduler, database) and format‑specific shims for RSS/Atom/JSON Feed parsing.

`generator` serializes caller-supplied normalized values without importing CMS,
filesystem, or application types. Its initial Atom implementation accepts a
`Feed` graph and returns XML bytes; format-specific caller conversion remains
outside HyperFeed.

* `SubscriptionList`
  * `Owner`
  * `Metadata`
* `Feed`
  * `Author`
  * `Metadata`
* `Item`
  * `RSS`
  * `Atom`
  * `JSONFeed`
  * `Metadata`
* `Attachment`

## Publishing data additions

The domain model can carry publishing data in memory and JSON. This does not
change subscriber parsing, Markdown conversion, database storage, or ranking.

| Type | Fields | Meaning |
| --- | --- | --- |
| `Feed` | `Bylines`, `Links`, `ModifiedAt` | Optional feed attribution, links, and source modification time (`modified_at`). |
| `Item` | `GUID` | Source/publishing identifier serialized as `guid`, separate from database `ID`. |
| `Byline` | `Role`, `Order` | Contextual attribution role and one-based position; empty role and zero order are unspecified. |
| `Link` (formerly `HyperLink`) | `Rel`, `Mimetype`, `HrefLang`, `Length`, `Origin` | Optional syndication attributes; `href_lang` is snake case, `length` is bytes, and nil length differs from zero. Origin can describe content or metadata links. |

New fields are omitted from JSON when unspecified. Role and order describe an
attribution occurrence, not the shared person's identity. They are retained
exactly as supplied; no ordering or role inference is performed. Existing
parser assignments, including GUID assignment, are unchanged; parsers leave
the newly added feed, byline, and link fields unspecified. Database reloads
likewise leave those fields unspecified because existing tables and views are
unchanged.

`Item.Content` and `Item.Summary` are existing strings. Subscriber workflows
continue producing Markdown; publishing callers may supply HTML. Conversion
between these uses belongs to the caller. `Feed.URL` is the planned Atom feed
identifier; database identity methods remain unchanged.


## `SubscriptionList`

Represents a user’s subscriptions as sourced from an OPML file.

- URL: `URI *url.URL` (location of the OPML)
- Document: `OPML *opml.OPML` (parsed via `github.com/herdworks/hypertemplates-go/opml`)
- Identity: `Title string`, `Owner Owner`, `CreatedAt time.Time`, `UpdatedAt time.Time`
- Extensions: `Metadata Metadata`
- Contents: `Feeds []*Feed`
- Behavior: `Load()` populates fields from OPML head values and timestamps.

Source: `github.com/herdworks/hyperfeed/subscription.go`

## `Feed`

Represents a single RSS/Atom/JSON feed endpoint.

- Identity: `URL *url.URL`, `Domain string`, `ID string`, `Kind FeedKind`
- Presentation: `Title string`, `Description string`, `Language string`, `Updated time.Time`, `Authors []Author`
- Extensions: `Metadata Metadata`
- Contents: `Items []*Item`
- Behavior:
  - `Identify()` populates `Domain` and `ID` using normalization helpers
  - `CanonicalID() *url.URL` returns a stable host/path

Source: `github.com/herdworks/hyperfeed/feed.go`

### `FeedKind`

Enumerates known feed type/version pairs.

- RSS: `rss/0.91`, `rss/0.92`, `rss/2.0`
- Atom: `atom/0.3`, `atom/1.0`
- JSON Feed: `jsonfeed/1.0`, `jsonfeed/1.1`

## `Item`

A single entry in a feed.

- Identity: `ID string`, `GUID string`, `URL *url.URL`
- Text: `Title string`, `Summary string`, `Content string`
- People: `Authors []Author`
- Timestamps: `Published time.Time`, `Updated time.Time`
- Media: `Attachments []Attachment`, `Image *Image`
- Taxonomy: `Categories []string`
- Extensions: `Metadata Metadata`
- Behavior: `CanonicalID()` returns ID → GUID → URL fallback

Source: `github.com/herdworks/hyperfeed/item.go`

## `Attachment`

Lowest common denominator for media across RSS enclosures, Atom `link rel="enclosure"`, and JSON Feed attachments.

- Addressing: `URL *url.URL`
- Identification: `Title string`, `Mimetype string`
- Size/Time: `Size int64` (bytes), `Duration int64` (seconds)
- Labels: `Labels []string` (generic tags)

Cross‑format mapping guidance:

- URL/href → `URL`
- length/size_in_bytes → `Size`
- type/mime_type → `Mimetype`
- duration_in_seconds (JSON Feed; podcast extensions) → `Duration`
- Atom `link@title` or extensions → `Title`
- Non‑standard labels/tags → `Labels`

Source: `github.com/herdworks/hyperfeed/attachment.go`

## Supporting Types

### `Metadata`

Generic bag for non‑standard properties on SubscriptionList, Feed, and Item.

- Type: `type Metadata map[string]Property`
- Property: `{ Namespace, Name, Value, RawValue }`
- XML capture: `Extension` preserves unknown namespaced XML (name, attrs, text, inner XML) for RSS/Atom parsing.

Source: `github.com/herdworks/hyperfeed/metadata.go`

### `RSS`

Wire model for RSS 0.9x/2.0 parsing with extension preservation.

- Root: `RSS` { `Version`, `Channel` }
- Channel: standard RSS elements (`title`, `link`, `description`, etc.) plus `Items []RSSItem`
- Item: core RSS fields plus `Enclosures []RSSEnclosure` and `Extensions []Extension`
- Enclosure: `{ URL, Length, Type }` attributes
- Helpers: `RSSDetect([]byte) bool`, `ParseRSS([]byte) (*RSS, error)` with `charset.NewReaderLabel` for non‑UTF8 encodings

Source: `github.com/herdworks/hyperfeed/rss/rss.go`

### `Atom`

Atom 1.0 parsing will leverage the shared templates project for strict schema fidelity.

- Parser/types: `github.com/herdworks/hypertemplates-go/atom`
- Usage: parse with the external package, then map to core `Feed`, `Item`, and `Attachment` in this package
  (e.g., Atom `link rel="enclosure"` → `Attachment`, `entry` → `Item`).
- Extensions: unknown namespaced elements should be preserved via `Metadata` where appropriate.

Note: This package intentionally does not duplicate a full Atom schema; it consumes the external package and focuses on normalization.

Source: `github.com/herdworks/hypertemplates-go/types/atom.go`

### `JSONFeed`

Detection and parsing for JSON Feed 1.x.

- Detect: `JSONFeedDetect([]byte) bool`
- Parse: `JSONFeedParse([]byte, JSONFeedOptions) (*JSONFeedResult, error)` (mapping to core types)
- Options: `BaseURL`, `Now func() time.Time`, `AllowExtensions`
- Mapping highlights: `attachments[]` → `Attachment` (`url`, `mime_type`, `size_in_bytes`, `duration_in_seconds`)

Source: `github.com/herdworks/hyperfeed/jsonfeed/jsonfeed.go`

## Serialization

- Core types use JSON field tags for wire representation.
- URL fields use `*url.URL` and are expected to be absolute and normalized at parse time.
- Timestamps use `time.Time` in UTC; parsers should handle common date defects and timezone normalization.

## IDs and Canonicalization

- SubscriptionList/Feed/Item provide stable identifiers suitable for database keys and deduplication.
- `Feed.Identify()` and `Feed.CanonicalID()` encapsulate domain + hex ID generation.
- `Item.CanonicalID()` falls back in order: `ID` → `GUID` → `URL`.

## Next Steps

- Implement mappers:
  - RSS → core `Feed/Item/Attachment`
  - Atom (via `hypertemplates-go/atom`) → core types
  - JSON Feed → core types
- Preserve unknown extensions via `Metadata` while keeping the normalized model minimal.
