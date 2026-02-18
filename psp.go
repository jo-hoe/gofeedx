package gofeedx

// PSP-1: The Podcast RSS Standard encoder and builder
// Emits RSS 2.0 with required namespaces, enforces required PSP elements,
// and provides builder-style helpers.
//
// see https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification
// and https://podcast-standard.org/podcast_standard/

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	XMLName     xml.Name `xml:"channel"`
	Title       string   `xml:"title"`       // required
	Description string   `xml:"description"` // required (may embed CDATA in content:encoded for rich HTML elsewhere)
	Link        string   `xml:"link"`        // required
	Language    string   `xml:"language"`    // required

	// Recommended and optional standard RSS fields
	Copyright     string `xml:"copyright,omitempty"`
	PubDate       string `xml:"pubDate,omitempty"`
	LastBuildDate string `xml:"lastBuildDate,omitempty"`

	// atom:link rel="self"
	AtomSelf *PSPAtomLink `xml:"atom:link,omitempty"`

	// iTunes channel fields
	ItunesImage      *ItunesImage      `xml:"itunes:image,omitempty"`
	ItunesExplicit   *ItunesExplicit   `xml:"itunes:explicit,omitempty"`
	ItunesAuthor     string            `xml:"itunes:author,omitempty"`
	ItunesType       string            `xml:"itunes:type,omitempty"`
	ItunesComplete   *ItunesComplete   `xml:"itunes:complete,omitempty"`
	ItunesCategories []*ItunesCategory `xml:"itunes:category,omitempty"`

	// podcast namespace channel fields
	PodcastLocked  *PodcastLocked  `xml:"podcast:locked,omitempty"`
	PodcastGuid    string          `xml:"podcast:guid,omitempty"`
	PodcastTXT     *PodcastTXT     `xml:"podcast:txt,omitempty"`
	PodcastFunding *PodcastFunding `xml:"podcast:funding,omitempty"`

	// Items
	Items []*PSPItem `xml:"item"`

	// Custom channel nodes
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

// ItunesExplicit emits "true"/"false"
type ItunesExplicit struct {
	XMLName xml.Name `xml:"itunes:explicit"`
	Value   string   `xml:",chardata"` // "true" or "false"
}

// ItunesComplete emits "yes"
type ItunesComplete struct {
	XMLName xml.Name `xml:"itunes:complete"`
	Value   string   `xml:",chardata"` // "yes"
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
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`                 // required
	Link        string   `xml:"link,omitempty"`        // recommended
	Description string   `xml:"description,omitempty"` // recommended (wrap HTML in CDATA)
	PubDate     string   `xml:"pubDate,omitempty"`     // recommended RFC2822

	// RSS 2.0 enclosure
	Enclosure *RssEnclosure `xml:"enclosure"` // required

	// RSS 2.0 guid
	Guid *RssGuid `xml:"guid"` // required

	// iTunes item fields
	ItunesDuration    string          `xml:"itunes:duration,omitempty"` // seconds
	ItunesImage       *ItunesImage    `xml:"itunes:image,omitempty"`
	ItunesExplicit    *ItunesExplicit `xml:"itunes:explicit,omitempty"`
	ItunesEpisode     *int            `xml:"itunes:episode,omitempty"`
	ItunesSeason      *int            `xml:"itunes:season,omitempty"`
	ItunesEpisodeType string          `xml:"itunes:episodeType,omitempty"` // "full", "trailer", "bonus"
	ItunesBlock       *ItunesBlock    `xml:"itunes:block,omitempty"`

	// podcast namespace item fields
	Transcripts []PSPTranscript `xml:"podcast:transcript,omitempty"`

	// Extra custom nodes
	Extra []ExtensionNode `xml:",any"`
}

// ItunesBlock emits "yes"
type ItunesBlock struct {
	XMLName xml.Name `xml:"itunes:block"`
	Value   string   `xml:",chardata"` // "yes"
}

/*
Unified PSP-1 handling: configure podcast fields directly on Feed and Item,
then call Feed.ToPSPRSSFeed()/WritePSPRSS() or ToPSPRSSString() to render a compliant PSP-1 RSS feed.
*/

// PSP is a wrapper to marshal a Feed as PSP-1 RSS with required namespaces.
type PSP struct {
	*Feed
}

/*
ToPSPRSSString creates a PSP-1 RSS representation of this feed as a string.
Use ToPSPRSS() if you need the structured root object for further processing.
*/
func (f *Feed) ToPSPRSSString() (string, error) {
	if err := f.ValidatePSP(); err != nil {
		return "", err
	}
	return ToXML(&PSP{f})
}

/*
ToPSPRSSFeed returns the PSP-1 RSS root struct for this feed.
*/
func (f *Feed) ToPSPRSSFeed() (*PSPRSSRoot, error) {
	if err := f.ValidatePSP(); err != nil {
		return nil, err
	}
	p := &PSP{f}
	return p.wrapRoot(p.buildChannel()), nil
}


// WritePSPRSS writes a PSP-1 RSS representation of this feed to the writer.
func (f *Feed) WritePSPRSS(w io.Writer) error {
	if err := f.ValidatePSP(); err != nil {
		return err
	}
	return WriteXML(&PSP{f}, w)
}

// FeedXml returns an XML-Ready object for a PSP wrapper.
func (p *PSP) FeedXml() interface{} {
	return p.wrapRoot(p.buildChannel())
}

// ValidatePSP enforces PSP-1 required elements at channel and item levels using generic Feed/Item fields.
func (f *Feed) ValidatePSP() error {
	// Channel-level required
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
		return errors.New("psp: at least one itunes:category required")
	}
	if f.ItunesExplicit == nil {
		return errors.New("psp: itunes:explicit required")
	}
	if strings.TrimSpace(f.ItunesImageHref) == "" {
		return errors.New("psp: itunes:image (href) required")
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
		// PSP-1: podcast:transcript must include url and type if present
		for _, tr := range it.Transcripts {
			if strings.TrimSpace(tr.Url) == "" || strings.TrimSpace(tr.Type) == "" {
				return fmt.Errorf("psp: item[%d] podcast:transcript requires url and type", i)
			}
		}
	}
	// Conditional requirements per PSP-1:
	// For serial podcasts, each episode MUST include an itunes:episode number (non-zero integer).
	if strings.ToLower(strings.TrimSpace(f.ItunesType)) == "serial" {
		for i, it := range f.Items {
			epNum := 0
			if it.ItunesEpisode != nil {
				epNum = *it.ItunesEpisode
			}
			if epNum <= 0 {
				return fmt.Errorf("psp: serial podcasts require itunes:episode (non-zero) for item[%d]", i)
			}
		}
	}
	return nil
}

func (p *PSP) wrapRoot(ch *PSPChannel) *PSPRSSRoot {
	needsContent := false
	// Trigger content namespace if any item has Content or Description includes HTML tags (heuristic)
	for _, it := range p.Feed.Items {
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
	pub := anyTimeFormat(time.RFC1123Z, p.Feed.Created, p.Feed.Updated)
	build := anyTimeFormat(time.RFC1123Z, p.Feed.Updated)
	linkHref := ""
	if p.Feed.Link != nil {
		linkHref = p.Feed.Link.Href
	}
	ch := &PSPChannel{
		Title:         p.Feed.Title,
		Description:   p.Feed.Description,
		Link:          linkHref,
		Language:      p.Feed.Language,
		Copyright:     p.Feed.Copyright,
		PubDate:       pub,
		LastBuildDate: build,
	}
	// atom:link rel="self"
	if strings.TrimSpace(p.Feed.FeedURL) != "" {
		ch.AtomSelf = &PSPAtomLink{Href: p.Feed.FeedURL, Rel: "self", Type: "application/rss+xml"}
	}

	// iTunes channel fields
	if p.Feed.ItunesImageHref != "" {
		ch.ItunesImage = &ItunesImage{Href: p.Feed.ItunesImageHref}
	}
	if p.Feed.ItunesExplicit != nil {
		ch.ItunesExplicit = &ItunesExplicit{Value: boolToTrueFalse(*p.Feed.ItunesExplicit)}
	}
	if p.Feed.Author != nil && strings.TrimSpace(p.Feed.Author.Name) != "" {
		ch.ItunesAuthor = p.Feed.Author.Name
	}
	itype := strings.ToLower(strings.TrimSpace(p.Feed.ItunesType))
	if itype == "serial" || itype == "episodic" {
		ch.ItunesType = itype
	}
	if p.Feed.ItunesComplete {
		ch.ItunesComplete = &ItunesComplete{Value: "yes"}
	}
	ch.ItunesCategories = convertCategories(p.Feed.Categories)

	// podcast channel fields
	if p.Feed.PodcastLocked != nil {
		if *p.Feed.PodcastLocked {
			ch.PodcastLocked = &PodcastLocked{Value: "yes"}
		} else {
			ch.PodcastLocked = &PodcastLocked{Value: "no"}
		}
	}
	if strings.TrimSpace(p.Feed.ID) != "" {
		// Treat Feed.ID as the authoritative podcast GUID when provided
		ch.PodcastGuid = p.Feed.ID
	} else if strings.TrimSpace(p.Feed.PodcastGuidSeed) != "" {
		ch.PodcastGuid = computePodcastGuid(p.Feed.PodcastGuidSeed)
	} else if strings.TrimSpace(p.Feed.FeedURL) != "" {
		ch.PodcastGuid = computePodcastGuid(p.Feed.FeedURL)
	}
	if p.Feed.PodcastTXT != nil {
		ch.PodcastTXT = p.Feed.PodcastTXT
	}
	if p.Feed.PodcastFunding != nil {
		ch.PodcastFunding = p.Feed.PodcastFunding
	}

	// Items
	for _, it := range p.Feed.Items {
		ch.Items = append(ch.Items, p.buildItem(it))
	}

	// Custom channel nodes
	if len(p.Feed.Extensions) > 0 {
		ch.Extra = append(ch.Extra, p.Feed.Extensions...)
	}
	return ch
}

func (p *PSP) buildItem(it *Item) *PSPItem {
	pi := &PSPItem{
		Title:       it.Title,
		Description: it.Description,
		PubDate:     anyTimeFormat(time.RFC1123Z, it.Created, it.Updated),
	}
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

	// iTunes item fields
	if it.DurationSeconds > 0 {
		pi.ItunesDuration = fmt.Sprintf("%d", it.DurationSeconds)
	}
	if it.ItunesImageHref != "" {
		pi.ItunesImage = &ItunesImage{Href: it.ItunesImageHref}
	}
	if it.ItunesExplicit != nil {
		pi.ItunesExplicit = &ItunesExplicit{Value: boolToTrueFalse(*it.ItunesExplicit)}
	}
	if it.ItunesEpisode != nil {
		pi.ItunesEpisode = it.ItunesEpisode
	}
	if it.ItunesSeason != nil {
		pi.ItunesSeason = it.ItunesSeason
	}
	if v := strings.ToLower(strings.TrimSpace(it.ItunesEpisodeType)); v == "full" || v == "trailer" || v == "bonus" {
		pi.ItunesEpisodeType = v
	}
	if it.ItunesBlock {
		pi.ItunesBlock = &ItunesBlock{Value: "yes"}
	}

	// podcast transcripts
	if len(it.Transcripts) > 0 {
		pi.Transcripts = append(pi.Transcripts, it.Transcripts...)
	}

	// Custom item nodes from src
	if len(it.Extensions) > 0 {
		pi.Extra = append(pi.Extra, it.Extensions...)
	}
	return pi
}

func boolToTrueFalse(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// convertCategories maps generic Categories to iTunes category XML structure (including nested subcategories).
func convertCategories(cats []*Category) []*ItunesCategory {
	var out []*ItunesCategory
	for _, c := range cats {
		if c == nil || strings.TrimSpace(c.Text) == "" {
			continue
		}
		ic := &ItunesCategory{Text: c.Text}
		if len(c.Sub) > 0 {
			ic.Sub = convertCategories(c.Sub)
		}
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
