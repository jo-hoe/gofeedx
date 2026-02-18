# gofeedx: minimal-dependency feed generator for Go

gofeedx is a small Go library for generating:

- [RSS 2.0.1](https://www.rssboard.org/rss-2-0-1)
- [Atom 1.0](https://www.ietf.org/rfc/rfc4287.txt)
- [PSP-1: The Podcast RSS Standard](https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification)
- and [JSON Feed 1.1](https://jsonfeed.org/version/1.1/)

It follows common Go patterns, uses only the standard library, and supports custom namespaced nodes without extra dependencies.

Install

- Go 1.24+
- Add as a module dependency

Example:

```bash
  go get github.com/jo-hoe/gofeedx@latest
```

## Basic usage

Create a feed and generate RSS, Atom, JSON Feed:

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

    // RSS
    rss, _ := feed.ToRSSString()
    fmt.Println(rss)

    // Atom
    atom, _ := feed.ToAtomString()
    fmt.Println(atom)

    // JSON Feed
    jsonStr, _ := feed.ToJSONString()
    fmt.Println(jsonStr)

    // PSP-1 Podcast RSS
    pspRSS, _ := feed.ToPSPRSSString()
    fmt.Println(pspRSS)
  }
```

Builder pattern for extensions and format-specific fields

There is a single, consistent way to set additional attributes and namespaced nodes across all formats:

- Feed-level: feed.ApplyExtensions(opts...)
- Item-level: item.ApplyExtensions(opts...)
- Typed builders for common formats:
  - PSP/iTunes/Podcasting 2.0: WithPSPChannel, WithPSPItem
  - RSS channel-only fields: WithRSSChannel
- Fully custom nodes: WithCustomFeed, WithCustomItem

## PSP-1 (Podcast) feed

PSP-1 builds on RSS 2.0 and requires specific namespaces and elements. Use generic Feed/Item fields for core data and the unified builders to set PSP/iTunes specifics.

```go
  package main

  import (
    "fmt"
    "time"

    "github.com/jo-hoe/gofeedx"
  )

  func boolPtr(b bool) *bool { return &b }
  func intPtr(i int) *int    { return &i }

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

    // Core PSP channel-level values derived from generic fields
    feed.FeedURL = "https://example.com/podcast.rss"               // atom:link rel=self (required)
    feed.Image = &gofeedx.Image{Url: "https://example.com/art.jpg"} // itunes:image
    feed.Author = &gofeedx.Author{Name: "My Podcast Team"}          // itunes:author
    feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"}) // itunes:category

    // Item-level duration -> itunes:duration
    item.DurationSeconds = 1801

    // Typed PSP/iTunes fields via unified builders
    feed.ApplyExtensions(
      gofeedx.WithPSPChannel(gofeedx.PSPChannelFields{
        ItunesExplicit:  boolPtr(true), // <itunes:explicit>true</itunes:explicit>
        ItunesType:      "serial",      // <itunes:type>serial</itunes:type>
        ItunesComplete:  true,          // <itunes:complete>yes</itunes:complete>
        ItunesAuthor:    "Override Author",
        ItunesImageHref: "https://example.com/cover.png",
        Categories:      []string{"Technology", "News"},
        PodcastLocked:   boolPtr(true), // <podcast:locked>yes</podcast:locked>
        PodcastGuid:     "custom-guid-123",
        PodcastTXT:      &gofeedx.PodcastTXT{Value: "ownership-token", Purpose: "verify"},
        PodcastFunding:  &gofeedx.PodcastFunding{Url: "https://example.com/support", Text: "Support Us"},
      }),
    )

    item.ApplyExtensions(
      gofeedx.WithPSPItem(gofeedx.PSPItemFields{
        ItunesImageHref:   "https://example.com/ep1.jpg",
        ItunesExplicit:    boolPtr(false),
        ItunesEpisode:     intPtr(1),
        ItunesSeason:      intPtr(1),
        ItunesEpisodeType: "full",
        ItunesBlock:       true,
        Transcripts:       []gofeedx.PSPTranscript{{Url: "https://example.com/ep1.vtt", Type: "text/vtt"}},
      }),
    )

    if err := feed.ValidatePSP(); err != nil {
      panic(err)
    }
    xmlStr, _ := feed.ToPSPRSSString()
    fmt.Println(xmlStr)
  }
```

## RSS channel-only fields with the unified builder

Use WithRSSChannel to set RSS 2.0 fields that don’t exist in other formats (e.g., image width/height, TTL, category override):

```go
  feed.ApplyExtensions(
    gofeedx.WithRSSChannel(gofeedx.RSSChannelFields{
      ImageWidth:  1400,
      ImageHeight: 1400,
      TTL:         60,
      Category:    "Technology",
    }),
  )
```

## Adding custom nodes (extensions)

Attach arbitrary namespaced nodes with the unified builder:

### Feed/channel scope

```go
  feed.ApplyExtensions(
    gofeedx.WithCustomFeed(
      gofeedx.ExtensionNode{
        Name: "itunes:owner",
        Children: []gofeedx.ExtensionNode{
          {Name: "itunes:name", Text: "Alice Example"},
          {Name: "itunes:email", Text: "podcast@example.com"},
        },
      },
    ),
  )
```

### Item/entry scope

```go
  item.ApplyExtensions(
    gofeedx.WithCustomItem(
      gofeedx.ExtensionNode{
        Name: "podcast:value",
        Attrs: map[string]string{
          "type":      "lightning",
          "method":    "keysend",
          "suggested": "0.00000015000",
        },
      },
    ),
  )
```

## Notes on namespaces

- RSS: content namespace (http://purl.org/rss/1.0/modules/content/) is declared only if content:encoded is used.
- Atom: xmlns is set to http://www.w3.org/2005/Atom on the feed root element.
- PSP-1: required namespaces for iTunes (http://www.itunes.com/dtds/podcast-1.0.dtd), podcast (https://podcastindex.org/namespace/1.0), and Atom are declared on the RSS root.

## IDs and dates

- RSS: dates use RFC1123Z.
- Atom: dates use RFC3339; entry IDs generated as tag:host,date:path if not provided, else random UUID URN.
- JSON Feed: version 1.1, author array is supported (mapped from single author for convenience).
- PSP-1: podcast:guid is generated via UUID v5 using the feed URL (scheme removed, trailing slashes trimmed) with namespace ead4c236-bf58-58c6-a2c6-a6b28d128cb6 when Feed.ID is empty.

## Field-to-format mapping

The following tables show how generic fields in feed.go map to each target format.

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

## Validation helpers

- ValidateRSS(), ValidateAtom(), ValidateJSON(), ValidatePSP() enforce minimal conformance before writing. Use them to catch issues early.

## License

MIT
