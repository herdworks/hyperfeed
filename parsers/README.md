# HyperFeed Parsers

The `github.com/herdworks/hyperfeed-go/parsers` package defines public interfaces and types
for parsing RSS and Atom documents into HyperFeed domain types. It does not perform any
network I/O; callers provide the raw document bytes.

Goals

- Clear interface for RSS/Atom parsers
- Tolerant of common feed defects; report non-fatal warnings
- No external dependencies beyond the standard library and `types`
- Easy to extend with additional formats if needed

Notes

- Concrete implementations (e.g., RSS, Atom) can live as subpackages later
  (e.g., `parsers/rss`, `parsers/atom`) or as internal implementations registered
  behind the public interfaces.
- Parsing should return normalized `types.Feed` and `types.Items`.

