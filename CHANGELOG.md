# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog], and this project adheres to [Semantic Versioning].

## 0.19.3 (2026-03-31)

* Force link attachments (mimetype `text/x-url`) to `display=1`
* Add a maintenance script to fix non-displayable link attachments

## 0.19.2 (2026-03-31)

* Added `hypergraph add` test harness command (usage: `hypergraph add https://clone.fyi/rss.xml`)
* Fixed RSS item GUID permalink parser (was resolving external links as item URLs)
* Updated `github.com/herdworks/hypergraph-go` to `v0.19.1` with improved feed detection

## 0.19.1 (2026-03-30)

* Updated `github.com/herdworks/hypergraph-go` dependency to `v0.19.0`.

## 0.19.0 (2026-03-30)

Added support for Atom XMLNS extensions, improved RSS link & link attachment parsing, normalized "podcast" detection. Improved YouTube feed parsing!

* Added this changelog 🤌🏽
* Added support for XMLNS extensions in Atom feeds (e.g. for YouTube feed metadata) 
* Added `atom.Atom.Extensions` and `atom.Entry.Extensions` fields
* Added new `github.com/herdworks/hyperfeed-go/types/xmlns` package
* Added `hyperfeed.Item.ContainsPodcastMedia()` method
* Added `hyperfeed.ParseExtensionAttachments()` function for parsing attachments from RSS and Atom XML extension metadata
* Changed RSS and Atom parsers to use `media:description` extension metadata as fallback `item.Summary` (for YouTube feeds)
* Changed parsers to convert `feed.Description` contents to Markdown
* Changed parsers.go to use more consistent `parser.Bylines()` helpers
* Changed `rss.ItemLink()` to correctly return `nil` when `rss.Item.Comments` is empty;
  this fixes "RT" style RSS feeds with items containing external links only (no canonical or Comments URL).
* Changed `hyperfeed.ParseItemLinkAttachments()` to detect external links when the parent item has no URL
* Changed `hyperfeed.Attachment.Displayable()` minimum size threshold from 512px -> 480px for YouTube thumbnails
* Changed `hot_hyperlinks_view` from "3 days" window to "48 hours"
* Changed `hyperfeed.ParseLinks()` to return parser errors
* Removed `github.com/herdworks/hyperfeed-go/types/rss.Extension` type methods in favor of `xmlns.Extension` type 
* Removed `rss.IsPodcast()` and `rss.Item.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed `atom.IsPodcast()` and `atom.Entry.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed `jsonfeed.IsPodcast()` and `jsonfeed.Item.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed XML extension types/methods/functions from `github.com/herdworks/hyperfeed-go/types/rss` package in favor of the new `xmlns` package

> **NOTE:** until now the `github.com/herdworks/hyperfeed-go` package has attempted to stay in sync with the full suite of HyperTexting packages: 
> 
> * `hypergraph-go`
> * `hyperfeed-go`
> * `hypertemplates-go`
> * `hyperproviders-go`
> * `hypertexting-go`
> * `hyperctl`
> * `libhypertexting`
> * `HyperTextingKit`
> * `hypertexting-app`
> 
> Starting with this release we will decouple the library package versions from the user-facing app versions. 


<!-- Links -->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
