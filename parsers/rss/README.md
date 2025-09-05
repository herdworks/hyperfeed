# RSS Parser

The `parsers/rss` subpackage provides a parser implementation for RSS
(0.90–2.0). It maps RSS documents into `types.Feed` and `types.Item`.

Scope

- Detect RSS via lightweight sniffing (XML root `<rss>`).
- Parse core fields, tolerate common defects, record non-fatal warnings.
- No network I/O; callers provide raw bytes and options (base URL, clock).

