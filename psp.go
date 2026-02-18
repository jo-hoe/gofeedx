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

// PSPFeed is a builder/encoder for PSP-compliant RSS.
type PSPFeed struct {
	src *Feed

	// PSP channel config
	atomSelf         string
	itunesImage      string
	itunesExplicit   *bool
	itunesAuthor     string
	itunesType       string // episodic/serial
	itunesComplete   bool
	itunesCategories []*ItunesCategory

	podcastLocked   *bool  // yes/no
	podcastGuidSeed string // the feed URL to seed GUID generation
	podcastTXT      *PodcastTXT
	podcastFunding  *PodcastFunding

	// extra namespaces (prefix->url) if needed
	extraNS map[string]string

	// per-item overrides keyed by index
	itemExt map[int]*PSPItemConfig
}

// PSPItemConfig holds PSP/iTunes extras per item.
type PSPItemConfig struct {
	ItunesDurationSeconds int
	ItunesImageHref       string
	ItunesExplicit        *bool
	ItunesEpisode         *int
	ItunesSeason          *int
	ItunesEpisodeType     string // full/trailer/bonus
	ItunesBlock           bool

	Transcripts []PSPTranscript
}

// NewPSPFeed creates a builder from a generic Feed.
func NewPSPFeed(feed *Feed) *PSPFeed {
	return &PSPFeed{
		src:     feed,
		extraNS: map[string]string{},
		itemExt: map[int]*PSPItemConfig{},
	}
}

// WithLanguage sets channel language (ISO 639-1/2 code).
func (p *PSPFeed) WithLanguage(lang string) *PSPFeed {
	p.src.Language = lang
	return p
}

// WithAtomSelf sets the atom:link rel="self".
func (p *PSPFeed) WithAtomSelf(feedURL string) *PSPFeed {
	p.atomSelf = feedURL
	return p
}

// WithItunesExplicit sets channel explicit flag.
func (p *PSPFeed) WithItunesExplicit(explicit bool) *PSPFeed {
	p.itunesExplicit = &explicit
	return p
}

// WithItunesAuthor sets channel iTunes author string.
func (p *PSPFeed) WithItunesAuthor(author string) *PSPFeed {
	p.itunesAuthor = author
	return p
}

// WithItunesImage sets channel artwork URL.
func (p *PSPFeed) WithItunesImage(href string) *PSPFeed {
	p.itunesImage = href
	return p
}

// WithItunesType sets episodic/serial.
func (p *PSPFeed) WithItunesType(t string) *PSPFeed {
	p.itunesType = strings.ToLower(t)
	return p
}

// WithItunesComplete marks the podcast as complete (no more episodes).
func (p *PSPFeed) WithItunesComplete(complete bool) *PSPFeed {
	p.itunesComplete = complete
	return p
}

// WithItunesCategory adds a top-level category with optional subcategories.
func (p *PSPFeed) WithItunesCategory(parent string, subs ...string) *PSPFeed {
	c := &ItunesCategory{Text: parent}
	for _, s := range subs {
		c.Sub = append(c.Sub, &ItunesCategory{Text: s})
	}
	p.itunesCategories = append(p.itunesCategories, c)
	return p
}

// WithPodcastLocked sets podcast:locked yes/no.
func (p *PSPFeed) WithPodcastLocked(locked bool) *PSPFeed {
	p.podcastLocked = &locked
	return p
}

// WithPodcastGuidFromURL sets the feed URL to seed podcast:guid (UUIDv5).
func (p *PSPFeed) WithPodcastGuidFromURL(feedURL string) *PSPFeed {
	p.podcastGuidSeed = feedURL
	return p
}

// WithPodcastTXT sets podcast:txt value and optional purpose.
func (p *PSPFeed) WithPodcastTXT(value string, purpose string) *PSPFeed {
	p.podcastTXT = &PodcastTXT{Value: value, Purpose: strings.TrimSpace(purpose)}
	return p
}

// WithPodcastFunding sets podcast:funding url and text label.
func (p *PSPFeed) WithPodcastFunding(url, text string) *PSPFeed {
	p.podcastFunding = &PodcastFunding{Url: url, Text: text}
	return p
}

// WithExtraNS adds an additional xmlns declaration at root (prefix -> uri).
func (p *PSPFeed) WithExtraNS(prefix, uri string) *PSPFeed {
	if p.extraNS == nil {
		p.extraNS = map[string]string{}
	}
	p.extraNS[prefix] = uri
	return p
}

// ConfigureItem sets PSP/iTunes item extras for the Nth item.
func (p *PSPFeed) ConfigureItem(index int, cfg PSPItemConfig) *PSPFeed {
	cp := cfg // copy
	p.itemExt[index] = &cp
	return p
}

// Validate enforces PSP-1 required elements at channel and item levels.
func (p *PSPFeed) Validate() error {
	if p.src == nil {
		return errors.New("psp: source feed is nil")
	}
	// Channel-level required
	if strings.TrimSpace(p.src.Title) == "" {
		return errors.New("psp: channel title required")
	}
	if strings.TrimSpace(p.src.Description) == "" {
		return errors.New("psp: channel description required")
	}
	// PSP-1: channel description maximum 4000 bytes
	if len([]byte(p.src.Description)) > 4000 {
		return errors.New("psp: channel description must be <= 4000 bytes")
	}
	if p.src.Link == nil || strings.TrimSpace(p.src.Link.Href) == "" {
		return errors.New("psp: channel link required")
	}
	if strings.TrimSpace(p.src.Language) == "" {
		return errors.New("psp: channel language required")
	}
	if len(p.itunesCategories) == 0 {
		return errors.New("psp: at least one itunes:category required")
	}
	if p.itunesExplicit == nil {
		return errors.New("psp: itunes:explicit required")
	}
	if strings.TrimSpace(p.itunesImage) == "" {
		return errors.New("psp: itunes:image (href) required")
	}
	if strings.TrimSpace(p.atomSelf) == "" {
		return errors.New("psp: atom:link rel=self required")
	}
	// Items
	if len(p.src.Items) == 0 {
		return errors.New("psp: at least one item required")
	}
	for i, it := range p.src.Items {
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
		if cfg := p.itemExt[i]; cfg != nil {
			for _, tr := range cfg.Transcripts {
				if strings.TrimSpace(tr.Url) == "" || strings.TrimSpace(tr.Type) == "" {
					return fmt.Errorf("psp: item[%d] podcast:transcript requires url and type", i)
				}
			}
		}
	}
	// Conditional requirements per PSP-1:
	// For serial podcasts, each episode MUST include an itunes:episode number (non-zero integer).
	if strings.ToLower(p.itunesType) == "serial" {
		for i := range p.src.Items {
			epNum := 0
			if cfg := p.itemExt[i]; cfg != nil && cfg.ItunesEpisode != nil {
				epNum = *cfg.ItunesEpisode
			}
			if epNum <= 0 {
				return fmt.Errorf("psp: serial podcasts require itunes:episode (non-zero) for item[%d]", i)
			}
		}
	}
	return nil
}

// ToPSPRSS returns the PSP-1 RSS XML string.
func (p *PSPFeed) ToPSPRSS() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	x := p.buildChannel()
	root := p.wrapRoot(x)
	data, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	// Compose header without trailing newline
	return xml.Header[:len(xml.Header)-1] + string(data), nil
}

// WritePSPRSS encodes the PSP-1 RSS XML to writer.
func (p *PSPFeed) WritePSPRSS(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	x := p.buildChannel()
	root := p.wrapRoot(x)
	if _, err := w.Write([]byte(xml.Header[:len(xml.Header)-1])); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(root)
}

func (p *PSPFeed) wrapRoot(ch *PSPChannel) *PSPRSSRoot {
	needsContent := false
	// Trigger content namespace if any item has Content or Description includes HTML tags (heuristic)
	for _, it := range p.src.Items {
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
	// extra namespaces
	// Note: encoding/xml cannot dynamically declare arbitrary prefixes except as attributes.
	// We attach them as raw attributes via XMLNode custom nodes if necessary, but here we add known ones only.
	return root
}

func (p *PSPFeed) buildChannel() *PSPChannel {
	pub := anyTimeFormat(time.RFC1123Z, p.src.Created, p.src.Updated)
	build := anyTimeFormat(time.RFC1123Z, p.src.Updated)
	linkHref := ""
	if p.src.Link != nil {
		linkHref = p.src.Link.Href
	}
	ch := &PSPChannel{
		Title:         p.src.Title,
		Description:   p.src.Description,
		Link:          linkHref,
		Language:      p.src.Language,
		Copyright:     p.src.Copyright,
		PubDate:       pub,
		LastBuildDate: build,
		AtomSelf:      &PSPAtomLink{Href: p.atomSelf, Rel: "self", Type: "application/rss+xml"},
	}

	// iTunes channel fields
	if p.itunesImage != "" {
		ch.ItunesImage = &ItunesImage{Href: p.itunesImage}
	}
	if p.itunesExplicit != nil {
		ch.ItunesExplicit = &ItunesExplicit{Value: boolToTrueFalse(*p.itunesExplicit)}
	}
	if p.itunesAuthor != "" {
		ch.ItunesAuthor = p.itunesAuthor
	}
	if p.itunesType == "serial" || p.itunesType == "episodic" || p.itunesType == "" {
		// episodic is default; only emit when explicitly set to serial or episodic
		if p.itunesType != "" {
			ch.ItunesType = p.itunesType
		}
	} else {
		// ignore invalid values
	}
	if p.itunesComplete {
		ch.ItunesComplete = &ItunesComplete{Value: "yes"}
	}
	ch.ItunesCategories = append(ch.ItunesCategories, p.itunesCategories...)

	// podcast channel fields
	if p.podcastLocked != nil {
		if *p.podcastLocked {
			ch.PodcastLocked = &PodcastLocked{Value: "yes"}
		} else {
			ch.PodcastLocked = &PodcastLocked{Value: "no"}
		}
	}
	if p.podcastGuidSeed != "" {
		ch.PodcastGuid = computePodcastGuid(p.podcastGuidSeed)
	}
	if p.podcastTXT != nil {
		ch.PodcastTXT = p.podcastTXT
	}
	if p.podcastFunding != nil {
		ch.PodcastFunding = p.podcastFunding
	}

	// Items
	for idx, it := range p.src.Items {
		ch.Items = append(ch.Items, p.buildItem(idx, it))
	}

	// Custom channel nodes
	if len(p.src.Extensions) > 0 {
		ch.Extra = append(ch.Extra, p.src.Extensions...)
	}
	return ch
}

func (p *PSPFeed) buildItem(index int, it *Item) *PSPItem {
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
		// Should not happen if Validate passed
		pi.Guid = &RssGuid{ID: fallbackItemGuid(it), IsPermaLink: "false"}
	}

	// per-item extras
	if cfg := p.itemExt[index]; cfg != nil {
		if cfg.ItunesDurationSeconds > 0 {
			pi.ItunesDuration = fmt.Sprintf("%d", cfg.ItunesDurationSeconds)
		}
		if cfg.ItunesImageHref != "" {
			pi.ItunesImage = &ItunesImage{Href: cfg.ItunesImageHref}
		}
		if cfg.ItunesExplicit != nil {
			pi.ItunesExplicit = &ItunesExplicit{Value: boolToTrueFalse(*cfg.ItunesExplicit)}
		}
		if cfg.ItunesEpisode != nil {
			pi.ItunesEpisode = cfg.ItunesEpisode
		}
		if cfg.ItunesSeason != nil {
			pi.ItunesSeason = cfg.ItunesSeason
		}
		if cfg.ItunesEpisodeType != "" {
			switch strings.ToLower(cfg.ItunesEpisodeType) {
			case "full", "trailer", "bonus":
				pi.ItunesEpisodeType = strings.ToLower(cfg.ItunesEpisodeType)
			}
		}
		if cfg.ItunesBlock {
			pi.ItunesBlock = &ItunesBlock{Value: "yes"}
		}
		if len(cfg.Transcripts) > 0 {
			pi.Transcripts = append(pi.Transcripts, cfg.Transcripts...)
		}
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
