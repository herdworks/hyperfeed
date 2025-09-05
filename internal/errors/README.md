# Internal Errors

The `internal/errors` package provides shared error kinds and wrappers used across
HyperFeed internals (e.g. transporter, scheduler, parsers). Keeping this package
internal discourages external dependency on specific error implementations while
allowing consistent handling within the project.

Guidelines

- Use error kinds for control flow (e.g., NotModified, RateLimited, Temporary).
- Wrap underlying errors to preserve context.
- Prefer returning sentinel kinds over string matching on messages.

