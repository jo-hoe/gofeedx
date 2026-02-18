# gofeedx: minimal-dependency feed generator for Go

gofeedx is a small Go library for generating:

- [RSS 2.0.1](https://www.rssboard.org/rss-2-0-1)
- [Atom](https://www.ietf.org/rfc/rfc4287.txt)
- [PSP-1: The Podcast RSS Standard](https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification)
- and [JSON Feed 1.1](https://jsonfeed.org/version/1.1/)

It follows common Go patterns, uses only the standard library, and allows adding custom nodes (extensions) without extra dependencies.

## Install

- Go 1.24+
- Add as a module dependency (local dev uses a replace or your VCS URL if published)

Example:

```bash
go get github.com/jo-hoe/gofeedx@latest
```

## Usage

Create a basic feed and generate RSS, Atom, JSON Feed:

```go
package main

import (
 "fmt"
 "time"

 "github.com/jo-hoe/gofeedx"
)

func main() {
 feed := &gofeedx.Feed{
  Title:       "My Blog",
  Link:        &gofeedx.Link{Href: "https://example.com"},
  Description: "Example feed",
  Language:    "en-us",
  Author:      &gofeedx.Author{Name: "Alice", Email: "alice@example.com"},
  Created:     time.Now(),
 }

 item := &gofeedx.Item{
  Title:       "Hello World",
  Link:        &gofeedx.Link{Href: "https://example.com/hello"},
  Description: "First post",
  ID:          "post-1",
  Created:     time.Now(),
  Content:     "<p>Welcome to my blog.</p>",
  Enclosure:   &gofeedx.Enclosure{Url: "https://example.com/cover.jpg", Type: "image/jpeg", Length: 0},
 }
 feed.Add(item)

 // RSS 2.0.1
 rss, _ := feed.ToRSSString()
 fmt.Println(rss)

 // Atom 1.0
 atom, _ := feed.ToAtomString()
 fmt.Println(atom)

 // JSON Feed 1.1
 jsonStr, _ := feed.ToJSONString()
 fmt.Println(jsonStr)
}
```

## PSP-1 (Podcast) feed

PSP-1 builds on RSS 2.0 and requires specific namespaces and elements. Use the generic fields on Feed and Item; the PSP encoder will emit the required iTunes and podcast:* elements.

```go
package main

import (
  "fmt"
  "time"

  "github.com/jo-hoe/gofeedx"
)

func main() {
  feed := &gofeedx.Feed{
    Title:       "My Podcast",
    Link:        &gofeedx.Link{Href: "https://example.com/podcast"},
    Description: "A show about Go.",
    Language:    "en-us",
    Created:     time.Now(),
  }

  item := &gofeedx.Item{
    Title:   "Episode 1",
    ID:      "ep-1",
    Created: time.Now(),
    Enclosure: &gofeedx.Enclosure{
      Url:    "https://cdn.example.com/audio/ep1.mp3",
      Type:   "audio/mpeg",
      Length: 12345678, // bytes
    },
    Description: "We talk about Go modules.",
  }
  feed.Add(item)

  // Channel-level PSP/iTunes/podcast essentials derived from generic fields
  feed.FeedURL = "https://example.com/podcast.rss" // atom:link rel=self
  feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"} // itunes:image
  feed.Author = &gofeedx.Author{Name: "My Podcast Team"}              // itunes:author
  feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"}) // itunes:category

  // Item-level duration (maps to itunes:duration)
  item.DurationSeconds = 1801

  if err := feed.ValidatePSP(); err != nil {
    panic(err)
  }
  xml, _ := feed.ToPSPRSSString()
  fmt.Println(xml)
}
```

## Adding custom XML nodes (extensions)

You can attach arbitrary namespaced nodes to channel/feed and item/entry scopes using ExtensionNode.

Channel-level example:

```go
feed.Extensions = append(feed.Extensions, gofeedx.ExtensionNode{
  Name: "itunes:owner",
  Children: []gofeedx.ExtensionNode{
    {Name: "itunes:name", Text: "Alice Example"},
    {Name: "itunes:email", Text: "podcast@example.com"},
  },
})
```

Per item:

```go
feed.Items[0].Extensions = append(feed.Items[0].Extensions, gofeedx.ExtensionNode{
  Name: "podcast:value",
  Attrs: map[string]string{
    "type":      "lightning",
    "method":    "keysend",
    "suggested": "0.00000015000",
  },
})
```

## Target-specific fields: idiomatic patterns

You can keep using the generic Feed/Item structs while supplying format-specific details in two idiomatic ways:

1) Functional options for RSS-only fields (encoder-time)
Use options with the Opts variants to set fields like image width/height and TTL that exist only in RSS:
```go
rssXML, _ := feed.ToRSSStringOpts(
  gofeedx.WithRSSImageSize(1400, 1400),
  gofeedx.WithRSSTTL(60),
  gofeedx.WithRSSChannelCategory("Technology"),
)
```

2) Namespaced extensions (plain RSS) or PSP encoder (iTunes/podcast)
- For plain RSS, you can inject namespaced elements via ExtensionNode at channel or item scope:
```go
// Channel-level custom node (e.g., itunes:owner)
gofeedx.AppendFeedExtensions(feed, gofeedx.ExtensionNode{
  Name: "itunes:owner",
  Children: []gofeedx.ExtensionNode{
    {Name: "itunes:name", Text: "Alice Example"},
    {Name: "itunes:email", Text: "podcast@example.com"},
  },
})
// Item-level custom node (e.g., itunes:image)
gofeedx.AppendItemExtensions(item, gofeedx.ExtensionNode{
  Name:  "itunes:image",
  Attrs: map[string]string{"href": "https://example.com/cover.jpg"},
})
```
- For podcast feeds, prefer the PSP encoder (ToPSPRSSString/Feed) which emits required iTunes and podcast:* elements and enforces PSP-1. Use generic fields (FeedURL, Image.Url, Author, Categories, DurationSeconds, etc.) and/or attach additional PSP elements using ExtensionNode.

## Notes on namespaces

- RSS: content namespace (<http://purl.org/rss/1.0/modules/content/>) is declared only if content:encoded is used.
- Atom: xmlns is set to <http://www.w3.org/2005/Atom> on the `<feed>` root.
- PSP-1: namespaces for iTunes (<http://www.itunes.com/dtds/podcast-1.0.dtd>), podcast (<https://podcastindex.org/namespace/1.0>) and Atom are declared on the root `<rss>` element.

## IDs and dates

- RSS: dates use RFC1123Z.
- Atom: dates use RFC3339; entry IDs generated as tag:host,date:path if not provided, else random UUID URN.
- JSON Feed: version 1.1, author array is supported (mapped from single author for convenience).
- PSP-1: podcast:guid is generated via UUID v5 using the feed URL (scheme removed, trailing slashes trimmed) with namespace ead4c236-bf58-58c6-a2c6-a6b28d128cb6.

## Field-to-format mapping

The following tables show how generic fields in feed.go map to each target format. See “IDs and dates” above for generation and formatting rules.

### Feed-level mapping

| feed.go field | RSS 2.0 | Atom 1.0 | JSON Feed 1.1 | PSP-1 RSS |
| --- | --- | --- | --- | --- |
| Title | `<channel><title>` | `<feed><title>` | `title` | `<channel><title>` (required) |
| Link.Href | `<channel><link>` | `<feed><link rel="alternate" href>` | `home_page_url` | `<channel><link>` (required) |
| Description | `<channel><description>` | `<feed><subtitle>` | `description` | `<channel><description>` (required, <= 4000 bytes) |
| Author.Name / Author.Email | `<channel><managingEditor>` as "email (Name)" | `<feed><author><name>`, `<email>` | `authors[0].name` | `itunes:author` = Author.Name |
| Updated | `<channel><lastBuildDate>` (RFC1123Z) | `<feed><updated>` (RFC3339; Updated, else Created) | — | `<channel><lastBuildDate>` (RFC1123Z) |
| Created | `<channel><pubDate>` (RFC1123Z) | used as fallback in `<feed><updated>` (RFC3339) | — | `<channel><pubDate>` (RFC1123Z) |
| ID | — | `<feed><id>` = firstNonEmpty(ID, Link.Href) | — | `podcast:guid` = ID if set, else UUIDv5(feed_url) |
| Items | `<channel><item>[]` | `<feed><entry>[]` | `items[]` | `<channel><item>[]` |
| Copyright | `<channel><copyright>` | `<feed><rights>` | — | `<channel><copyright>` |
| Image.Url / Title / Link | `<channel><image> url/title/link` | `<feed><logo>` and `<icon>` = Image.Url | `icon` and `favicon` = Image.Url | `itunes:image@href` = Image.Url |
| Language | `<channel><language>` | — | `language` | `<channel><language>` (required) |
| Extensions | channel: appended as custom nodes | feed: appended as custom nodes | flattened into top-level keys (name: text) | channel: appended as custom nodes |
| FeedURL | — | — | `feed_url` | `atom:link rel="self" type="application/rss+xml"` (required) |
| Categories (top-level) | `<channel><category>` = first non-empty | `<feed><category>` = first non-empty | — | `itunes:category` from all non-empty categories |

Notes:

- Atom and RSS only use the first top-level Category (if present).
- PSP-1 maps all non-empty Categories to `itunes:category` (single level) via convertCategories.
- JSON Feed does not map feed-level Categories; use item-level tags if needed (not part of generic Item in this library).

### Item-level mapping

| feed.go Item field | RSS 2.0 | Atom 1.0 | JSON Feed 1.1 | PSP-1 RSS |
| --- | --- | --- | --- | --- |
| Title | `<item><title>` | `<entry><title>` | `items[].title` | `<item><title>` |
| Link.Href | `<item><link>` | `<entry><link rel="alternate">` | `items[].url` | `<item><link>` (recommended) |
| Source.Href | `<item><source>` | `<entry><link rel="related">` | `items[].external_url` | — |
| Author.Name / Author.Email | `<item><author>` as "email (Name)" | `<entry><author><name>`, `<email>` | `items[].authors[0].name` | — |
| Description | `<item><description>` | `<entry><summary type="html">` | `items[].summary` | `<item><description>` (recommended) |
| Content (HTML) | `content:encoded` (CDATA) | `<entry><content type="html">` | `items[].content_html` | — |
| ID | `<item><guid>` (with optional `isPermaLink`) | `<entry><id>` (generated if empty) | `items[].id` (generated if empty) | `<item><guid>` (generated if empty) |
| IsPermaLink | `guid@isPermaLink` ("true"/"false"/omit) | — | — | `guid@isPermaLink` |
| Updated | `<item><pubDate>` (RFC1123Z; Created or Updated) | `<entry><updated>` (RFC3339) | `items[].date_modified` | `<item><pubDate>` (RFC1123Z) |
| Created | `<item><pubDate>` (RFC1123Z) | `<entry><published>` (RFC3339) | `items[].date_published` | `<item><pubDate>` (RFC1123Z) |
| Enclosure.Url / Type / Length | `<item><enclosure url type length>` (all required by spec) | `<entry><link rel="enclosure" type length>` | image -> `items[].image`; otherwise `attachments[]` with `url`,`mime_type`,`size` | `<item><enclosure>` (required) |
| DurationSeconds | — | — | `attachments[].duration_in_seconds` | `itunes:duration` |
| Extensions | item: appended as custom nodes | entry: appended as custom nodes | flattened into item object (name: text) | item: appended as custom nodes |

Notes:

- JSON Feed enclosure mapping: if Enclosure.Type starts with "image/", it populates the `items[].image` field; otherwise it adds a JSON attachment with `url`, `mime_type`, `size` (int32-capped), and optional duration.
- ID generation when missing uses a tag:host,date:path URI if Link.Href and any timestamp exists; otherwise a URN with a random UUID v4.
