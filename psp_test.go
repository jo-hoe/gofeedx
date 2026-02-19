package gofeedx_test

import (
	"crypto/sha1"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

// uuidV5 computes a UUID v5 from a namespace UUID string and a name per RFC 4122.
// It is used here to compute expected podcast:guid values without external resources.
func uuidV5(namespaceUUID, name string) string {
	ns := mustParseUUID(namespaceUUID)
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	var uuid [16]byte
	copy(uuid[:], sum[:16])
	// Set version 5
	uuid[6] = (uuid[6] & 0x0f) | (5 << 4)
	// Set variant to RFC 4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return formatUUID(uuid)
}

func mustParseUUID(s string) [16]byte {
	var out [16]byte
	hex := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '{' || c == '}' {
			continue
		}
		hex = append(hex, c)
	}
	if len(hex) != 32 {
		panic("invalid UUID format")
	}
	for i := 0; i < 16; i++ {
		b := fromHex(hex[i*2])<<4 | fromHex(hex[i*2+1])
		out[i] = byte(b)
	}
	return out
}

func fromHex(b byte) byte {
	switch {
	case '0' <= b && b <= '9':
		return b - '0'
	case 'a' <= b && b <= 'f':
		return b - 'a' + 10
	case 'A' <= b && b <= 'F':
		return b - 'A' + 10
	default:
		panic("invalid hex")
	}
}

func formatUUID(u [16]byte) string {
	b := make([]byte, 36)
	hex := "0123456789abcdef"
	idx := 0
	for i := 0; i < 16; i++ {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			b[idx] = '-'
			idx++
		}
		b[idx] = hex[u[i]>>4]
		b[idx+1] = hex[u[i]&0x0f]
		idx += 2
	}
	return string(b)
}

// newBaseFeed constructs a minimal base feed as documented in README.
func newBaseFeed() *gofeedx.Feed {
	return &gofeedx.Feed{
		Title:       "My Podcast",
		Link:        &gofeedx.Link{Href: "https://example.com/podcast"},
		Description: "A show about Go.",
		Language:    "en-us",
		Created:     time.Now(),
	}
}

// newBaseEpisode constructs a minimal base item as documented in README.
func newBaseEpisode() *gofeedx.Item {
	return &gofeedx.Item{
		Title:   "Episode 1",
		ID:      "ep-1",
		Created: time.Now(),
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://cdn.example.com/audio/ep1.mp3",
			Type:   "audio/mpeg",
			Length: 12345678,
		},
		Description: "We talk about Go modules.",
	}
}

// buildValidPSPFeed creates a PSP feed including required elements from generic fields
// and a single compliant item, then returns XML output.
func buildValidPSPFeed(t *testing.T) (string, error) {
	t.Helper()

	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	// Configure channel fields available in generic structs
	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Author = &gofeedx.Author{Name: "My Podcast Team"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	// Configure item fields available in generic structs
	item.DurationSeconds = 1801

	if err := gofeedx.ValidatePSP(feed); err != nil {
		return "", err
	}
	return gofeedx.ToPSP(feed)
}

// Test that a configured feed passes validation and includes expected namespaces
// and PSP elements derivable from generic structs.
func TestPSPValidMinimalFeed(t *testing.T) {
	xml, err := buildValidPSPFeed(t)
	if err != nil {
		t.Fatalf("expected valid PSP feed, got error: %v", err)
	}

	// Required namespaces for PSP-1
	if !strings.Contains(xml, `xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`) {
		t.Errorf("missing itunes namespace declaration")
	}
	if !strings.Contains(xml, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`) {
		t.Errorf("missing podcast namespace declaration")
	}
	if !strings.Contains(xml, `xmlns:atom="http://www.w3.org/2005/Atom"`) {
		t.Errorf("missing atom namespace declaration pristine")
	}

	// Required channel elements
	if !strings.Contains(xml, "<title>My Podcast</title>") {
		t.Errorf("missing required channel title element")
	}
	if !strings.Contains(xml, "<description>A show about Go.</description>") {
		t.Errorf("missing required channel description element")
	}
	if !strings.Contains(xml, "<link>https://example.com/podcast</link>") {
		t.Errorf("missing required channel link element")
	}
	if !strings.Contains(xml, "<language>en-us</language>") {
		t.Errorf("missing required channel language element")
	}
	if !strings.Contains(xml, "<itunes:category") {
		t.Errorf("missing required itunes:category element")
	}
	// itunes:image derived from generic Image.Url
	if !strings.Contains(xml, `<itunes:image`) || !strings.Contains(xml, `href="https://example.com/artwork.jpg"`) {
		t.Errorf("missing itunes:image element with href from Image.Url")
	}

	// <atom:link rel="self" type="application/rss+xml"/>
	if !strings.Contains(xml, `<atom:link`) || !strings.Contains(xml, `rel="self"`) || !strings.Contains(xml, `type="application/rss+xml"`) {
		t.Errorf("missing required atom:link rel=self type=application/rss+xml")
	}
	if !strings.Contains(xml, `href="https://example.com/podcast.rss"`) {
		t.Errorf("atom:link rel=self missing or wrong href")
	}

	// Required item elements
	if !strings.Contains(xml, "<item>") || !strings.Contains(xml, "</item>") {
		t.Fatalf("missing item element")
	}
	if !strings.Contains(xml, "<title>Episode 1</title>") {
		t.Errorf("missing required item title element")
	}
	if !strings.Contains(xml, "<enclosure ") {
		t.Errorf("missing required item enclosure element")
	}
	if !strings.Contains(xml, `url="https://cdn.example.com/audio/ep1.mp3"`) ||
		!strings.Contains(xml, `type="audio/mpeg"`) ||
		!strings.Contains(xml, `length="12345678"`) {
		t.Errorf("enclosure missing required attributes url, type, or length")
	}
	if !strings.Contains(xml, "<guid") {
		t.Errorf("missing required item guid element")
	}

	// Recommended item elements added by config
	if !strings.Contains(xml, "<itunes:duration>1801</itunes:duration>") {
		t.Errorf("missing recommended itunes:duration from DurationSeconds")
	}
}

// Test that ToPSPRSS includes content namespace when HTML content is present (as per README notes),
// and emits a content:encoded element.
func TestPSPContentNamespaceWhenHTMLContent(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	item.Content = "<p>Welcome</p>"
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})
	item.DurationSeconds = 10

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("expected valid feed with HTML content, got error: %v", err)
	}
	xml, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	if !strings.Contains(xml, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`) {
		t.Errorf("expected content namespace to be declared when HTML content is present")
	}
}

// Test that podcast:guid is generated from URL per spec using UUID v5 with namespace
// ead4c236-bf58-58c6-a2c6-a6b28d128cb6. Deterministic example using example.com.
func TestPSPPodcastGUIDFromURLDeterministic(t *testing.T) {
	feed := &gofeedx.Feed{
		Title:       "Example Show",
		Link:        &gofeedx.Link{Href: "https://example.com"},
		Description: "Example",
		Language:    "en-us",
		Created:     time.Now(),
	}
	feed.Items = append(feed.Items, &gofeedx.Item{
		Title:   "Episode X",
		ID:      "x",
		Created: time.Now(),
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://example.com/audio/x.mp3",
			Type:   "audio/mpeg",
			Length: 100,
		},
	})

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/art.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "News"})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	xml, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	// Compute expected UUIDv5 per PSP spec: namespace ead4c236-bf58-58c6-a2c6-a6b28d128cb6, name = feed URL with scheme removed and trailing slashes trimmed.
	expectedGUID := uuidV5("ead4c236-bf58-58c6-a2c6-a6b28d128cb6", "example.com/podcast.rss")
	expected := "<podcast:guid>" + expectedGUID + "</podcast:guid>"
	if !strings.Contains(xml, expected) {
		t.Errorf("expected podcast:guid %s, got xml: %s", expected, xml)
	}
}

// Test that missing required channel-level elements cause validation to fail.
func TestPSPValidateFailsMissingRequiredChannelElements(t *testing.T) {
	feed := &gofeedx.Feed{
		Title:       "Missing Stuff Podcast",
		Link:        &gofeedx.Link{Href: "https://example.com/podcast"},
		Description: "desc",
		Language:    "en-us",
		Created:     time.Now(),
	}
	feed.Items = append(feed.Items, newBaseEpisode())

	// Intentionally omit categories and atom self (FeedURL)
	if err := gofeedx.ValidatePSP(feed); err == nil {
		t.Fatalf("expected ValidatePSP to fail when required channel elements are missing")
	}
}

// Test that missing or incomplete enclosure attributes cause validation to fail.
func TestPSPValidateFailsMissingEnclosureAttributes(t *testing.T) {
	feed := newBaseFeed()
	item := &gofeedx.Item{
		Title:   "Bad Episode",
		ID:      "bad-1",
		Created: time.Now(),
		// Enclosure with missing type and length
		Enclosure: &gofeedx.Enclosure{
			Url: "https://cdn.example.com/audio/bad.mp3",
		},
	}
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	if err := gofeedx.ValidatePSP(feed); err == nil {
		t.Fatalf("expected ValidatePSP to fail when enclosure attributes are missing")
	}
}

// Test that <atom:link rel="self" type="application/rss+xml"> is rendered correctly.
func TestPSPAtomSelfLinkAttributes(t *testing.T) {
	feed := newBaseFeed()
	feed.Items = append(feed.Items, newBaseEpisode())

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}
	xml, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	if !strings.Contains(xml, `<atom:link`) ||
		!strings.Contains(xml, `rel="self"`) ||
		!strings.Contains(xml, `type="application/rss+xml"`) ||
		!strings.Contains(xml, `href="https://example.com/podcast.rss"`) {
		t.Errorf("atom self link missing or attributes incorrect")
	}
}

// Test that itunes:category is emitted for top-level categories.
func TestPSPItunesCategoryTopLevelOnly(t *testing.T) {
	feed := newBaseFeed()
	feed.Items = append(feed.Items, newBaseEpisode())

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{
		Text: "Technology",
	})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}
	xml, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	// Ensure top-level category appears
	if !strings.Contains(xml, `itunes:category text="Technology"`) {
		t.Errorf("missing itunes:category Technology")
	}
}

// Test that PSP-only fields not represented in generic structs are not emitted by default.
func TestPSPDoesNotEmitExplicitOrLocked(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}
	xml, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	if strings.Contains(xml, "<itunes:explicit>") {
		t.Errorf("did not expect itunes:explicit to be emitted without generic field")
	}
	if strings.Contains(xml, "<podcast:locked>") {
		t.Errorf("did not expect podcast:locked to be emitted without generic field")
	}
}

// Test that channel description limit is enforced.
func TestPSPChannelDescriptionLengthLimit(t *testing.T) {
	// Construct a description of 4001 bytes
	long := strings.Repeat("a", 4001)
	feed := &gofeedx.Feed{
		Title:       "Too Long Desc",
		Link:        &gofeedx.Link{Href: "https://example.com/podcast"},
		Description: long,
		Language:    "en-us",
		Created:     time.Now(),
	}
	feed.Items = append(feed.Items, newBaseEpisode())

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	if err := gofeedx.ValidatePSP(feed); err == nil {
		t.Fatalf("expected ValidatePSP to fail when channel description > 4000 bytes")
	}
}

// Test that PSP-specific elements can still be injected via Extensions.
func TestPSPExtensionsAllowed(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	// Add PSP-like elements via ExtensionNode
	feed.Extensions = []gofeedx.ExtensionNode{
		{Name: "podcast:funding", Attrs: map[string]string{"url": "https://example.com/support"}, Text: "Support Us"},
	}
	item.Extensions = []gofeedx.ExtensionNode{
		{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/cover.jpg"}},
	}

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}
	xmlStr, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	if !strings.Contains(xmlStr, "<podcast:funding") || !strings.Contains(xmlStr, `url="https://example.com/support"`) || !strings.Contains(xmlStr, ">Support Us<") {
		t.Errorf("expected podcast:funding extension in PSP output")
	}
	if !strings.Contains(xmlStr, "<itunes:image") || !strings.Contains(xmlStr, `href="https://example.com/cover.jpg"`) {
		t.Errorf("expected itunes:image extension in PSP item output")
	}
}


// Test that PSP builder-style channel extras are applied.
func TestPSPBuilderChannelExtrasApplied(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Author = &gofeedx.Author{Name: "My Podcast Team"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Original"})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}

	// Override fields using supported generic structs and existing builder capabilities
	feed.Author = &gofeedx.Author{Name: "Override Author"}
	feed.Categories = []*gofeedx.Category{
		{Text: "Technology"},
		{Text: "News"},
	}
	feed.ID = "custom-guid-123"

	// Manually append equivalent PSP channel extension nodes (builder-only assumption)
	feed.Extensions = append(feed.Extensions,
		gofeedx.ExtensionNode{Name: "itunes:explicit", Text: "true"},
		gofeedx.ExtensionNode{Name: "itunes:type", Text: "serial"},
		gofeedx.ExtensionNode{Name: "itunes:complete", Text: "yes"},
		gofeedx.ExtensionNode{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/cover.png"}},
		gofeedx.ExtensionNode{Name: "podcast:locked", Text: "yes"},
		gofeedx.ExtensionNode{Name: "podcast:txt", Text: "ownership-token", Attrs: map[string]string{"purpose": "verify"}},
		gofeedx.ExtensionNode{Name: "podcast:funding", Text: "Support Us", Attrs: map[string]string{"url": "https://example.com/support"}},
	)
	xmlStr, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSSStringOpts failed: %v", err)
	}

	// Channel-level assertions
	if !strings.Contains(xmlStr, "<itunes:explicit>true</itunes:explicit>") {
		t.Errorf("expected itunes:explicit=true in channel")
	}
	if !strings.Contains(xmlStr, "<itunes:type>serial</itunes:type>") {
		t.Errorf("expected itunes:type=serial in channel")
	}
	if !strings.Contains(xmlStr, "<itunes:complete>yes</itunes:complete>") {
		t.Errorf("expected itunes:complete=yes in channel")
	}
	if !strings.Contains(xmlStr, "<itunes:author>Override Author</itunes:author>") {
		t.Errorf("expected overridden itunes:author in channel")
	}
	if !strings.Contains(xmlStr, `<itunes:image`) || !strings.Contains(xmlStr, `href="https://example.com/cover.png"`) {
		t.Errorf("expected overridden itunes:image href in channel")
	}
	if !strings.Contains(xmlStr, `itunes:category text="Technology"`) || !strings.Contains(xmlStr, `itunes:category text="News"`) {
		t.Errorf("expected overridden itunes:category elements in channel")
	}
	if !strings.Contains(xmlStr, "<podcast:locked>yes</podcast:locked>") {
		t.Errorf("expected podcast:locked=yes in channel")
	}
	if !strings.Contains(xmlStr, "<podcast:guid>custom-guid-123</podcast:guid>") {
		t.Errorf("expected overridden podcast:guid in channel")
	}
	if !strings.Contains(xmlStr, `<podcast:txt`) || !strings.Contains(xmlStr, `purpose="verify"`) || !strings.Contains(xmlStr, ">ownership-token<") {
		t.Errorf("expected podcast:txt with purpose and value in channel")
	}
	if !strings.Contains(xmlStr, `<podcast:funding`) || !strings.Contains(xmlStr, `url="https://example.com/support"`) || !strings.Contains(xmlStr, ">Support Us<") {
		t.Errorf("expected podcast:funding with url and label in channel")
	}
}

// Test that PSP builder-style item extras are applied by ID.
func TestPSPBuilderItemExtrasApplied(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	item.ID = "ep-1"
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	if err := gofeedx.ValidatePSP(feed); err != nil {
		t.Fatalf("ValidatePSP failed: %v", err)
	}

	// Manually append equivalent PSP item extension nodes (builder-only assumption)
	item.Extensions = append(item.Extensions,
		gofeedx.ExtensionNode{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/ep1.jpg"}},
		gofeedx.ExtensionNode{Name: "itunes:explicit", Text: "false"},
		gofeedx.ExtensionNode{Name: "itunes:episode", Text: "1"},
		gofeedx.ExtensionNode{Name: "itunes:season", Text: "1"},
		gofeedx.ExtensionNode{Name: "itunes:episodeType", Text: "full"},
		gofeedx.ExtensionNode{Name: "itunes:block", Text: "yes"},
		gofeedx.ExtensionNode{Name: "podcast:transcript", Attrs: map[string]string{"url": "https://example.com/ep1.vtt", "type": "text/vtt"}},
	)
	xmlStr, err := gofeedx.ToPSP(feed)
	if err != nil {
		t.Fatalf("ToPSPRSSStringOpts failed: %v", err)
	}

	// Item-level assertions
	if !strings.Contains(xmlStr, `<itunes:image`) || !strings.Contains(xmlStr, `href="https://example.com/ep1.jpg"`) {
		t.Errorf("expected itunes:image item override")
	}
	if !strings.Contains(xmlStr, "<itunes:explicit>false</itunes:explicit>") {
		t.Errorf("expected itunes:explicit=false on item")
	}
	if !strings.Contains(xmlStr, "<itunes:episode>1</itunes:episode>") {
		t.Errorf("expected itunes:episode on item")
	}
	if !strings.Contains(xmlStr, "<itunes:season>1</itunes:season>") {
		t.Errorf("expected itunes:season on item")
	}
	if !strings.Contains(xmlStr, "<itunes:episodeType>full</itunes:episodeType>") {
		t.Errorf("expected itunes:episodeType on item")
	}
	if !strings.Contains(xmlStr, "<itunes:block>yes</itunes:block>") {
		t.Errorf("expected itunes:block=yes on item")
	}
	if !strings.Contains(xmlStr, `<podcast:transcript`) || !strings.Contains(xmlStr, `url="https://example.com/ep1.vtt"`) || !strings.Contains(xmlStr, `type="text/vtt"`) {
		t.Errorf("expected podcast:transcript on item")
	}
}
