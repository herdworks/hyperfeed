# Atom Parser

The `parsers/atom` subpackage provides a parser implementation for Atom
(0.3/1.0). It maps Atom documents into `types.Feed` and `types.Item`.

Scope

- Detect Atom via lightweight sniffing (XML root `<feed>` with Atom namespace).
- Parse core fields, tolerate common defects, record non-fatal warnings.
- No network I/O; callers provide raw bytes and options (base URL, clock).

