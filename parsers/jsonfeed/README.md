# JSON Feed Parser

The `parsers/jsonfeed` subpackage provides a parser implementation for
the JSON Feed format (https://www.jsonfeed.org/version/1).

Scope

- Detect JSON Feed documents via content sniffing and/or `Content-Type: application/feed+json`.
- Parse into `types.Feed` and `types.Items`, applying normalization rules where appropriate.
- Be tolerant of minor schema deviations; record non-fatal warnings.

