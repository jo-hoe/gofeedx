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
	mustNoErr(t, err, "ToRSS failed")

	mustContain(t, xmlStr, `<rss version="2.0"`, "missing or wrong <rss version=\"2.0\"> root")
	mustContain(t, xmlStr, "<title>Example RSS Feed</title>", "missing required channel title")
	mustContain(t, xmlStr, "<link>https://example.org/</link>", "missing required channel link")
	mustContain(t, xmlStr, "<description>A feed for RSS 2.0 tests.</description>", "missing required channel description")

	// Parse and verify date formats (RFC1123Z)
	var doc rssRoot
	mustNoErr(t, xml.Unmarshal([]byte(xmlStr), &doc), "xml unmarshal")

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
	mustNoErr(t, err, "ToRSS failed")

	// Expect content namespace declaration
	mustContain(t, xmlStr, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`, "expected xmlns:content declaration when content:encoded is used")
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
	mustNoErr(t, err, "ToRSS failed")

	var doc rssRoot
	mustNoErr(t, xml.Unmarshal([]byte(xmlStr), &doc), "xml unmarshal")
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
	mustNoErr(t, err, "ToRSS failed")

	var doc rssRoot
	mustNoErr(t, xml.Unmarshal([]byte(xmlStr), &doc), "xml unmarshal")
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
	mustNoErr(t, err, "ToRSS failed")

	var doc rssRoot
	mustNoErr(t, xml.Unmarshal([]byte(xmlStr), &doc), "xml unmarshal")
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
	mustNoErr(t, err, "ToRSS failed")
	mustContain(t, xmlStr, "<title>Item 1</title>", "item should include at least a title or a description")
}

func TestRSSDoesNotIncludePSPFields(t *testing.T) {
	f := newRSSBaseFeed()
	item := newRSSBaseItem()
	f.Items = append(f.Items, item)

	// Configure some generic fields; ensure PSP-only fields do not leak into plain RSS
	f.FeedURL = "https://example.com/podcast.rss"
	f.Categories = append(f.Categories, &gofeedx.Category{Text: "Technology"})
	item.DurationSeconds = 42
	f.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}

	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")

	mustNotContain(t, xmlStr, `xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`, "unexpected itunes namespace in plain RSS output")
	mustNotContain(t, xmlStr, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`, "unexpected podcast namespace in plain RSS output")
	mustNotContain(t, xmlStr, `xmlns:atom="http://www.w3.org/2005/Atom"`, "unexpected atom namespace in plain RSS output")
	mustNotContain(t, xmlStr, "<itunes:", "unexpected itunes:* elements in plain RSS output")
	mustNotContain(t, xmlStr, "<podcast:", "unexpected podcast:* elements in plain RSS output")
	mustNotContain(t, xmlStr, "<atom:link", "unexpected atom:link in plain RSS output")
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
	mustNoErr(t, err, "ToRSS failed")

	mustContain(t, xmlStr, "<podcast:funding", "expected podcast:funding extension in RSS output")
	mustContain(t, xmlStr, `url="https://example.com/fund"`, "expected podcast:funding url attr in channel")
	mustContain(t, xmlStr, "<itunes:image", "expected itunes:image extension in RSS item output")
	mustContain(t, xmlStr, `href="https://example.com/cover.jpg"`, "expected itunes:image href in item")
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

func TestValidateRSS_MissingChannelTitle(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "Desc",
	}
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "channel title required") {
		t.Fatalf("ValidateRSS() expected channel title required error, got: %v", err)
	}
}

func TestValidateRSS_MissingChannelLink(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        nil,
		Description: "Desc",
	}
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "channel link required") {
		t.Fatalf("ValidateRSS() expected channel link required error, got: %v", err)
	}
}

func TestValidateRSS_MissingChannelDescription(t *testing.T) {
	f := &gofeedx.Feed{
		Title:       "RSS Title",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "",
	}
	err := gofeedx.ValidateRSS(f)
	if err == nil || !strings.Contains(err.Error(), "channel description required") {
		t.Fatalf("ValidateRSS() expected channel description required error, got: %v", err)
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
	mustNoErr(t, err, "Build() unexpected error")

	xml, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")

	// Channel-level checks
	mustContain(t, xml, "<ttl>60</ttl>", "expected <ttl>60</ttl> in channel")
	mustContain(t, xml, "<category>OverrideCat</category>", "expected channel category override")
	mustContain(t, xml, "<webMaster>webmaster@example.org</webMaster>", "expected webMaster element")
	mustContain(t, xml, "<generator>gofeedx</generator>", "expected generator element")
	mustContain(t, xml, "<docs>https://example.org/docs</docs>", "expected docs element")
	mustContain(t, xml, "<cloud>cloud svc</cloud>", "expected cloud element")
	mustContain(t, xml, "<rating>PG</rating>", "expected rating element")
	mustContain(t, xml, "<skipHours>1 2</skipHours>", "expected skipHours element")
	mustContain(t, xml, "<skipDays>Mon Tue</skipDays>", "expected skipDays element")

	// Image size mapping
	mustContain(t, xml, "<image>", "expected image element in channel")
	mustContain(t, xml, "<width>144</width>", "expected image width from WithRSSImageSize")
	mustContain(t, xml, "<height>144</height>", "expected image height from WithRSSImageSize")

	// Item-level checks for helpers
	mustContain(t, xml, "<item>", "expected an item in RSS output")
	mustContain(t, xml, "<category>ItemCat</category>", "expected item category from WithRSSItemCategory")
	mustContain(t, xml, "<comments>https://example.org/comments/1</comments>", "expected comments element from WithRSSComments")
}

// Additional RSS tests to increase coverage of mapping helpers and conditions

func TestRSS_NoContentNamespaceWhenNoContentEncoded(t *testing.T) {
	f := newRSSBaseFeed()
	// No item.Content set
	f.Items = append(f.Items, newRSSBaseItem())
	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")
	mustNotContain(t, xmlStr, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`, "did not expect content namespace when no content:encoded")
}

func TestRSS_ItemSourceLinkMapped(t *testing.T) {
	f := newRSSBaseFeed()
	it := newRSSBaseItem()
	it.Source = &gofeedx.Link{Href: "https://mirror.example.org/item1"}
	f.Items = append(f.Items, it)
	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")

	// Expect <source> element carrying the source href
	mustContain(t, xmlStr, "<source>https://mirror.example.org/item1</source>", "expected source element from Item.Source")
}

func TestRSS_ManagingEditorFromAuthorFormatting(t *testing.T) {
	f := newRSSBaseFeed()
	// Set feed.Author; managingEditor should use email (Name) formatting
	f.Author = &gofeedx.Author{Name: "Alice", Email: "alice@example.org"}
	f.Items = append(f.Items, newRSSBaseItem())
	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")

	mustContain(t, xmlStr, "<managingEditor>alice@example.org (Alice)</managingEditor>", "expected managingEditor formatted as email (Name)")
}

func TestRSS_ChannelCategoryFromGenericMapping(t *testing.T) {
	f := newRSSBaseFeed()
	// Provide top-level categories without RSS override -> first category should map
	f.Categories = []*gofeedx.Category{{Text: "Tech"}, {Text: "News"}}
	f.Items = append(f.Items, newRSSBaseItem())

	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")
	mustContain(t, xmlStr, "<category>Tech</category>", "expected channel category mapped from first generic category")
}

// Additional coverage: guid attribute behaviors and enclosure omission

func TestRSSGuidIsPermaLinkOmittedWhenEmpty(t *testing.T) {
	f := newRSSBaseFeed()
	it := newRSSBaseItem()
	it.ID = "abc"
	it.IsPermaLink = "" // omitempty
	f.Items = append(f.Items, it)
	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")
	// Guid value present, isPermaLink attribute omitted
	mustContain(t, xmlStr, "<guid>abc</guid>", "expected guid value element without isPermaLink attribute")
	mustNotContain(t, xmlStr, `isPermaLink="`, "did not expect isPermaLink attribute when empty")
}

func TestRSSEnclosureOmittedWhenInvalid(t *testing.T) {
	f := newRSSBaseFeed()
	it := newRSSBaseItem()
	// Invalid enclosure (missing type and length) -> should not emit enclosure element
	it.Enclosure = &gofeedx.Enclosure{Url: "", Type: "", Length: 0}
	f.Items = append(f.Items, it)
	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")
	mustNotContain(t, xmlStr, "<enclosure ", "did not expect enclosure element when invalid attributes")
}

// Additional encoding cases

func TestRSSItemDescriptionOmittedWhenWhitespaceOnly(t *testing.T) {
	f := newRSSBaseFeed()
	it := newRSSBaseItem()
	it.Description = "   " // whitespace-only should be trimmed to empty and element omitted
	f.Items = append(f.Items, it)

	xmlStr, err := gofeedx.ToRSS(f)
	mustNoErr(t, err, "ToRSS failed")

	// Inspect only the first item block to avoid matching the channel-level description
	start := strings.Index(xmlStr, "<item>")
	if start == -1 {
		t.Fatalf("expected <item> element present")
	}
	rest := xmlStr[start:]
	end := strings.Index(rest, "</item>")
	if end == -1 {
		t.Fatalf("expected </item> closing tag present")
	}
	itemBlock := rest[:end]
	mustNotContain(t, itemBlock, "<description>", "did not expect item description element when whitespace-only")
}
