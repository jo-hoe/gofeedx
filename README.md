# gofeedx

[![Test Status](https://github.com/jo-hoe/gofeedx/workflows/test/badge.svg)](https://github.com/jo-hoe/gofeedx/actions?workflow=test)
[![Lint Status](https://github.com/jo-hoe/gofeedx/workflows/lint/badge.svg)](https://github.com/jo-hoe/gofeedx/actions?workflow=lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/jo-hoe/gofeedx)](https://goreportcard.com/report/github.com/jo-hoe/gofeedx)
[![Coverage Status](https://coveralls.io/repos/github/jo-hoe/gofeedx/badge.svg?branch=main)](https://coveralls.io/github/jo-hoe/gofeedx?branch=main)

gofeedx is a small Go library for generating feeds using only the Go standard library.
It exposes a single, consistent, builder-based API and supports custom namespaced extensions via explicit ExtensionNode values.

## Supported formats

- [RSS 2.0.1](https://www.rssboard.org/rss-2-0-1)
- [Atom 1.0](https://www.ietf.org/rfc/rfc4287.txt)
- [JSON Feed 1.1](https://jsonfeed.org/version/1.1/)
- and [PSP-1: The Podcast RSS Standard](https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification)

## Installation

```bash
go get github.com/jo-hoe/gofeedx@latest
```

## Quickstart

Build and render RSS, Atom, PSP (podcast), and JSON Feed using the same canonical model. Use builders for core fields, and WithExtensions for any target-specific elements you need.

```go
package main

import (
 "fmt"
 "time"

 "github.com/jo-hoe/gofeedx"
)

func main() {
 // Build a small site feed
 feed, err := gofeedx.NewFeed("Example Site").
  WithLink("https://example.org").
  WithDescription("Updates from Example Site").
  WithImage("https://example.org/logo.png", "Example Site", "https://example.org").
  WithCategories("Technology").
  // Add one item
  AddItem(
   gofeedx.NewItem("Hello World").
    WithLink("https://example.org/posts/hello-world").
    WithDescription("<p>Welcome!</p>").
    WithCreated(time.Now()),
  ).
  // Add another item with an enclosure (e.g., downloadable asset)
  AddItem(
   gofeedx.NewItem("Downloadable Asset").
    WithLink("https://example.org/downloads/asset").
    WithDescription("Binary download").
    WithEnclosure("https://cdn.example.org/asset.bin", 4096, "application/octet-stream").
    WithCreated(time.Now().Add(-24 * time.Hour)),
  ).
  Build()
 if err != nil {
  panic(err)
 }

 // Optional: validate for a specific format before rendering
 if err := gofeedx.ValidateRSS(feed); err != nil {
  panic(err)
 }

 rssXML, _ := gofeedx.ToRSS(feed)
 atomXML, _ := gofeedx.ToAtom(feed)
 jsonDoc, _ := gofeedx.ToJSON(feed)

 fmt.Println(len(rssXML), len(atomXML), len(jsonDoc))

 // Podcast quickstart (PSP-1): minimal, standards-compliant
 // PSP requires: language, feed URL (for atom:link rel=self), image artwork, at least one itunes:category,
 // and items with enclosure + guid. Duration is recommended.
 podcast, err := gofeedx.NewFeed("My Podcast").
  WithLink("https://example.com/podcast").
  WithFeedURL("https://example.com/podcast.rss").
  WithLanguage("en-us").
  WithDescription("A show about Go.").
  WithImage("https://example.com/podcast-art.png", "My Podcast", "https://example.com/podcast").
  WithCategories("Technology").
  AddItem(
   gofeedx.NewItem("Episode 1").
    WithLink("https://example.com/podcast/ep1").
    WithDescription("We talk about modules.").
    WithCreated(time.Now()).
    WithEnclosure("https://cdn.example.com/audio/ep1.mp3", 12345678, "audio/mpeg").
    WithDurationSeconds(1801),
  ).
  // PSP-specific convenience methods for common fields
  WithPSPExplicit(true).
  WithPSPFunding("https://example.com/support", "Support Us").
  Build()
  
 if err != nil {
  panic(err)
 }
 if err := gofeedx.ValidatePSP(podcast); err != nil {
  panic(err)
 }
 pspXML, _ := gofeedx.ToPSP(podcast)
 fmt.Println(len(pspXML))
}
```

## Namespaces and format notes

- RSS: the content namespace (<http://purl.org/rss/1.0/modules/content/>) is declared only if content:encoded is used.
- Atom: xmlns is set to <http://www.w3.org/2005/Atom> on the feed root element.
- PSP-1: required namespaces for iTunes (<http://www.itunes.com/dtds/podcast-1.0.dtd>), podcast (<https://podcastindex.org/namespace/1.0>), and Atom are declared on the RSS root.

## Field-to-format mapping

### Feed-level mapping

| feed.go field | RSS 2.0 | Atom 1.0 | JSON Feed 1.1 | PSP-1 RSS |
| --- | --- | --- | --- | --- |
| Title | `<channel><title>` | `<feed><title>` | title | `<channel><title>` (required) |
| Link.Href | `<channel><link>` | `<feed><link rel="alternate" href>` | home_page_url | `<channel><link>` (required) |
| Description | `<channel><description>` | `<feed><subtitle>` | description | `<channel><description>` (required, <= 4000 bytes) |
| Author.Name / Author.Email | `<channel><managingEditor>` as "email (Name)" | `<feed><author>` | authors[0].name | itunes:author = Author.Name |
| Updated | `<channel><lastBuildDate>` (RFC1123Z) | `<feed><updated>` (RFC3339; Updated, else Created) | — | `<channel><lastBuildDate>` (RFC1123Z) |
| Created | `<channel><pubDate>` (RFC1123Z) | used in `<feed><updated>` fallback | — | `<channel><pubDate>` (RFC1123Z) |
| ID | — | `<feed><id>` = firstNonEmpty(ID, Link.Href) | — | podcast:guid = ID if set, else UUIDv5(feed_url) |
| Items | `<channel><item>`[] | `<feed><entry>`[] | items[] | `<channel><item>`[] |
| Copyright | `<channel><copyright>` | `<feed><rights>` | — | `<channel><copyright>` |
| Image.Url / Title / Link | `<channel><image>` url/title/link | `<feed><logo>`, `<icon>` = Image.Url | icon, favicon = Image.Url | itunes:image@href = Image.Url |
| Language | `<channel><language>` | — | language | `<channel><language>` (required) |
| Extensions | channel: custom nodes | feed: custom nodes | flattened into top-level keys (name: text) | channel: custom nodes |
| FeedURL | — | — | feed_url | atom:link rel="self" type="application/rss+xml" (required) |
| Categories | `<channel><category>` = first non-empty | `<feed><category>` = first non-empty | — | itunes:category for all non-empty |

### Item-level mapping

| feed.go Item field | RSS 2.0 | Atom 1.0 | JSON Feed 1.1 | PSP-1 RSS |
| --- | --- | --- | --- | --- |
| Title | `<item><title>` | `<entry><title>` | items[].title | `<item><title>` |
| Link.Href | `<item><link>` | `<entry><link rel="alternate">` | items[].url | `<item><link>` (recommended) |
| Source.Href | `<item><source>` | `<entry><link rel="related">` | items[].external_url | — |
| Author.Name / Author.Email | `<item><author>` as "email (Name)" | `<entry><author>` | items[].authors[0].name | — |
| Description | `<item><description>` | `<entry><summary type="html">` | items[].summary | `<item><description>` (recommended) |
| Content (HTML) | content:encoded (CDATA) | `<entry><content type="html">` | items[].content_html | — |
| ID | `<item><guid>` (with isPermaLink) | `<entry><id>` (generated if empty) | items[].id (generated if empty) | `<item><guid>` (generated if empty) |
| IsPermaLink | guid@isPermaLink | — | — | guid@isPermaLink |
| Updated | `<item><pubDate>` (RFC1123Z; Created or Updated) | `<entry><updated>` (RFC3339) | items[].date_modified | `<item><pubDate>` (RFC1123Z) |
| Created | `<item><pubDate>` (RFC1123Z) | `<entry><published>` (RFC3339) | items[].date_published | `<item><pubDate>` (RFC1123Z) |
| Enclosure.Url / Type / Length | `<item><enclosure url type length>` | `<entry><link rel="enclosure" ...>` | image -> items[].image; else attachments[] | `<item><enclosure>` (required) |
| DurationSeconds | — | — | attachments[].duration_in_seconds | itunes:duration |
| Extensions | item: custom nodes | entry: custom nodes | flattened into item (name: text) | item: custom nodes |

## Notes

- Atom dates use RFC3339; RSS/PSP-1 dates use RFC1123Z.
- Atom entry IDs are generated as `tag:host,date:path` when not provided and sufficient link/date context exists; otherwise a random UUID URN is used.
- JSON Feed version 1.1 is produced; a single author maps to authors[0].
- PSP-1 podcast:guid is generated via UUID v5 using the feed URL (scheme removed, trailing slashes trimmed) with namespace `ead4c236-bf58-58c6-a2c6-a6b28d128cb6` when Feed.ID is empty.

