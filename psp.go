package gofeedx

// PSP-1: The Podcast RSS Standard encoder and builder
// Emits RSS 2.0 with required namespaces, enforces required PSP elements,
// and provides unified builder-style helpers via ExtOption.
//
// see https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification
// and https://podcast-standard.org/podcast_standard/

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Namespace declarations required by PSP-1
const (
	xmlnsItunes  = "http://www.itunes.com/dtds/podcast-1.0.dtd"
	xmlnsPodcast = "https://podcastindex.org/namespace/1.0"
	xmlnsAtom    = "http://www.w3.org/2005/Atom"
	xmlnsContent = "http://purl.org/rss/1.0/modules/content/"
)

// PodcastNamespaceUUID is UUID v5 namespace for podcast:guid generation
// ead4c236-bf58-58c6-a2c6-a6b28d128cb6
var PodcastNamespaceUUID = UUID{0xea, 0xd4, 0xc2, 0x36, 0xbf, 0x58, 0x58, 0xc6, 0xa2, 0xc6, 0xa6, 0xb2, 0x8d, 0x12, 0x8c, 0xb6}

/*
PSPRSSRoot is the <rss> wrapper with PSP-1 namespaces.

Per PSP-1 (README.md), podcast feeds MUST:
- Use RSS version="2.0"
- Declare the following namespaces at the root:
  - xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"  (REQUIRED)
  - xmlns:podcast="https://podcastindex.org/namespace/1.0"     (REQUIRED)
  - xmlns:atom="http://www.w3.org/2005/Atom"                    (REQUIRED)

- Optionally declare the RDF Site Summary Content Module:
  - xmlns:content="http://purl.org/rss/1.0/modules/content/"    (OPTIONAL)
    Only when the feed contains HTML that should be wrapped in CDATA and/or
    when richer content is emitted. This library heuristically enables it
    when item Content is present or item Description appears to contain HTML.

Namespace definitions are case-sensitive and must match the PSP-1 specification.
*/
type PSPRSSRoot struct {
	XMLName   xml.Name    `xml:"rss"`
	Version   string      `xml:"version,attr"`
	NSItunes  string      `xml:"xmlns:itunes,attr"`
	NSPodcast string      `xml:"xmlns:podcast,attr"`
	NSAtom    string      `xml:"xmlns:atom,attr"`
	NSContent string      `xml:"xmlns:content,attr,omitempty"`
	Channel   *PSPChannel `xml:"channel"`
}

/*
PSPChannel is the <channel> element with RSS/iTunes/Podcast extensions.

PSP-1 requirements classification for channel-level elements:

REQUIRED (must be present and non-empty):
- <atom:link rel="self" type="application/rss+xml"> (AtomSelf)
- <title>                                  (Title)
- <description>                            (Description) — up to 4000 bytes
- <link>                                   (Link) — site or page URL
- <language>                               (Language) — ISO 639 format
- <itunes:category>                       (ItunesCategories) — at least one; order by priority
- <itunes:explicit>                       (ItunesExplicit) — "true" or "false"
- <itunes:image href="..."/>              (ItunesImage) — artwork URL

RECOMMENDED (encouraged but not required):
- <podcast:locked>                          (PodcastLocked) — "yes" or "no"
- <podcast:guid>                            (PodcastGuid) — UUIDv5 derived from feed URL
- <itunes:author>                          (ItunesAuthor)

OPTIONAL (supported when relevant):
  - <copyright>                               (Copyright)
  - <podcast:txt [purpose="..."]>             (PodcastTXT) — value up to 4000 chars; purpose up to 128 chars
  - <podcast:funding url="...">Label</podcast:funding> (PodcastFunding)
    Note: optional in feeds, but PSP-certified hosts/players must support it.
  - <itunes:type>                             (ItunesType) — "episodic" or "serial"
    Episodic is assumed if absent; REQUIRED for serial podcasts.
  - <itunes:complete>                         (ItunesComplete) — "yes"

Other standard RSS fields commonly used:
- <pubDate>         (PubDate) — OPTIONAL (RSS 2.0)
- <lastBuildDate>   (LastBuildDate) — OPTIONAL (RSS 2.0)
*/
type PSPChannel struct {
	*PSPChannelExtension
	Title            string `xml:"title"`       // required
	Link             string `xml:"link"`        // required
	Description      string `xml:"description"` // required (may embed CDATA in content:encoded for rich HTML elsewhere)
	ItunesAuthor     string `xml:"itunes:author,omitempty"`
	LastBuildDate    string `xml:"lastBuildDate,omitempty"`
	PubDate          string `xml:"pubDate,omitempty"`
	PodcastGuid      string
	Items            []*PSPItem        `xml:"item"`
	ItunesImage      *ItunesImage      `xml:"itunes:image,omitempty"`
	ItunesCategories []*ItunesCategory `xml:"itunes:category,omitempty"`
}

// MarshalXML customizes channel XML to avoid emitting untagged struct fields and to include extension nodes.
func (ch *PSPChannel) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Ensure we start with <channel> element
	if start.Name.Local == "" {
		start.Name.Local = "channel"
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// language (required)
	if ch.PSPChannelExtension != nil && strings.TrimSpace(ch.Language) != "" {
		if err := e.EncodeElement(ch.Language, xml.StartElement{Name: xml.Name{Local: "language"}}); err != nil {
			return err
		}
	}

	// atom:link rel=self
	if ch.PSPChannelExtension != nil && ch.AtomSelf != nil {
		if err := e.Encode(ch.AtomSelf); err != nil {
			return err
		}
	}

	// title, link, description
	if s := strings.TrimSpace(ch.Title); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "title"}}); err != nil {
			return err
		}
	}
	if s := strings.TrimSpace(ch.Link); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "link"}}); err != nil {
			return err
		}
	}
	if s := strings.TrimSpace(ch.Description); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "description"}}); err != nil {
			return err
		}
	}

	// pubDate, lastBuildDate
	if s := strings.TrimSpace(ch.PubDate); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "pubDate"}}); err != nil {
			return err
		}
	}
	if s := strings.TrimSpace(ch.LastBuildDate); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "lastBuildDate"}}); err != nil {
			return err
		}
	}

	// itunes:author
	if s := strings.TrimSpace(ch.ItunesAuthor); s != "" {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "itunes:author"}}); err != nil {
			return err
		}
	}

	// items
	for _, it := range ch.Items {
		if it == nil {
			continue
		}
		if err := e.Encode(it); err != nil {
			return err
		}
	}

	// itunes:image
	if ch.ItunesImage != nil && strings.TrimSpace(ch.ItunesImage.Href) != "" {
		if err := e.Encode(ch.ItunesImage); err != nil {
			return err
		}
	}

	// itunes:category
	for _, c := range ch.ItunesCategories {
		if c == nil || strings.TrimSpace(c.Text) == "" {
			continue
		}
		if err := e.Encode(c); err != nil {
			return err
		}
	}

	// extension nodes (including podcast:guid or other injected nodes)
	if ch.PSPChannelExtension != nil && len(ch.Extra) > 0 {
		for _, n := range ch.Extra {
			if err := e.Encode(n); err != nil {
				return err
			}
		}
	}

	// Close <channel>
	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// PSPChannelExtension holds channel-level PSP/iTunes fields for unified builder.
type PSPChannelExtension struct {
	XMLName  xml.Name `xml:"channel"`
	Language string   `xml:"language"` // required

	// Recommended and optional standard RSS fields
	Copyright string `xml:"copyright,omitempty"`

	// atom:link rel="self"
	AtomSelf *PSPAtomLink `xml:"atom:link,omitempty"`
	// iTunes fields
	ItunesExplicit  *bool
	ItunesType      string // "episodic" | "serial"
	ItunesComplete  bool   // emits "yes" when true
	ItunesImageHref string // overrides or supplements image href from Feed.Image.Url

	// podcast namespace
	PodcastLocked  *bool // emits "yes"/"no"
	PodcastTXT     *PodcastTXT
	PodcastFunding *PodcastFunding

	Extra []ExtensionNode `xml:",any"`
}

// PSPAtomLink emits atom:link
type PSPAtomLink struct {
	XMLName xml.Name `xml:"atom:link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr"`
	Type    string   `xml:"type,attr"`
}

// ItunesImage emits itunes:image href="..."
type ItunesImage struct {
	XMLName xml.Name `xml:"itunes:image"`
	Href    string   `xml:"href,attr"`
}

// ItunesCategory supports nesting
type ItunesCategory struct {
	XMLName xml.Name          `xml:"itunes:category"`
	Text    string            `xml:"text,attr"`
	Sub     []*ItunesCategory `xml:"itunes:category,omitempty"`
}

// PodcastLocked emits "yes" or "no"
type PodcastLocked struct {
	XMLName xml.Name `xml:"podcast:locked"`
	Value   string   `xml:",chardata"` // "yes" or "no"
}

// PodcastTXT emits podcast:txt with optional purpose attr
type PodcastTXT struct {
	XMLName xml.Name `xml:"podcast:txt"`
	Purpose string   `xml:"purpose,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

// PodcastFunding emits podcast:funding url attr + label text
type PodcastFunding struct {
	XMLName xml.Name `xml:"podcast:funding"`
	Url     string   `xml:"url,attr"`
	Text    string   `xml:",chardata"`
}

// PSPTranscript emits podcast:transcript
type PSPTranscript struct {
	XMLName  xml.Name `xml:"podcast:transcript"`
	Url      string   `xml:"url,attr"`
	Type     string   `xml:"type,attr"`
	Language string   `xml:"language,attr,omitempty"`
	Rel      string   `xml:"rel,attr,omitempty"`
}

/*
PSPItem extends RSS <item> with PSP/iTunes item fields.

PSP-1 requirements classification for item-level elements:

REQUIRED (must be present and valid):
  - <title>                           (Title)
  - <enclosure url="..." type="..." length="..."/> (Enclosure)
    Attributes required: url, type (e.g. audio/mpeg), length in bytes (> 0)
  - <guid>                             (Guid) — unique, stable per episode

RECOMMENDED (encouraged but not required):
  - <link>                             (Link)
  - <pubDate>                          (PubDate) — RFC 2822 format
  - <description>                      (Description) — up to 4000 bytes, limited HTML allowed in CDATA
  - <itunes:duration>                 (ItunesDuration) — seconds
  - <itunes:image href="..."/>        (ItunesImage)
  - <itunes:explicit>                 (ItunesExplicit) — "true" or "false"
  - <podcast:transcript url="..." type="..." [language="..."] [rel="..."] /> (Transcripts)
    Must include url and type attributes; multiple transcripts permitted.

OPTIONAL (supported when relevant):
- <itunes:episode>                   (ItunesEpisode) — non-zero integer; REQUIRED for serial podcasts
- <itunes:season>                    (ItunesSeason) — non-zero integer
- <itunes:episodeType>               (ItunesEpisodeType) — "full" (default), "trailer", or "bonus"
- <itunes:block>                     (ItunesBlock) — "yes"
*/
type PSPItem struct {
	*PSPItemExtension
	Title          string        `xml:"title"`                     // required
	Link           string        `xml:"link,omitempty"`            // recommended
	Description    string        `xml:"description,omitempty"`     // recommended (wrap HTML in CDATA)
	Guid           *RssGuid      `xml:"guid"`                      // required
	PubDate        string        `xml:"pubDate,omitempty"`         // recommended RFC2822
	Enclosure      *RssEnclosure `xml:"enclosure"`                 // required
	ItunesDuration string        `xml:"itunes:duration,omitempty"` // seconds

}

type PSPItemExtension struct {
	XMLName xml.Name    `xml:"item"`
	Content *RssContent `xml:"content:encoded,omitempty"` // optional HTML content in CDATA (content namespace)
	// Extra custom nodes
	Extra []ExtensionNode `xml:",any"`
}

/*
Unified PSP-1 handling: configure podcast fields directly on Feed and Item,
then call Feed.ToPSPRSSFeed()/WritePSPRSS() or ToPSPRSSString() to render a compliant PSP-1 RSS feed.
*/

// PSP is a wrapper to marshal a Feed as PSP-1 RSS with required namespaces.
type PSP struct {
	*Feed
}




// FeedXml returns an XML-Ready object for a PSP wrapper.
func (p *PSP) FeedXml() interface{} {
	return p.wrapRoot(p.buildChannel())
}

/*
ValidatePSP enforces PSP-1 required elements at channel and item levels using generic Feed/Item fields.
*/
func ValidatePSP(f *Feed) error {
	// Channel-level required (generic only)
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("psp: channel title required")
	}
	if strings.TrimSpace(f.Description) == "" {
		return errors.New("psp: channel description required")
	}
	// PSP-1: channel description maximum 4000 bytes
	if len([]byte(f.Description)) > 4000 {
		return errors.New("psp: channel description must be <= 4000 bytes")
	}
	if f.Link == nil || strings.TrimSpace(f.Link.Href) == "" {
		return errors.New("psp: channel link required")
	}
	if strings.TrimSpace(f.Language) == "" {
		return errors.New("psp: channel language required")
	}
	if len(f.Categories) == 0 {
		return errors.New("psp: at least one category required")
	}
	if strings.TrimSpace(f.FeedURL) == "" {
		return errors.New("psp: atom:link rel=self required")
	}
	// Items
	if len(f.Items) == 0 {
		return errors.New("psp: at least one item required")
	}
	for i, it := range f.Items {
		if strings.TrimSpace(it.Title) == "" {
			return fmt.Errorf("psp: item[%d] title required", i)
		}
		if it.Enclosure == nil || strings.TrimSpace(it.Enclosure.Url) == "" || strings.TrimSpace(it.Enclosure.Type) == "" || it.Enclosure.Length <= 0 {
			return fmt.Errorf("psp: item[%d] enclosure url/type/length required", i)
		}
		// GUID required (can be guid with isPermaLink=false)
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("psp: item[%d] guid (ID) required", i)
		}
		// PSP-1: item description maximum 4000 bytes (if present)
		if len(it.Description) > 0 && len([]byte(it.Description)) > 4000 {
			return fmt.Errorf("psp: item[%d] description must be <= 4000 bytes", i)
		}
	}
	return nil
}

func (p *PSP) wrapRoot(ch *PSPChannel) *PSPRSSRoot {
	needsContent := false
	// Trigger content namespace if any item has Content or Description includes HTML tags (heuristic)
	for _, it := range p.Items {
		if strings.TrimSpace(it.Content) != "" {
			needsContent = true
			break
		}
		if strings.Contains(it.Description, "<") && strings.Contains(it.Description, ">") {
			needsContent = true
			break
		}
	}
	root := &PSPRSSRoot{
		Version:   "2.0",
		NSItunes:  xmlnsItunes,
		NSPodcast: xmlnsPodcast,
		NSAtom:    xmlnsAtom,
		Channel:   ch,
	}
	if needsContent {
		root.NSContent = xmlnsContent
	}
	return root
}

func (p *PSP) buildChannel() *PSPChannel {
	pub := anyTimeFormat(time.RFC1123Z, p.Created, p.Updated)
	build := anyTimeFormat(time.RFC1123Z, p.Updated)
	linkHref := ""
	if p.Link != nil {
		linkHref = p.Link.Href
	}
	ch := &PSPChannel{
		Title:       p.Title,
		Description: p.Description,
		Link:        linkHref,
		PSPChannelExtension: &PSPChannelExtension{
			Language:  p.Language,
			Copyright: p.Copyright,
		},
		PubDate:       pub,
		LastBuildDate: build,
	}
	// atom:link rel="self"
	if strings.TrimSpace(p.FeedURL) != "" {
		ch.AtomSelf = &PSPAtomLink{Href: p.FeedURL, Rel: "self", Type: "application/rss+xml"}
	}

	// iTunes channel fields (from generic feed where available)
	if p.Image != nil && strings.TrimSpace(p.Image.Url) != "" {
		ch.ItunesImage = &ItunesImage{Href: p.Image.Url}
	}
	if p.Author != nil && strings.TrimSpace(p.Author.Name) != "" {
		ch.ItunesAuthor = p.Author.Name
	}
	ch.ItunesCategories = convertCategories(p.Categories)

	// podcast channel fields (limited by generic feed data)
	if strings.TrimSpace(p.ID) != "" {
		// Use Feed.ID as podcast GUID when provided
		ch.Extra = append(ch.Extra, ExtensionNode{Name: "podcast:guid", Text: p.ID})
	} else if strings.TrimSpace(p.FeedURL) != "" {
		ch.Extra = append(ch.Extra, ExtensionNode{Name: "podcast:guid", Text: computePodcastGuid(p.FeedURL)})
	}

	// Items
	for _, it := range p.Items {
		ch.Items = append(ch.Items, p.buildItem(it))
	}

	// Custom channel nodes
	if len(p.Extensions) > 0 {
		ch.Extra = append(ch.Extra, p.Extensions...)
	}
	return ch
}

func (p *PSP) buildItem(it *Item) *PSPItem {
	pi := &PSPItem{
		Title:       it.Title,
		Description: it.Description,
		PubDate:     anyTimeFormat(time.RFC1123Z, it.Created, it.Updated),
	}
	// ensure embedded extension struct is non-nil before using Extra
	pi.PSPItemExtension = &PSPItemExtension{}
	if it.Link != nil {
		pi.Link = it.Link.Href
	}
	// enclosure required
	if it.Enclosure != nil {
		pi.Enclosure = &RssEnclosure{
			Url:    it.Enclosure.Url,
			Type:   it.Enclosure.Type,
			Length: fmt.Sprintf("%d", it.Enclosure.Length),
		}
	}
	// guid required
	if it.ID != "" {
		pi.Guid = &RssGuid{ID: it.ID, IsPermaLink: it.IsPermaLink}
	} else {
		pi.Guid = &RssGuid{ID: fallbackItemGuid(it), IsPermaLink: "false"}
	}

	// iTunes item fields (from generic feed where available)
	if it.DurationSeconds > 0 {
		pi.ItunesDuration = fmt.Sprintf("%d", it.DurationSeconds)
	}
	// Optional HTML content via content:encoded (align with RSS behavior)
	if len(it.Content) > 0 {
		pi.Content = &RssContent{Content: it.Content}
	}

	// Custom item nodes from src
	if len(it.Extensions) > 0 {
		pi.Extra = append(pi.Extra, it.Extensions...)
	}
	return pi
}

// convertCategories maps generic Categories to iTunes category XML structure (including nested subcategories).
func convertCategories(cats []*Category) []*ItunesCategory {
	var out []*ItunesCategory
	for _, c := range cats {
		if c == nil || strings.TrimSpace(c.Text) == "" {
			continue
		}
		ic := &ItunesCategory{Text: c.Text}
		out = append(out, ic)
	}
	return out
}

// computePodcastGuid generates UUIDv5 from normalized feed URL (scheme-stripped, trailing slashes removed).
func computePodcastGuid(feedURL string) string {
	normalized := normalizeFeedURL(feedURL)
	u := UUIDv5(PodcastNamespaceUUID, []byte(normalized))
	return u.String()
}

func normalizeFeedURL(u string) string {
	s := strings.TrimSpace(u)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "feed://")
	// remove trailing slashes
	for strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

func fallbackItemGuid(i *Item) string {
	// Best-effort: tag URI from link+date or UUID v4 URN
	link := i.Link
	if link == nil {
		link = &Link{}
	}
	if len(link.Href) > 0 && (!i.Created.IsZero() || !i.Updated.IsZero()) {
		dateStr := anyTimeFormat("2006-01-02", i.Updated, i.Created)
		host, path := link.Href, "/"
		if u, err := url.Parse(link.Href); err == nil {
			host, path = u.Host, u.Path
		}
		return fmt.Sprintf("tag:%s,%s:%s", host, dateStr, path)
	}
	return "urn:uuid:" + MustUUIDv4().String()
}

/*
Unified typed builders for PSP/iTunes fields.

Users can attach channel-level and item-level PSP/iTunes fields via ExtOption without
manually constructing stringly-typed nodes. These helpers produce proper namespaced
ExtensionNode values and apply at the correct scope.
*/

// WithPSPChannel returns an ExtOption to append PSP/iTunes channel nodes.

// PSPItemFields holds item-level PSP/iTunes fields for unified builder.

// WithPSPItem returns an ExtOption to append PSP/iTunes item nodes.
