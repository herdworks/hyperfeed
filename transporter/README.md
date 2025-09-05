# HyperFeed Transporter

The `herdworks/hyperfeed-go/transporter` package provides conditional HTTP request management.

## Goals

* No external dependencies (only `net/http`, `x/net/html`, and related standard libraries)
* Uses `If-None-Match` and `If-Modified-Since` headers for conditional request
