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
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Author      string        `xml:"author"`
	Enclosure   *rssEnclosure `xml:"enclosure"`
	Guid        *rssGuid      `xml:"guid"`
	PubDate     string        `xml:"pubDate"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	PubDate       string    `xml:"pubDate"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
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
	f.Items = append(f.Items, newRSSBaseItem())

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	// Configure some generic fields; ensure PSP-only fields do not leak into plain RSS
	f.FeedURL = "https://example.com/podcast.rss" // PSP adds atom:link rel=self - should not appear in plain RSS writer
	f.Categories = append(f.Categories, &gofeedx.Category{Text: "Technology"})
	item.DurationSeconds = 42
	f.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}

	xmlStr, err := gofeedx.ToRSS(f)
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
	f.Items = append(f.Items, item)

	// Add PSP-like elements via ExtensionNode (allowed exception)
	f.Extensions = []gofeedx.ExtensionNode{
		{Name: "podcast:funding", Attrs: map[string]string{"url": "https://example.com/fund"}, Text: "Support"},
	}
	item.Extensions = []gofeedx.ExtensionNode{
		{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/cover.jpg"}},
	}

	xmlStr, err := gofeedx.ToRSS(f)
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

func TestValidateRSS_Success(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "Desc",
	}
	f.Items = append(f.Items, &gofeedx.Item{Title: "Item 1"})

	if err := gofeedx.ValidateRSS(f); err != nil {
		t.Fatalf("ValidateRSS() unexpected error: %v", err)
	}
}

func TestValidateRSS_ItemNeedsTitleOrDescription(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "Desc",
	}
	// Invalid: item without title and description
	f.Items = append(f.Items, &gofeedx.Item{})
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "must include a title or a description") {
		t.Fatalf("ValidateRSS() expected title/description error, got: %v", err)
	}
}

func TestValidateRSS_EnclosureValidation(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "Desc",
	}
	// Invalid enclosure: length <= 0
	f.Items = append(f.Items, &gofeedx.Item{
		Title: "Item",
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://cdn.example.org/x.mp3",
			Type:   "audio/mpeg",
			Length: 0,
		},
	})
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "enclosure url/type/length required") {
		t.Fatalf("ValidateRSS() expected enclosure error, got: %v", err)
	}
}

func TestValidateRSS_AuthorEmailRequired(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "Desc",
	}
	// Invalid RSS author: missing email
	f.Items = append(f.Items, &gofeedx.Item{
		Title:  "Item",
		Author: &gofeedx.Author{Name: "Alice", Email: ""},
	})
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "author must be an email") {
		t.Fatalf("ValidateRSS() expected author email error, got: %v", err)
	}
}

// Moved from rss_builder_test.go to maintain 1:1 mapping
func TestRSSBuilder_Helpers_ChannelAndItemFields_Moved(t *testing.T) {
	// Build feed using RSS-specific builder helpers
	b := gofeedx.NewFeed("RSS Title").
		WithLink("https://example.org/").
		WithDescription("Desc").
		WithLanguage("en-us").
		WithImage("https://example.org/logo.png", "", "").
		WithRSSTTL(60).
		WithRSSImageSize(144, 144).
		WithRSSCategory("OverrideCat").
		WithRSSWebMaster("webmaster@example.org").
		WithRSSGenerator("gofeedx").
		WithRSSDocs("https://example.org/docs").
		WithRSSCloud("cloud svc").
		WithRSSRating("PG").
		WithRSSSkipHours("1 2").
		WithRSSSkipDays("Mon Tue")

	ib := gofeedx.NewItem("Item 1").
		WithDescription("Item Desc").
		WithCreated(time.Now().UTC()).
		WithRSSItemCategory("ItemCat").
		WithRSSComments("https://example.org/comments/1")
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfileRSS).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	xml, err := gofeedx.ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	// Channel-level checks
	if !strings.Contains(xml, "<ttl>60</ttl>") {
		t.Errorf("expected <ttl>60</ttl> in channel")
	}
	if !strings.Contains(xml, "<category>OverrideCat</category>") {
		t.Errorf("expected channel category override")
	}
	if !strings.Contains(xml, "<webMaster>webmaster@example.org</webMaster>") {
		t.Errorf("expected webMaster element")
	}
	if !strings.Contains(xml, "<generator>gofeedx</generator>") {
		t.Errorf("expected generator element")
	}
	if !strings.Contains(xml, "<docs>https://example.org/docs</docs>") {
		t.Errorf("expected docs element")
	}
	if !strings.Contains(xml, "<cloud>cloud svc</cloud>") {
		t.Errorf("expected cloud element")
	}
	if !strings.Contains(xml, "<rating>PG</rating>") {
		t.Errorf("expected rating element")
	}
	if !strings.Contains(xml, "<skipHours>1 2</skipHours>") {
		t.Errorf("expected skipHours element")
	}
	if !strings.Contains(xml, "<skipDays>Mon Tue</skipDays>") {
		t.Errorf("expected skipDays element")
	}

	// Image size mapping
	if !strings.Contains(xml, "<image>") || !strings.Contains(xml, "<width>144</width>") || !strings.Contains(xml, "<height>144</height>") {
		t.Errorf("expected image with width/height from WithRSSImageSize")
	}

	// Item-level checks for helpers
	if !strings.Contains(xml, "<item>") {
		t.Fatalf("expected an item in RSS output")
	}
	if !strings.Contains(xml, "<category>ItemCat</category>") {
		t.Errorf("expected item category from WithRSSItemCategory")
	}
	if !strings.Contains(xml, "<comments>https://example.org/comments/1</comments>") {
		t.Errorf("expected comments element from WithRSSComments")
	}
}