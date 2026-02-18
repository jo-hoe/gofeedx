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
 rss, _ := feed.ToRSS()
 fmt.Println(rss)

 // Atom 1.0
 atom, _ := feed.ToAtom()
 fmt.Println(atom)

 // JSON Feed 1.1
 jsonStr, _ := feed.ToJSON()
 fmt.Println(jsonStr)
}
```

## PSP-1 (Podcast) feed

PSP-1 builds on RSS 2.0 and requires specific namespaces and elements. Configure podcast/iTunes fields directly on Feed and Item, then call ToPSPRSS() to render a compliant PSP-1 feed.

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

  // Channel-level PSP/iTunes/podcast settings
  explicit := false
  locked := true
  feed.AtomSelfHref = "https://example.com/podcast.rss"
  feed.ItunesImageHref = "https://example.com/artwork.jpg"
  feed.ItunesExplicit = &explicit
  feed.ItunesAuthor = "My Podcast Team"
  feed.ItunesType = "episodic"
  feed.ItunesCategories = append(feed.ItunesCategories, &gofeedx.ItunesCategory{
    Text: "Technology",
    Sub:  []*gofeedx.ItunesCategory{{Text: "Software"}},
  })
  feed.PodcastLocked = &locked
  feed.PodcastGuidSeed = "https://example.com/podcast.rss" // used to derive podcast:guid via UUIDv5
  feed.PodcastFunding = &gofeedx.PodcastFunding{Url: "https://example.com/support", Text: "Support"}
  feed.PodcastTXT = &gofeedx.PodcastTXT{Value: "ownership-token", Purpose: "verify"}

  // Item-level PSP/iTunes extras
  item.ItunesDurationSeconds = 1801
  item.ItunesEpisode = intPtr(1)
  item.ItunesSeason = intPtr(1)
  item.ItunesEpisodeType = "full"
  item.Transcripts = []gofeedx.PSPTranscript{
    {Url: "https://example.com/ep1.vtt", Type: "text/vtt"},
  }

  if err := feed.ValidatePSP(); err != nil {
    panic(err)
  }
  xml, _ := feed.ToPSPRSS()
  fmt.Println(xml)
}

func intPtr(i int) *int { return &i }
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

## Notes on namespaces

- RSS: content namespace (<http://purl.org/rss/1.0/modules/content/>) is declared only if content:encoded is used.
- Atom: xmlns is set to <http://www.w3.org/2005/Atom> on the <feed> root.
- PSP-1: namespaces for iTunes (<http://www.itunes.com/dtds/podcast-1.0.dtd>), podcast (<https://podcastindex.org/namespace/1.0>) and Atom are declared on the root <rss> element.

## IDs and dates

- RSS: dates use RFC1123Z.
- Atom: dates use RFC3339; entry IDs generated as tag:host,date:path if not provided, else random UUID URN.
- JSON Feed: version 1.1, author array is supported (mapped from single author for convenience).
- PSP-1: podcast:guid is generated via UUID v5 using the feed URL (scheme removed, trailing slashes trimmed) with namespace ead4c236-bf58-58c6-a2c6-a6b28d128cb6.
