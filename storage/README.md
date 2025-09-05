# HyperFeed Storage

This package implements HyperFeed’s on-disk storage.

The `storage` package persists normalized subscriptions, feeds, items, and extended metadata.
Frequently updated scheduler state (ETag, Last-Modified, backoff, stats) is maintained by the
`scheduler` package (e.g., under `hyperfeed/scheduler/<domain>/<feedID>.json`; see that package’s README).



## Schema

* Subscription
* SubscriptionExtendedMetadata
* Feed
* FeedExtendedMetadata (join table)
* Item
* ItemExtendedMetadata (join table)
* ExtendedMetadata
