# Internal Normalization

The `internal/normalization` package centralizes URL and string normalization
so all components (parsers, discovery, scheduler, storage) produce consistent
canonical identifiers and links.

Guidelines

- Normalize once, use everywhere; avoid duplicate ad-hoc rules.
- Keep behavior deterministic and side-effect free.
- Prefer RFC-compliant normalization over destructive rewrites.

