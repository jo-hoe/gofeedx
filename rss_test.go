package gofeedx_test

import (
	"encoding/xml"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type rssGuid struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr,omitempty"`
}

type rssItem struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	Author      string       `xml:"author"`
	Enclosure   *rssEnclosure `xml:"enclosure"`
	Guid        *rssGuid     `xml:"guid"`
	PubDate     string       `xml:"pubDate"`
}

type rssChannel struct {
	Title         string     `xml:"title"`
	Link          string     `xml:"link"`
	Description   string     `xml:"description"`
	Language      string     `xml:"language"`
	PubDate       string     `xml:"pubDate"`
	LastBuildDate string     `xml:"lastBuildDate"`
	Items         []rssItem  `xml:"item"`
}

type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

func newRSSBaseFeed() *gofeedx.Feed {
	return &gofeedx.Feed{
		Title:       "Example RSS Feed",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "A feed for RSS 2.0 tests.",
		Language:    "en-us",
		Created:     time.Now().UTC(),
		Updated:     time.Now().UTC(),
	}
}

func newRSSBaseItem() *gofeedx.Item {
	return &gofeedx.Item{
		Title:       "Item 1",
		Description: "Desc 1",
		Link:        &gofeedx.Link{Href: "https://example.org/item1"},
		Created:     time.Now().UTC(),
		Updated:     time.Now().UTC(),
	}
}

func TestRSSChannelRequiredElementsPresent(t *testing.T) {
	f := newRSSBaseFeed()
	f.Add(newRSSBaseItem())

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	if !strings.Contains(xmlStr, `<rss version="2.0"`) {
		t.Errorf("missing or wrong <rss version=\"2.0\"> root")
	}
	if !strings.Contains(xmlStr, "<title>Example RSS Feed</title>") {
		t.Errorf("missing required channel title")
	}
	if !strings.Contains(xmlStr, "<link>https://example.org/</link>") {
		t.Errorf("missing required channel link")
	}
	if !strings.Contains(xmlStr, "<description>A feed for RSS 2.0 tests.</description>") {
		t.Errorf("missing required channel description")
	}

	// Parse and verify date formats (RFC1123Z)
	var doc rssRoot
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if _, err := time.Parse(time.RFC1123Z, doc.Channel.PubDate); err != nil {
		t.Errorf("channel pubDate must be RFC1123Z, got %q: %v", doc.Channel.PubDate, err)
	}
	if doc.Channel.LastBuildDate != "" {
		if _, err := time.Parse(time.RFC1123Z, doc.Channel.LastBuildDate); err != nil {
			t.Errorf("channel lastBuildDate must be RFC1123Z, got %q: %v", doc.Channel.LastBuildDate, err)
		}
	}
}

func TestRSSContentNamespaceWhenContentEncoded(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	item.Content = "<p>HTML Content</p>"
	f.Add(item)

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	// Expect content namespace declaration
	if !strings.Contains(xmlStr, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`) {
		t.Errorf("expected xmlns:content declaration when content:encoded is used")
	}
	// Expect content:encoded element
	if !strings.Contains(xmlStr, "<content:encoded><![CDATA[") || !strings.Contains(xmlStr, "HTML Content") {
		t.Errorf("expected content:encoded element with CDATA content")
	}
}

func TestRSSEnclosureAttributesRequired(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	item.Enclosure = &gofeedx.Enclosure{
		Url:    "https://cdn.example.org/audio.mp3",
		Type:   "audio/mpeg",
		Length: 12345,
	}
	f.Add(item)

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	var doc rssRoot
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if len(doc.Channel.Items) == 0 {
		t.Fatalf("expected at least one item")
	}
	encl := doc.Channel.Items[0].Enclosure
	if encl == nil {
		t.Fatalf("expected enclosure element")
	}
	if encl.URL == "" || encl.Type == "" || encl.Length == "" {
		t.Errorf("enclosure must include url, type, and length attributes")
	}
	if _, err := strconv.ParseInt(encl.Length, 10, 64); err != nil {
		t.Errorf("enclosure length must be integer, got %q: %v", encl.Length, err)
	}
}

func TestRSSItemAuthorUsesEmailPerSpec(t *testing.T) {
	// RSS 2.0 author (item) should be an email address. This test encodes the spec rule.
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	item.Author = &gofeedx.Author{Name: "Alice", Email: "alice@example.org"}
	f.Add(item)

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	var doc rssRoot
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	got := doc.Channel.Items[0].Author
	if got == "" || !strings.Contains(got, "@") {
		t.Errorf("RSS 2.0 item author should be email address per spec; got %q", got)
	}
}

func TestRSSItemGuidAndIsPermaLink(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	item.ID = "abc-123"
	item.IsPermaLink = "false"
	f.Add(item)

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	var doc rssRoot
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	g := doc.Channel.Items[0].Guid
	if g == nil {
		t.Fatalf("expected guid element")
	}
	if g.Value != "abc-123" {
		t.Errorf("guid value=%q, want %q", g.Value, "abc-123")
	}
	if g.IsPermaLink != "false" {
		t.Errorf("guid isPermaLink=%q, want \"false\"", g.IsPermaLink)
	}
}

func TestRSSItemTitleOrDescriptionPresent(t *testing.T) {
	// Spec: an item should have at least a title or a description.
	// Build an item with title but no description to confirm at least one is present.
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	item.Description = ""
	f.Add(item)

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	if !strings.Contains(xmlStr, "<title>Item 1</title>") {
		t.Errorf("item should include at least a title or a description")
	}
}

func TestRSSDoesNotIncludePSPFields(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	f.Add(item)

	// Configure PSP-only fields that should not leak into plain RSS
	explicit := true
	locked := true
	f.FeedURL = "https://example.com/podcast.rss" // PSP adds atom:link rel=self
	f.ItunesImageHref = "https://example.com/artwork.jpg"
	f.ItunesExplicit = &explicit
	f.ItunesType = "episodic"
	f.ItunesComplete = true
	f.Categories = append(f.Categories, &gofeedx.Category{Text: "Technology"})
	f.PodcastLocked = &locked
	f.PodcastFunding = &gofeedx.PodcastFunding{Url: "https://example.com/fund", Text: "Fund us"}
	f.PodcastTXT = &gofeedx.PodcastTXT{Purpose: "verify", Value: "token"}

	item.DurationSeconds = 42
	item.ItunesImageHref = "https://example.com/item.jpg"
	item.ItunesExplicit = &explicit
	ep := 1
	se := 1
	item.ItunesEpisode = &ep
	item.ItunesSeason = &se
	item.ItunesEpisodeType = "full"
	item.ItunesBlock = false
	item.Transcripts = []gofeedx.PSPTranscript{{Url: "https://example.com/t.vtt", Type: "text/vtt"}}

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	// Assert PSP namespaces DO NOT appear
	if strings.Contains(xmlStr, `xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`) {
		t.Errorf("unexpected itunes namespace in plain RSS output")
	}
	if strings.Contains(xmlStr, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`) {
		t.Errorf("unexpected podcast namespace in plain RSS output")
	}
	if strings.Contains(xmlStr, `xmlns:atom="http://www.w3.org/2005/Atom"`) {
		t.Errorf("unexpected atom namespace in plain RSS output")
	}

	// Assert PSP elements DO NOT appear
	if strings.Contains(xmlStr, "<itunes:") {
		t.Errorf("unexpected itunes:* elements in plain RSS output")
	}
	if strings.Contains(xmlStr, "<podcast:") {
		t.Errorf("unexpected podcast:* elements in plain RSS output")
	}
	if strings.Contains(xmlStr, "<atom:link") {
		t.Errorf("unexpected atom:link in plain RSS output")
	}
}

func TestRSSExtensionNodesAllowed(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	f.Add(item)

	// Add PSP-like elements via ExtensionNode (allowed exception)
	f.Extensions = []gofeedx.ExtensionNode{
		{Name: "podcast:funding", Attrs: map[string]string{"url": "https://example.com/fund"}, Text: "Support"},
	}
	item.Extensions = []gofeedx.ExtensionNode{
		{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/cover.jpg"}},
	}

	xmlStr, err := f.ToRSS()
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	if !strings.Contains(xmlStr, "<podcast:funding") || !strings.Contains(xmlStr, `url="https://example.com/fund"`) {
		t.Errorf("expected podcast:funding extension in RSS output")
	}
	if !strings.Contains(xmlStr, "<itunes:image") || !strings.Contains(xmlStr, `href="https://example.com/cover.jpg"`) {
		t.Errorf("expected itunes:image extension in RSS item output")
	}
}
