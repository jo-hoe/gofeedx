package gofeedx_test

import (
	"crypto/sha1"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

// Test helpers to reduce per-function branching complexity.
func mustContain(t *testing.T, s, sub, msg string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%s", msg)
	}
}
func mustNotContain(t *testing.T, s, sub, msg string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("%s", msg)
	}
}
func mustNoErr(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", context, err)
	}
}
func mustErr(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s", context)
	}
}

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
	mustNoErr(t, err, "expected valid PSP feed, got error")

	// Required namespaces for PSP-1
	mustContain(t, xml, `xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`, "missing itunes namespace declaration")
	mustContain(t, xml, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`, "missing podcast namespace declaration")
	mustContain(t, xml, `xmlns:atom="http://www.w3.org/2005/Atom"`, "missing atom namespace declaration pristine")

	// Required channel elements
	mustContain(t, xml, "<title>My Podcast</title>", "missing required channel title element")
	mustContain(t, xml, "<description>A show about Go.</description>", "missing required channel description element")
	mustContain(t, xml, "<link>https://example.com/podcast</link>", "missing required channel link element")
	mustContain(t, xml, "<language>en-us</language>", "missing required channel language element")
	mustContain(t, xml, "<itunes:category", "missing required itunes:category element")
	// itunes:image derived from generic Image.Url
	mustContain(t, xml, `<itunes:image`, "missing itunes:image element")
	mustContain(t, xml, `href="https://example.com/artwork.jpg"`, "missing itunes:image href from Image.Url")

	// <atom:link rel="self" type="application/rss+xml"/>
	mustContain(t, xml, `<atom:link`, "missing required atom:link element")
	mustContain(t, xml, `rel="self"`, "missing atom:link rel=self")
	mustContain(t, xml, `type="application/rss+xml"`, "missing atom:link type")
	mustContain(t, xml, `href="https://example.com/podcast.rss"`, "atom:link rel=self missing or wrong href")

	// Required item elements
	mustContain(t, xml, "<item>", "missing item element start")
	mustContain(t, xml, "</item>", "missing item element end")
	mustContain(t, xml, "<title>Episode 1</title>", "missing required item title element")
	mustContain(t, xml, "<enclosure ", "missing required item enclosure element")
	mustContain(t, xml, `url="https://cdn.example.com/audio/ep1.mp3"`, "enclosure missing url")
	mustContain(t, xml, `type="audio/mpeg"`, "enclosure missing type")
	mustContain(t, xml, `length="12345678"`, "enclosure missing length")
	mustContain(t, xml, "<guid", "missing required item guid element")

	// Recommended item elements added by config
	mustContain(t, xml, "<itunes:duration>1801</itunes:duration>", "missing recommended itunes:duration from DurationSeconds")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "expected valid feed with HTML content")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	mustContain(t, xml, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`, "expected content namespace to be declared when HTML content is present")
}

func TestPSPContentNamespaceWhenDescriptionLooksHTML(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	// Clear content and set description with HTML-like tags to trigger heuristic
	item.Content = ""
	item.Description = "Intro <b>bold</b>"
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})
	item.DurationSeconds = 10

	mustNoErr(t, gofeedx.ValidatePSP(feed), "expected valid feed with HTML-like description")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")
	// Heuristic: description contains '<' and '>' -> content namespace should be present
	mustContain(t, xml, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`, "expected content namespace due to HTML-like description")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "Validate failed")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	expectedGUID := uuidV5("ead4c236-bf58-58c6-a2c6-a6b28d128cb6", "example.com/podcast.rss")
	mustContain(t, xml, "<podcast:guid>"+expectedGUID+"</podcast:guid>", "expected podcast:guid value not found")
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
	mustErr(t, gofeedx.ValidatePSP(feed), "expected ValidatePSP to fail when required channel elements are missing")
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

	mustErr(t, gofeedx.ValidatePSP(feed), "expected ValidatePSP to fail when enclosure attributes are missing")
}

// Test that <atom:link rel="self" type="application/rss+xml"> is rendered correctly.
func TestPSPAtomSelfLinkAttributes(t *testing.T) {
	feed := newBaseFeed()
	feed.Items = append(feed.Items, newBaseEpisode())

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	mustContain(t, xml, `<atom:link`, "atom self link missing")
	mustContain(t, xml, `rel="self"`, "atom self link rel missing")
	mustContain(t, xml, `type="application/rss+xml"`, "atom self link type missing")
	mustContain(t, xml, `href="https://example.com/podcast.rss"`, "atom self link href missing")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	mustContain(t, xml, `itunes:category text="Technology"`, "missing itunes:category Technology")
}

// Test that PSP-only fields not represented in generic structs are not emitted by default.
func TestPSPDoesNotEmitExplicitOrLocked(t *testing.T) {
	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	mustNotContain(t, xml, "<itunes:explicit>", "did not expect itunes:explicit to be emitted without generic field")
	mustNotContain(t, xml, "<podcast:locked>", "did not expect podcast:locked to be emitted without generic field")
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

	mustErr(t, gofeedx.ValidatePSP(feed), "expected ValidatePSP to fail when channel description > 4000 bytes")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")
	xmlStr, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSPRSS failed")

	mustContain(t, xmlStr, "<podcast:funding", "expected podcast:funding extension in PSP output")
	mustContain(t, xmlStr, `url="https://example.com/support"`, "expected podcast:funding url attribute")
	mustContain(t, xmlStr, ">Support Us<", "expected podcast:funding label in channel")
	mustContain(t, xmlStr, "<itunes:image", "expected itunes:image extension in PSP item output")
	mustContain(t, xmlStr, `href="https://example.com/cover.jpg"`, "expected itunes:image href in item")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")

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
	mustNoErr(t, err, "ToPSPRSSStringOpts failed")

	// Channel-level assertions
	mustContain(t, xmlStr, "<itunes:explicit>true</itunes:explicit>", "expected itunes:explicit=true in channel")
	mustContain(t, xmlStr, "<itunes:type>serial</itunes:type>", "expected itunes:type=serial in channel")
	mustContain(t, xmlStr, "<itunes:complete>yes</itunes:complete>", "expected itunes:complete=yes in channel")
	mustContain(t, xmlStr, "<itunes:author>Override Author</itunes:author>", "expected overridden itunes:author in channel")
	mustContain(t, xmlStr, "<itunes:image", "expected itunes:image in channel")
	mustContain(t, xmlStr, `href="https://example.com/cover.png"`, "expected overridden itunes:image href in channel")
	mustContain(t, xmlStr, `itunes:category text="Technology"`, "expected itunes:category Technology in channel")
	mustContain(t, xmlStr, `itunes:category text="News"`, "expected itunes:category News in channel")
	mustContain(t, xmlStr, "<podcast:locked>yes</podcast:locked>", "expected podcast:locked=yes in channel")
	mustContain(t, xmlStr, "<podcast:guid>custom-guid-123</podcast:guid>", "expected overridden podcast:guid in channel")
	mustContain(t, xmlStr, "<podcast:txt", "expected podcast:txt in channel")
	mustContain(t, xmlStr, `purpose="verify"`, "expected podcast:txt purpose attr in channel")
	mustContain(t, xmlStr, ">ownership-token<", "expected podcast:txt value in channel")
	mustContain(t, xmlStr, "<podcast:funding", "expected podcast:funding in channel")
	mustContain(t, xmlStr, `url="https://example.com/support"`, "expected podcast:funding url attr in channel")
	mustContain(t, xmlStr, ">Support Us<", "expected podcast:funding label in channel")
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

	mustNoErr(t, gofeedx.ValidatePSP(feed), "ValidatePSP failed")

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
	mustNoErr(t, err, "ToPSPRSSStringOpts failed")

	// Item-level assertions
	mustContain(t, xmlStr, "<itunes:image", "expected itunes:image item override")
	mustContain(t, xmlStr, `href="https://example.com/ep1.jpg"`, "expected itunes:image item href")
	mustContain(t, xmlStr, "<itunes:explicit>false</itunes:explicit>", "expected itunes:explicit=false on item")
	mustContain(t, xmlStr, "<itunes:episode>1</itunes:episode>", "expected itunes:episode on item")
	mustContain(t, xmlStr, "<itunes:season>1</itunes:season>", "expected itunes:season on item")
	mustContain(t, xmlStr, "<itunes:episodeType>full</itunes:episodeType>", "expected itunes:episodeType on item")
	mustContain(t, xmlStr, "<itunes:block>yes</itunes:block>", "expected itunes:block=yes on item")
	mustContain(t, xmlStr, "<podcast:transcript", "expected podcast:transcript on item")
	mustContain(t, xmlStr, `url="https://example.com/ep1.vtt"`, "expected transcript url attr on item")
	mustContain(t, xmlStr, `type="text/vtt"`, "expected transcript type attr on item")
}

// Added to consolidate helper tests into psp_test.go using public API

func TestPSPFallbackGuid_TagAndURN_PublicAPI(t *testing.T) {
	// Case 1: item without ID but with Link+Created -> tag: URI generated
	feed1 := &gofeedx.Feed{
		Title:       "Show",
		Link:        &gofeedx.Link{Href: "https://example.com"},
		Description: "d",
		Language:    "en-us",
		FeedURL:     "https://example.com/podcast.rss",
		Categories:  []*gofeedx.Category{{Text: "Tech"}},
	}
	item1 := &gofeedx.Item{
		Title:   "E1",
		Link:    &gofeedx.Link{Href: "https://example.com/ep/1"},
		Created: time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC),
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://cdn.example.com/ep1.mp3",
			Type:   "audio/mpeg",
			Length: 10,
		},
	}
	feed1.Items = []*gofeedx.Item{item1}
	xml1, err := gofeedx.ToPSP(feed1)
	mustNoErr(t, err, "ToPSP error")
	mustContain(t, xml1, "<guid>tag:example.com,2024-02-03:/ep/1</guid>", "expected tag: guid for item1")

	// Case 2: item without ID and without Link/Created -> urn:uuid generated
	feed2 := &gofeedx.Feed{
		Title:       "Show",
		Link:        &gofeedx.Link{Href: "https://example.com"},
		Description: "d",
		Language:    "en-us",
		FeedURL:     "https://example.com/podcast.rss",
		Categories:  []*gofeedx.Category{{Text: "Tech"}},
	}
	item2 := &gofeedx.Item{
		Title: "E2",
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://cdn.example.com/ep2.mp3",
			Type:   "audio/mpeg",
			Length: 10,
		},
	}
	feed2.Items = []*gofeedx.Item{item2}
	xml2, err := gofeedx.ToPSP(feed2)
	mustNoErr(t, err, "ToPSP error")
	mustContain(t, xml2, "<guid>urn:uuid:", "expected urn:uuid fallback for item2")
}

func TestPSPPodcastGUIDFromURL_UppercaseScheme(t *testing.T) {
	// normalizeFeedURL only trims lowercase http/https/feed; uppercase HTTPS:// remains
	feed := &gofeedx.Feed{
		Title:       "Upper",
		Link:        &gofeedx.Link{Href: "https://example.com"},
		Description: "d",
		Language:    "en-us",
		FeedURL:     "HTTPS://example.com/podcast.rss",
		Categories:  []*gofeedx.Category{{Text: "News"}},
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
	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSP error")
	expected := uuidV5("ead4c236-bf58-58c6-a2c6-a6b28d128cb6", "HTTPS://example.com/podcast.rss")
	mustContain(t, xml, "<podcast:guid>"+expected+"</podcast:guid>", "expected podcast:guid missing or wrong")
}

func TestPSPBuilderHelpers_ChannelScope(t *testing.T) {
	// Build via FeedBuilder and apply PSP-specific helpers
	b := gofeedx.NewFeed("My Podcast").
		WithLink("https://example.com/podcast").
		WithDescription("A show").
		WithLanguage("en-us").
		WithFeedURL("https://example.com/podcast.rss").
		WithCategories("Technology")

	// PSP builder helpers
	b = b.
		WithPSPExplicit(true).
		WithPSPLocked(true).
		WithPSPTXT("ownership", "verify").
		WithPSPItunesType("serial").
		WithPSPItunesComplete(true).
		WithPSPImageHref("https://example.com/art2.jpg")

	// Minimal item
	ib := gofeedx.NewItem("Ep 1").
		WithEnclosure("https://cdn.example.com/ep1.mp3", 100, "audio/mpeg")
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfilePSP).Build()
	mustNoErr(t, err, "Build PSP feed")
	xml, err := gofeedx.ToPSP(f)
	mustNoErr(t, err, "ToPSP failed")

	// Channel-level assertions from builder helpers
	mustContain(t, xml, "<itunes:explicit>true</itunes:explicit>", "expected itunes:explicit=true at channel")
	mustContain(t, xml, "<podcast:locked>yes</podcast:locked>", "expected podcast:locked yes at channel")
	mustContain(t, xml, "<podcast:txt", "expected podcast:txt at channel")
	mustContain(t, xml, `purpose="verify"`, "expected podcast:txt purpose attribute")
	mustContain(t, xml, ">ownership<", "expected podcast:txt value at channel")
	mustContain(t, xml, "<itunes:type>serial</itunes:type>", "expected itunes:type serial at channel")
	mustContain(t, xml, "<itunes:complete>yes</itunes:complete>", "expected itunes:complete yes at channel")
	mustContain(t, xml, "<itunes:image", "expected itunes:image element at channel")
	mustContain(t, xml, `href="https://example.com/art2.jpg"`, "expected itunes:image href override")
}

func TestPSPBuilderHelpers_ItemScope(t *testing.T) {
	// Feed with minimal required fields via builder
	b := gofeedx.NewFeed("Show").
		WithLink("https://example.com/show").
		WithDescription("d").
		WithLanguage("en-us").
		WithFeedURL("https://example.com/podcast.rss").
		WithCategories("Tech")

	// Item with PSP-specific helpers
	ib := gofeedx.NewItem("Ep").
		WithEnclosure("https://cdn.example.com/ep.mp3", 123, "audio/mpeg").
		WithPSPExplicit(false).
		WithPSPTranscript("https://example.com/ep.vtt", "text/vtt", "en", "captions").
		WithPSPImageHref("https://example.com/ep.jpg").
		WithPSPEpisode(2).
		WithPSPSeason(1).
		WithPSPEpisodeType("trailer").
		WithPSPBlock(true)
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfilePSP).Build()
	mustNoErr(t, err, "Build PSP feed with item helpers")
	xml, err := gofeedx.ToPSP(f)
	mustNoErr(t, err, "ToPSP failed")

	// Item-level assertions
	mustContain(t, xml, "<itunes:explicit>false</itunes:explicit>", "expected itunes:explicit=false on item")
	mustContain(t, xml, "<itunes:image", "expected itunes:image on item")
	mustContain(t, xml, `href="https://example.com/ep.jpg"`, "expected itunes:image href on item")
	mustContain(t, xml, "<itunes:episode>2</itunes:episode>", "expected itunes:episode on item")
	mustContain(t, xml, "<itunes:season>1</itunes:season>", "expected itunes:season on item")
	mustContain(t, xml, "<itunes:episodeType>trailer</itunes:episodeType>", "expected itunes:episodeType on item")
	mustContain(t, xml, "<itunes:block>yes</itunes:block>", "expected itunes:block yes on item")
	mustContain(t, xml, "<podcast:transcript", "expected podcast:transcript on item")
	mustContain(t, xml, `url="https://example.com/ep.vtt"`, "expected transcript url attr on item")
	mustContain(t, xml, `type="text/vtt"`, "expected transcript type attr on item")
	mustContain(t, xml, `language="en"`, "expected transcript language attr on item")
	mustContain(t, xml, `rel="captions"`, "expected transcript rel attr on item")
}

func TestPSPBuilderHelpers_InvalidInputsAreIgnored(t *testing.T) {
	// Channel-level invalid itunes:type should be ignored
	b := gofeedx.NewFeed("Show").
		WithLink("https://example.com").
		WithDescription("d").
		WithLanguage("en-us").
		WithFeedURL("https://example.com/podcast.rss").
		WithCategories("Tech")
	// Invalid channel helper inputs
	b = b.WithPSPItunesType("invalid") // ignored
	// Minimal item
	ib := gofeedx.NewItem("Ep").WithEnclosure("https://cdn.example.com/ep.mp3", 100, "audio/mpeg")
	b.AddItem(ib)
	f, err := b.WithProfiles(gofeedx.ProfilePSP).Build()
	mustNoErr(t, err, "Build PSP feed")
	xml, err := gofeedx.ToPSP(f)
	mustNoErr(t, err, "ToPSP failed")
	// itunes:type should not be present
	mustNotContain(t, xml, "<itunes:type>", "did not expect itunes:type for invalid input")

	// Item-level invalid episodeType should be ignored, and 0 values episode/season ignored
	b2 := gofeedx.NewFeed("Show2").
		WithLink("https://example.com").
		WithDescription("d").
		WithLanguage("en-us").
		WithFeedURL("https://example.com/podcast.rss").
		WithCategories("Tech")
	ib2 := gofeedx.NewItem("Ep2").
		WithEnclosure("https://cdn.example.com/ep2.mp3", 100, "audio/mpeg").
		WithPSPEpisode(0). // ignored
		WithPSPSeason(0).  // ignored
		WithPSPEpisodeType("foo") // ignored
	b2.AddItem(ib2)
	f2, err := b2.WithProfiles(gofeedx.ProfilePSP).Build()
	mustNoErr(t, err, "Build PSP feed 2")
	xml2, err := gofeedx.ToPSP(f2)
	mustNoErr(t, err, "ToPSP failed 2")
	mustNotContain(t, xml2, "<itunes:episode>", "did not expect itunes:episode for value 0")
	mustNotContain(t, xml2, "<itunes:season>", "did not expect itunes:season for value 0")
	mustNotContain(t, xml2, "<itunes:episodeType>", "did not expect itunes:episodeType for invalid value")
}

func TestValidatePSP_ItemMissingID(t *testing.T) {
	// ValidatePSP should fail when item ID (guid) is missing and we don't run builder fallback
	feed := newBaseFeed()
	item := newBaseEpisode()
	item.ID = "" // missing guid
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	err := gofeedx.ValidatePSP(feed)
	if err == nil || !strings.Contains(err.Error(), "guid (ID) required") {
		t.Fatalf("ValidatePSP() expected item guid/ID required error, got: %v", err)
	}
}

func TestPSP_NoContentNamespaceWhenNoHTML(t *testing.T) {
	// Ensure PSP root does not include content namespace when neither Content nor HTML-like Description exist
	feed := newBaseFeed()
	item := newBaseEpisode()
	item.Content = ""
	item.Description = "Plain text only"
	feed.Items = append(feed.Items, item)

	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSP failed")
	mustNotContain(t, xml, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`, "did not expect content namespace without HTML content/description")
}

// Additional PSP item validation coverage: item description length limit
func TestValidatePSP_ItemDescriptionLengthLimit(t *testing.T) {
	feed := newBaseFeed()
	// Valid feed-level requirements
	feed.FeedURL = "https://example.com/podcast.rss"
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	// Item with description exceeding 4000 bytes -> should fail ValidatePSPItems
	item := newBaseEpisode()
	item.Description = strings.Repeat("x", 4001)
	feed.Items = []*gofeedx.Item{item}

	err := gofeedx.ValidatePSP(feed)
	if err == nil || !strings.Contains(err.Error(), "description must be <= 4000 bytes") {
		t.Fatalf("ValidatePSP() expected item description length limit error, got: %v", err)
	}
}

func TestPSPChannelLockedFalseAndExplicitFalse(t *testing.T) {
	// Build minimal valid PSP feed
	b := gofeedx.NewFeed("Show").
		WithLink("https://example.com").
		WithDescription("desc").
		WithLanguage("en-us").
		WithFeedURL("https://example.com/podcast.rss").
		WithCategories("Tech")

	// Apply channel helpers with false values
	b = b.WithPSPLocked(false).WithPSPExplicit(false)

	// Minimal item
	ib := gofeedx.NewItem("Ep").
		WithEnclosure("https://cdn.example.com/ep.mp3", 100, "audio/mpeg")
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfilePSP).Build()
	mustNoErr(t, err, "Build PSP feed (locked=false, explicit=false)")
	xml, err := gofeedx.ToPSP(f)
	mustNoErr(t, err, "ToPSP failed")

	// Assert channel-level locked and explicit false encodings
	mustContain(t, xml, "<podcast:locked>no</podcast:locked>", "expected podcast:locked=no at channel")
	mustContain(t, xml, "<itunes:explicit>false</itunes:explicit>", "expected itunes:explicit=false at channel")
}

func TestPSPAtomSelfLinkOmittedWhenFeedURLEmpty(t *testing.T) {
	// Ensure atom:link rel=self is omitted when FeedURL is empty
	feed := newBaseFeed()
	item := newBaseEpisode()
	feed.Items = append(feed.Items, item)

	// Do not set FeedURL, but set other required fields
	feed.Image = &gofeedx.Image{Url: "https://example.com/artwork.jpg"}
	feed.Categories = append(feed.Categories, &gofeedx.Category{Text: "Technology"})

	xml, err := gofeedx.ToPSP(feed)
	mustNoErr(t, err, "ToPSP failed without FeedURL")
	mustNotContain(t, xml, "<atom:link", "did not expect atom:link when FeedURL is empty")
}
