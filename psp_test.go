package gofeedx_test

import (
	"crypto/sha1"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

// intPtr is a small helper to build optional int values for PSP item config.
func intPtr(i int) *int { return &i }

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

// buildValidPSPFeed creates a PSP feed including all required PSP-1 channel elements
// and a single compliant item, then returns XML output.
func buildValidPSPFeed(t *testing.T) (string, error) {
	t.Helper()

	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Add(item)

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us"). // channel-language
		WithAtomSelf("https://example.com/podcast.rss"). // channel-atom self
		WithItunesImage("https://example.com/artwork.jpg"). // channel-itunes-image
		WithItunesExplicit(false). // channel-itunes-explicit
		WithItunesAuthor("My Podcast Team"). // recommended channel-itunes-author
		WithItunesType("episodic"). // optional channel-itunes-type (episodic is default)
		WithItunesCategory("Technology", "Software"). // required channel-itunes-category (at least one)
		WithPodcastLocked(true). // recommended channel-podcast-locked
		WithPodcastGuidFromURL("https://example.com/podcast.rss"). // recommended channel-podcast-guid
		WithPodcastFunding("https://example.com/support", "Support"). // optional supported channel-podcast-funding
		WithPodcastTXT("ownership-token", "verify"). // optional supported channel-podcast-txt
		ConfigureItem(0, gofeedx.PSPItemConfig{
			ItunesDurationSeconds: 1801, // recommended item-itunes:duration
			ItunesEpisodeType:     "full",
			Transcripts: []gofeedx.PSPTranscript{
				{Url: "https://example.com/ep1.vtt", Type: "text/vtt"},
			},
		})

	if err := psp.Validate(); err != nil {
		return "", err
	}
	return psp.ToPSPRSS()
}

// Test that a fully-configured feed passes validation and includes required namespaces
// and PSP-1 required elements at both channel and item level.
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
	if !strings.Contains(xml, "<itunes:explicit>false</itunes:explicit>") {
		t.Errorf("missing required itunes:explicit element or wrong value")
	}
	if !strings.Contains(xml, `<itunes:image href="https://example.com/artwork.jpg" sop="`) && // guard against attribute reordering
		!strings.Contains(xml, `<itunes:image href="https://example.com/artwork.jpg"`) {
		t.Errorf("missing required itunes:image element with href")
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
		t.Errorf("missing recommended itunes:duration")
	}
	// podcast:transcript requires url and type
	if !strings.Contains(xml, `<podcast:transcript`) ||
		!strings.Contains(xml, `url="https://example.com/ep1.vtt"`) ||
		!strings.Contains(xml, `type="text/vtt"`) {
		t.Errorf("missing or incomplete podcast:transcript element")
	}
}

// Test that ToPSPRSS includes content namespace when HTML content is present (as per README notes),
// and emits a content:encoded element.
func TestPSPContentNamespaceWhenHTMLContent(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	item.Content = "<p>Welcome</p>"
	feed.Add(item)

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesCategory("Technology", "Software").
		ConfigureItem(0, gofeedx.PSPItemConfig{
			ItunesDurationSeconds: 10,
		})
	if err := psp.Validate(); err != nil {
		t.Fatalf("expected valid feed with HTML content, got error: %v", err)
	}
	xml, err := psp.ToPSPRSS()
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
	feed.Add(&gofeedx.Item{
		Title:   "Episode X",
		ID:      "x",
		Created: time.Now(),
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://example.com/audio/x.mp3",
			Type:   "audio/mpeg",
			Length: 100,
		},
	})

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/art.jpg").
		WithItunesExplicit(false).
		WithItunesCategory("News").
		WithPodcastGuidFromURL("https://example.com/podcast.rss").
		ConfigureItem(0, gofeedx.PSPItemConfig{})

	if err := psp.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	xml, err := psp.ToPSPRSS()
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
	feed.Add(newBaseEpisode())

	// Intentionally omit: itunes:category, itunes:image, itunes:explicit, atom self
	psp := gofeedx.NewPSPFeed(feed)

	if err := psp.Validate(); err == nil {
		t.Fatalf("expected Validate to fail when required channel elements are missing")
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
	feed.Add(item)

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesCategory("Technology").
		ConfigureItem(0, gofeedx.PSPItemConfig{})

	if err := psp.Validate(); err == nil {
		t.Fatalf("expected Validate to fail when enclosure attributes are missing")
	}
}

// Test that <atom:link rel="self" type="application/rss+xml"> is rendered correctly.
func TestPSPAtomSelfLinkAttributes(t *testing.T) {
	feed := newBaseFeed()
	feed.Add(newBaseEpisode())

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesCategory("Technology").
		ConfigureItem(0, gofeedx.PSPItemConfig{})

	if err := psp.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	xml, err := psp.ToPSPRSS()
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

// Test that when itunes:type is serial, itunes:episode is required per item.
func TestPSPSerialTypeRequiresEpisodeNumber(t *testing.T) {
	feed := newBaseFeed()
	feed.Add(newBaseEpisode())

	// Mark feed as serial but do not set itunes:episode on the item -> expect validation failure
	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesType("serial").
		WithItunesCategory("Technology").
		ConfigureItem(0, gofeedx.PSPItemConfig{
			// No ItunesEpisode set
			ItunesEpisodeType: "full",
		})

	if err := psp.Validate(); err == nil {
		t.Fatalf("expected Validate to fail for serial feed without itunes:episode on item")
	}

	// Now set the episode number and expect success
	pspOK := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesType("serial").
		WithItunesCategory("Technology").
		ConfigureItem(0, gofeedx.PSPItemConfig{
			ItunesEpisode:     intPtr(1),
			ItunesEpisodeType: "full",
		})
	if err := pspOK.Validate(); err != nil {
		t.Fatalf("expected Validate to succeed for serial feed with itunes:episode, got: %v", err)
	}
}

// Test that itunes:category with subcategory appears as nested elements.
func TestPSPItunesCategoryStructure(t *testing.T) {
	feed := newBaseFeed()
	feed.Add(newBaseEpisode())

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(false).
		WithItunesCategory("Technology", "Software").
		ConfigureItem(0, gofeedx.PSPItemConfig{})

	if err := psp.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	xml, err := psp.ToPSPRSS()
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	// At minimum ensure both category texts appear; nested structure is encouraged by spec
	if !strings.Contains(xml, `itunes:category text="Technology"`) {
		t.Errorf("missing itunes:category Technology")
	}
	if !strings.Contains(xml, `itunes:category text="Software"`) {
		t.Errorf("missing nested itunes:category Software")
	}
}

// Test that itunes:explicit true is rendered for items/channels when configured.
func TestPSPItunesExplicitBooleanValues(t *testing.T) {
	feed := newBaseFeed()
	feed.Add(newBaseEpisode())

	psp := gofeedx.NewPSPFeed(feed).
		WithLanguage("en-us").
		WithAtomSelf("https://example.com/podcast.rss").
		WithItunesImage("https://example.com/artwork.jpg").
		WithItunesExplicit(true). // channel-level explicit true
		WithItunesCategory("Technology").
		ConfigureItem(0, gofeedx.PSPItemConfig{
			ItunesEpisodeType: "full",
		})

	if err := psp.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	xml, err := psp.ToPSPRSS()
	if err != nil {
		t.Fatalf("ToPSPRSS failed: %v", err)
	}

	if !strings.Contains(xml, "<itunes:explicit>true</itunes:explicit>") {
		t.Errorf("expected itunes:explicit true at channel level")
	}
}