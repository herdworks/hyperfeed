# HyperFeed Subscriber

The `herdworks/hyperfeed-go/subscriber` package provides subscription lifecycle management.

## Goals

* Subscription list management
* Feed normalization and identification
* Feed aliasing, deduplication, etc
* Feed statistics (request counts, item counts, error rates, modification tracking, etc)
* Feed hybernation (automated disabling & restoration of feed due to error rates, spam, and other conditions)

## Implementation details

