gofeedx: minimal-dependency feed generator for Go

gofeedx is a small Go library for generating:

- RSS 2.0
- Atom 1.0
- PSP-1: The Podcast RSS Standard (RSS 2.0 with itunes/podcast/atom namespaces)
- JSON Feed 1.1

It follows common Go patterns, uses only the standard library, and allows adding custom XML nodes (extensions) without extra dependencies.

Install

- Go 1.24+
- Add as a module dependency (local dev uses a replace or your VCS URL if published)

Core model

- Feed: shared model across formats
- Item: shared item/entry model

Types (simplified)

- Feed
  - Title, Link, Description, Author, Updated, Created, ID, Subtitle
  - Items []*Item
  - Copyright, Image, Language
  - CustomChannelNodes []XMLNode
- Item
  - Title, Link, Source, Author, Description, ID, IsPermaLink
  - Updated, Created
  - Enclosure (Url, Length, Type)
  - Content (HTML)
  - CustomItemNodes []XMLNode
- XMLNode: lightweight arbitrary XML element structure (name, attrs, text, children)

Usage

Create a basic feed and generate RSS, Atom, JSON Feed
package main

import (
 "fmt"
 "time"

 "github.com/jo-hoe/gofeedx"
)

func main() {
 feed := &gofeedx.Feed{
  Title:       "My Blog",
  Link:        &gofeedx.Link{Href: "<https://example.com"}>,
  Description: "Example feed",
  Language:    "en-us",
  Author:      &gofeedx.Author{Name: "Alice", Email: "<alice@example.com>"},
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

 // RSS 2.0
 rss, _ := feed.ToRSS()
 fmt.Println(rss)

 // Atom 1.0
 atom, _ := feed.ToAtom()
 fmt.Println(atom)

 // JSON Feed 1.1
 jsonStr, _ := feed.ToJSON()
 fmt.Println(jsonStr)
}

PSP-1 (Podcast) feed
PSP-1 builds on RSS 2.0 and requires specific namespaces and elements. Use the builder PSPFeed to ensure required fields are present.

feed := &gofeedx.Feed{
 Title:       "My Podcast",
 Link:        &gofeedx.Link{Href: "<https://example.com/podcast"}>,
 Description: "A show about Go.",
 Language:    "en-us",
 Created:     time.Now(),
}
feed.Add(&gofeedx.Item{
 Title:     "Episode 1",
 ID:        "ep-1",
 Created:   time.Now(),
 Enclosure: &gofeedx.Enclosure{
  Url:    "<https://cdn.example.com/audio/ep1.mp3>",
  Type:   "audio/mpeg",
  Length: 12345678, // bytes
 },
 Description: "We talk about Go modules.",
})

psp := gofeedx.NewPSPFeed(feed).
 WithLanguage("en-us").
 WithAtomSelf("<https://example.com/podcast.rss>").
 WithItunesImage("<https://example.com/artwork.jpg>").
 WithItunesExplicit(false).
 WithItunesAuthor("My Podcast Team").
 WithItunesType("episodic").
 WithItunesCategory("Technology", "Software").
 WithPodcastLocked(true).
 WithPodcastGuidFromURL("<https://example.com/podcast.rss>").
 WithPodcastFunding("<https://example.com/support>", "Support").
 WithPodcastTXT("ownership-token", "verify").
 ConfigureItem(0, gofeedx.PSPItemConfig{
  ItunesDurationSeconds: 1801,
  ItunesEpisode:         intPtr(1),
  ItunesSeason:          intPtr(1),
  ItunesEpisodeType:     "full",
  Transcripts: []gofeedx.PSPTranscript{
   {Url: "<https://example.com/ep1.vtt>", Type: "text/vtt"},
  },
 })

if err := psp.Validate(); err != nil {
 panic(err)
}
xml, _ := psp.ToPSPRSS()
fmt.Println(xml)

func intPtr(i int) *int { return &i }

Adding custom XML nodes (extensions)
You can attach arbitrary namespaced nodes to channel/feed and item/entry scopes.

feed.CustomChannelNodes = append(feed.CustomChannelNodes, gofeedx.XMLNode{
 Name: "itunes:owner",
 Children: []gofeedx.XMLNode{
  {Name: "itunes:name", Text: "Alice Example"},
  {Name: "itunes:email", Text: "<podcast@example.com>"},
 },
})

// Per item
feed.Items[0].CustomItemNodes = append(feed.Items[0].CustomItemNodes, gofeedx.XMLNode{
 Name: "podcast:value",
 Attrs: map[string]string{
  "type":     "lightning",
  "method":   "keysend",
  "suggested": "0.00000015000",
 },
})

Notes on namespaces

- RSS 2.0: content namespace (<http://purl.org/rss/1.0/modules/content/>) is declared only if content:encoded is used.
- PSP RSS: xmlns:itunes, xmlns:podcast, xmlns:atom are always declared. content namespace is declared when HTML content is present.
- Atom: xmlns is set to <http://www.w3.org/2005/Atom> on the <feed> root.

IDs and dates

- RSS: dates use RFC1123Z.
- Atom: dates use RFC3339; entry IDs generated as tag:host,date:path if not provided, else random UUID URN.
- JSON Feed: version 1.1, author array is supported (mapped from single author for convenience).
- PSP-1: podcast:guid is generated via UUID v5 using the feed URL (scheme removed, trailing slashes trimmed) with namespace ead4c236-bf58-58c6-a2c6-a6b28d128cb6.
