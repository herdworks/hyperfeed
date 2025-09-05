# HyperFeed Cache

This package implements HyperFeed’s on-disk cache.

The `store` package reads `scheduler` and `subscription` state; it only writes the rebuildable artifacts under hyperfeed/cache/…. All frequently updated metadata (ETag, Last-Modified, backoff, stats) is maintained by the scheduler package in hyperfeed/scheduler/<domain>/<feedID>.json (see that package’s README).



## Schema

* Subscription
* SubscriptionExtendedMetadata
* Feed
* FeedExtendedMetadata (join table)
* Item
* ItemExtendedMetadata (join table)
* ExtendedMetadata

