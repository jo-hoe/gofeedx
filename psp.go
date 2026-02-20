package gofeedx

// PSP-1: The Podcast RSS Standard encoder.
// Emits RSS 2.0 with required namespaces and enforces required PSP elements.
//
// see https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification
// and https://podcast-standard.org/podcast_standard/

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strconv"
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

// PSPChannel is the RSS channel with PSP/iTunes extensions.
type PSPChannel struct {
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

// ToPSP renders the feed to a PSP-1 compliant RSS string after validating ProfilePSP.
func ToPSP(feed *Feed) (string, error) {
	if feed == nil {
		return "", errors.New("nil feed")
	}
	return ToXML(&PSP{feed})
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

	// CDATA preference from extensions (default: enabled)
	use := UseCDATAFromExtensions(ch.Extra)

	// Run encoders in sequence to keep MarshalXML complexity low
	steps := []func(*xml.Encoder) error{
		func(enc *xml.Encoder) error { return ch.encodeLanguage(enc, use) },
		ch.encodeAtomSelf,
		func(enc *xml.Encoder) error { return ch.encodeCoreText(enc, use) },
		func(enc *xml.Encoder) error { return ch.encodeDates(enc, use) },
		func(enc *xml.Encoder) error { return ch.encodeItunesAuthor(enc, use) },
		ch.encodeItunesExplicit,
		func(enc *xml.Encoder) error { return ch.encodeItunesType(enc, use) },
		ch.encodeItunesComplete,
		ch.encodePodcastLocked,
		ch.encodePodcastTXT,
		ch.encodePodcastFunding,
		ch.encodeItems,
		ch.encodeItunesImage,
		ch.encodeItunesCategories,
		ch.encodeExtensions,
	}
	for _, step := range steps {
		if err := step(e); err != nil {
			return err
		}
	}

	// Close <channel>
	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// Internal helpers to reduce cyclomatic complexity of MarshalXML.

func (ch *PSPChannel) encodeTextIfSet(e *xml.Encoder, name, value string, use bool) error {
	if s := strings.TrimSpace(value); s != "" {
		return encodeElementCDATA(e, name, s, use)
	}
	return nil
}

func (ch *PSPChannel) encodeLanguage(e *xml.Encoder, use bool) error {
	return ch.encodeTextIfSet(e, "language", ch.Language, use)
}

func (ch *PSPChannel) encodeAtomSelf(e *xml.Encoder) error {
	if ch.AtomSelf != nil {
		return e.Encode(ch.AtomSelf)
	}
	return nil
}

func (ch *PSPChannel) encodeCoreText(e *xml.Encoder, use bool) error {
	if err := ch.encodeTextIfSet(e, "title", ch.Title, use); err != nil {
		return err
	}
	if err := ch.encodeTextIfSet(e, "link", ch.Link, use); err != nil {
		return err
	}
	return ch.encodeTextIfSet(e, "description", ch.Description, use)
}

func (ch *PSPChannel) encodeDates(e *xml.Encoder, use bool) error {
	if err := ch.encodeTextIfSet(e, "pubDate", ch.PubDate, use); err != nil {
		return err
	}
	return ch.encodeTextIfSet(e, "lastBuildDate", ch.LastBuildDate, use)
}

func (ch *PSPChannel) encodeItunesAuthor(e *xml.Encoder, use bool) error {
	return ch.encodeTextIfSet(e, "itunes:author", ch.ItunesAuthor, use)
}

func (ch *PSPChannel) encodeItunesExplicit(e *xml.Encoder) error {
	return encodeBoolElement(e, "itunes:explicit", ch.ItunesExplicit, "true", "false")
}

func (ch *PSPChannel) encodeItunesType(e *xml.Encoder, use bool) error {
	return ch.encodeTextIfSet(e, "itunes:type", ch.ItunesType, use)
}

func (ch *PSPChannel) encodeItunesComplete(e *xml.Encoder) error {
	return encodeFlagElement(e, "itunes:complete", ch.ItunesComplete, "yes")
}

func (ch *PSPChannel) encodePodcastLocked(e *xml.Encoder) error {
	return encodeBoolElement(e, "podcast:locked", ch.PodcastLocked, "yes", "no")
}

func (ch *PSPChannel) encodePodcastTXT(e *xml.Encoder) error {
	if ch.PodcastTXT != nil {
		return e.Encode(ch.PodcastTXT)
	}
	return nil
}

func (ch *PSPChannel) encodePodcastFunding(e *xml.Encoder) error {
	if ch.PodcastFunding != nil {
		return e.Encode(ch.PodcastFunding)
	}
	return nil
}

func (ch *PSPChannel) encodeItems(e *xml.Encoder) error {
	for _, it := range ch.Items {
		if it == nil {
			continue
		}
		if err := e.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

func (ch *PSPChannel) encodeItunesImage(e *xml.Encoder) error {
	if s := strings.TrimSpace(ch.ItunesImageHref); s != "" {
		return e.Encode(&ItunesImage{Href: s})
	}
	if ch.ItunesImage != nil && strings.TrimSpace(ch.ItunesImage.Href) != "" {
		return e.Encode(ch.ItunesImage)
	}
	return nil
}

func (ch *PSPChannel) encodeItunesCategories(e *xml.Encoder) error {
	for _, c := range ch.ItunesCategories {
		if c == nil || strings.TrimSpace(c.Text) == "" {
			continue
		}
		if err := e.Encode(c); err != nil {
			return err
		}
	}
	return nil
}

func (ch *PSPChannel) encodeExtensions(e *xml.Encoder) error {
	for _, n := range ch.Extra {
		if err := e.Encode(n); err != nil {
			return err
		}
	}
	return nil
}

/*
Generic helpers to apply DRY across PSP encoding and extension handling.
*/
func textLowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func attrTrim(attrs map[string]string, key string) string {
	if attrs == nil {
		return ""
	}
	return strings.TrimSpace(attrs[key])
}

func encodeBoolElement(e *xml.Encoder, name string, val *bool, trueStr, falseStr string) error {
	if val == nil {
		return nil
	}
	v := falseStr
	if *val {
		v = trueStr
	}
	return e.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: name}})
}

func encodeFlagElement(e *xml.Encoder, name string, flag bool, value string) error {
	if !flag {
		return nil
	}
	return e.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: name}})
}

func encodeStringIfSet(e *xml.Encoder, name, value string) error {
	if s := strings.TrimSpace(value); s != "" {
		return e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: name}})
	}
	return nil
}

func encodePositiveIntIfSet(e *xml.Encoder, name string, v int) error {
	if v > 0 {
		return e.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: name}})
	}
	return nil
}

func isYes(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "yes")
}

func processExtensions(exts []ExtensionNode, handlers map[string]func(ExtensionNode) bool) (extras []ExtensionNode) {
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		if h, ok := handlers[name]; ok {
			if h(n) {
				continue
			}
		}
		extras = append(extras, n)
	}
	return extras
}

func hasHTML(s string) bool {
	// Heuristic: contains any angle brackets
	return strings.Contains(s, "<") && strings.Contains(s, ">")
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
	Title             CData            `xml:"title"`                        // required
	Link              string           `xml:"link,omitempty"`               // recommended
	Description       CData            `xml:"description,omitempty"`        // recommended (wrap HTML in CDATA)
	Guid              *RssGuid         `xml:"guid"`                         // required
	PubDate           string           `xml:"pubDate,omitempty"`            // recommended RFC2822
	Enclosure         *RssEnclosure    `xml:"enclosure"`                    // required
	ItunesDuration    string           `xml:"itunes:duration,omitempty"`    // seconds
	ItunesImage       *ItunesImage     `xml:"itunes:image,omitempty"`       // item artwork
	ItunesExplicit    string           `xml:"itunes:explicit,omitempty"`    // "true" | "false"
	ItunesEpisode     int              `xml:"itunes:episode,omitempty"`     // > 0
	ItunesSeason      int              `xml:"itunes:season,omitempty"`      // > 0
	ItunesEpisodeType string           `xml:"itunes:episodeType,omitempty"` // "full" | "trailer" | "bonus"
	ItunesBlock       string           `xml:"itunes:block,omitempty"`       // "yes"
	Transcripts       []*PSPTranscript `xml:"podcast:transcript,omitempty"` // multiple allowed

	XMLName xml.Name    `xml:"item"`
	Content *RssContent `xml:"content:encoded,omitempty"` // optional HTML content in CDATA (content namespace)
	// Extra custom nodes
	Extra []ExtensionNode `xml:",any"`
}

// MarshalXML customizes PSP item encoding to emit CDATA based on extensions (default on).
func (it *PSPItem) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Force correct element name regardless of caller-provided start
	start.Name.Local = "item"
	use := UseCDATAFromExtensions(it.Extra)
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// Encode in small steps to keep cyclomatic complexity low
	steps := []func(*xml.Encoder, bool) error{
		func(enc *xml.Encoder, use bool) error { return it.encodeTitle(enc, use) },
		func(enc *xml.Encoder, use bool) error { return it.encodeLink(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeDescription(enc, use) },
		func(enc *xml.Encoder, use bool) error { return it.encodeGuid(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodePubDate(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeEnclosure(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeContent(enc, use) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesDuration(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesImage(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesExplicit(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesEpisode(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesSeason(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesEpisodeType(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeItunesBlock(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeTranscripts(enc) },
		func(enc *xml.Encoder, use bool) error { return it.encodeExtras(enc) },
	}
	for _, step := range steps {
		if err := step(e, use); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

func (it *PSPItem) encodeTitle(e *xml.Encoder, use bool) error {
	return encodeElementCDATA(e, "title", string(it.Title), use)
}

func (it *PSPItem) encodeLink(e *xml.Encoder) error {
	return encodeStringIfSet(e, "link", it.Link)
}

func (it *PSPItem) encodeDescription(e *xml.Encoder, use bool) error {
	return encodeElementCDATA(e, "description", string(it.Description), use)
}

func (it *PSPItem) encodeGuid(e *xml.Encoder) error {
	if it.Guid != nil {
		return e.Encode(it.Guid)
	}
	return nil
}

func (it *PSPItem) encodePubDate(e *xml.Encoder) error {
	return encodeStringIfSet(e, "pubDate", it.PubDate)
}

func (it *PSPItem) encodeEnclosure(e *xml.Encoder) error {
	if it.Enclosure != nil {
		return e.Encode(it.Enclosure)
	}
	return nil
}

func (it *PSPItem) encodeContent(e *xml.Encoder, use bool) error {
	if it.Content != nil && strings.TrimSpace(it.Content.Content) != "" {
		return encodeElementCDATA(e, "content:encoded", it.Content.Content, use)
	}
	return nil
}

func (it *PSPItem) encodeItunesDuration(e *xml.Encoder) error {
	return encodeStringIfSet(e, "itunes:duration", it.ItunesDuration)
}

func (it *PSPItem) encodeItunesImage(e *xml.Encoder) error {
	if it.ItunesImage != nil {
		return e.Encode(it.ItunesImage)
	}
	return nil
}

func (it *PSPItem) encodeItunesExplicit(e *xml.Encoder) error {
	return encodeStringIfSet(e, "itunes:explicit", it.ItunesExplicit)
}

func (it *PSPItem) encodeItunesEpisode(e *xml.Encoder) error {
	return encodePositiveIntIfSet(e, "itunes:episode", it.ItunesEpisode)
}

func (it *PSPItem) encodeItunesSeason(e *xml.Encoder) error {
	return encodePositiveIntIfSet(e, "itunes:season", it.ItunesSeason)
}

func (it *PSPItem) encodeItunesEpisodeType(e *xml.Encoder) error {
	return encodeStringIfSet(e, "itunes:episodeType", it.ItunesEpisodeType)
}

func (it *PSPItem) encodeItunesBlock(e *xml.Encoder) error {
	return encodeStringIfSet(e, "itunes:block", it.ItunesBlock)
}

func (it *PSPItem) encodeTranscripts(e *xml.Encoder) error {
	for _, tr := range it.Transcripts {
		if tr == nil {
			continue
		}
		if err := e.Encode(tr); err != nil {
			return err
		}
	}
	return nil
}

func (it *PSPItem) encodeExtras(e *xml.Encoder) error {
	for _, n := range it.Extra {
		if err := e.Encode(n); err != nil {
			return err
		}
	}
	return nil
}

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
	if err := validatePSPChannel(f); err != nil {
		return err
	}
	return validatePSPItems(f)
}

func validatePSPChannel(f *Feed) error {
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
	return nil
}

func validatePSPItems(f *Feed) error {
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
		if hasHTML(it.Description) {
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
	ch := deriveBasicChannel(p)
	addAtomSelf(p, ch)
	addItunesChannelFields(p, ch)
	addPodcastGUID(p, ch)
	addItems(p, ch)
	mapChannelExtensions(p.Extensions, ch)
	return ch
}

// Helpers to reduce cyclomatic complexity of buildChannel.

func deriveBasicChannel(p *PSP) *PSPChannel {
	pub := anyTimeFormat(time.RFC1123Z, p.Created, p.Updated)
	build := anyTimeFormat(time.RFC1123Z, p.Updated)
	linkHref := ""
	if p.Link != nil {
		linkHref = p.Link.Href
	}
	return &PSPChannel{
		Title:         p.Title,
		Description:   p.Description,
		Link:          linkHref,
		Language:      p.Language,
		Copyright:     p.Copyright,
		PubDate:       pub,
		LastBuildDate: build,
	}
}

func addAtomSelf(p *PSP, ch *PSPChannel) {
	if strings.TrimSpace(p.FeedURL) != "" {
		ch.AtomSelf = &PSPAtomLink{Href: p.FeedURL, Rel: "self", Type: "application/rss+xml"}
	}
}

func addItunesChannelFields(p *PSP, ch *PSPChannel) {
	if p.Image != nil && strings.TrimSpace(p.Image.Url) != "" {
		ch.ItunesImage = &ItunesImage{Href: p.Image.Url}
	}
	if p.Author != nil && strings.TrimSpace(p.Author.Name) != "" {
		ch.ItunesAuthor = p.Author.Name
	}
	ch.ItunesCategories = convertCategories(p.Categories)
}

func addPodcastGUID(p *PSP, ch *PSPChannel) {
	if strings.TrimSpace(p.ID) != "" {
		// Use Feed.ID as podcast GUID when provided
		ch.Extra = append(ch.Extra, ExtensionNode{Name: "podcast:guid", Text: p.ID})
	} else if strings.TrimSpace(p.FeedURL) != "" {
		ch.Extra = append(ch.Extra, ExtensionNode{Name: "podcast:guid", Text: computePodcastGuid(p.FeedURL)})
	}
}

func addItems(p *PSP, ch *PSPChannel) {
	for _, it := range p.Items {
		ch.Items = append(ch.Items, p.buildItem(it))
	}
}

func mapChannelExtensions(exts []ExtensionNode, ch *PSPChannel) {
	if len(exts) == 0 {
		return
	}
	handlers := map[string]func(ExtensionNode) bool{
		"itunes:explicit": func(n ExtensionNode) bool { return handleExtItunesExplicit(ch, n) },
		"itunes:type":     func(n ExtensionNode) bool { return handleExtItunesType(ch, n) },
		"itunes:complete": func(n ExtensionNode) bool { return handleExtItunesComplete(ch, n) },
		"itunes:image":    func(n ExtensionNode) bool { return handleExtItunesImage(ch, n) },
		"podcast:locked":  func(n ExtensionNode) bool { return handleExtPodcastLocked(ch, n) },
		"podcast:txt":     func(n ExtensionNode) bool { return handleExtPodcastTXT(ch, n) },
		"podcast:funding": func(n ExtensionNode) bool { return handleExtPodcastFunding(ch, n) },
	}
	extras := processExtensions(exts, handlers)
	if len(extras) > 0 {
		ch.Extra = append(ch.Extra, extras...)
	}
}

func handleExtItunesExplicit(ch *PSPChannel, n ExtensionNode) bool {
	t := textLowerTrim(n.Text)
	switch t {
	case "true":
		v := true
		ch.ItunesExplicit = &v
		return true
	case "false":
		v := false
		ch.ItunesExplicit = &v
		return true
	default:
		return false
	}
}

func handleExtItunesType(ch *PSPChannel, n ExtensionNode) bool {
	t := textLowerTrim(n.Text)
	if t == "episodic" || t == "serial" {
		ch.ItunesType = t
		return true
	}
	return false
}

func handleExtItunesComplete(ch *PSPChannel, n ExtensionNode) bool {
	if isYes(n.Text) {
		ch.ItunesComplete = true
		return true
	}
	return false
}

func handleExtItunesImage(ch *PSPChannel, n ExtensionNode) bool {
	href := attrTrim(n.Attrs, "href")
	if href != "" {
		ch.ItunesImageHref = href
		return true
	}
	return false
}

func handleExtPodcastLocked(ch *PSPChannel, n ExtensionNode) bool {
	t := textLowerTrim(n.Text)
	if t == "yes" || t == "no" {
		v := t == "yes"
		ch.PodcastLocked = &v
		return true
	}
	return false
}

func handleExtPodcastTXT(ch *PSPChannel, n ExtensionNode) bool {
	val := strings.TrimSpace(n.Text)
	if val == "" {
		return false
	}
	pt := &PodcastTXT{Value: val}
	if n.Attrs != nil {
		pt.Purpose = attrTrim(n.Attrs, "purpose")
	}
	ch.PodcastTXT = pt
	return true
}

func handleExtPodcastFunding(ch *PSPChannel, n ExtensionNode) bool {
	href := attrTrim(n.Attrs, "url")
	if href != "" || strings.TrimSpace(n.Text) != "" {
		ch.PodcastFunding = &PodcastFunding{Url: href, Text: n.Text}
		return true
	}
	return false
}

// Item-level PSP/iTunes extension mapping

func mapItemExtensions(exts []ExtensionNode, it *PSPItem) (extras []ExtensionNode) {
	if len(exts) == 0 {
		return nil
	}
	handlers := map[string]func(ExtensionNode) bool{
		"itunes:explicit":    func(n ExtensionNode) bool { return itemHandleItunesExplicit(it, n) },
		"itunes:image":       func(n ExtensionNode) bool { return itemHandleItunesImage(it, n) },
		"itunes:episode":     func(n ExtensionNode) bool { return itemHandleItunesEpisode(it, n) },
		"itunes:season":      func(n ExtensionNode) bool { return itemHandleItunesSeason(it, n) },
		"itunes:episodetype": func(n ExtensionNode) bool { return itemHandleItunesEpisodeType(it, n) },
		"itunes:block":       func(n ExtensionNode) bool { return itemHandleItunesBlock(it, n) },
		"podcast:transcript": func(n ExtensionNode) bool { return itemHandlePodcastTranscript(it, n) },
	}
	return processExtensions(exts, handlers)
}

func itemHandleItunesExplicit(it *PSPItem, n ExtensionNode) bool {
	t := textLowerTrim(n.Text)
	if t == "true" || t == "false" {
		it.ItunesExplicit = t
		return true
	}
	return false
}

func itemHandleItunesImage(it *PSPItem, n ExtensionNode) bool {
	href := attrTrim(n.Attrs, "href")
	if href != "" {
		it.ItunesImage = &ItunesImage{Href: href}
		return true
	}
	return false
}

func itemHandleItunesEpisode(it *PSPItem, n ExtensionNode) bool {
	if v, ok := parsePositiveInt(n.Text); ok {
		it.ItunesEpisode = v
		return true
	}
	return false
}

func itemHandleItunesSeason(it *PSPItem, n ExtensionNode) bool {
	if v, ok := parsePositiveInt(n.Text); ok {
		it.ItunesSeason = v
		return true
	}
	return false
}

func itemHandleItunesEpisodeType(it *PSPItem, n ExtensionNode) bool {
	t := textLowerTrim(n.Text)
	switch t {
	case "full", "trailer", "bonus":
		it.ItunesEpisodeType = t
		return true
	default:
		return false
	}
}

func itemHandleItunesBlock(it *PSPItem, n ExtensionNode) bool {
	if isYes(n.Text) {
		it.ItunesBlock = "yes"
		return true
	}
	return false
}

func itemHandlePodcastTranscript(it *PSPItem, n ExtensionNode) bool {
	url := attrTrim(n.Attrs, "url")
	typ := attrTrim(n.Attrs, "type")
	if url == "" || typ == "" {
		return false
	}
	tr := &PSPTranscript{
		Url:  url,
		Type: typ,
	}
	if s := attrTrim(n.Attrs, "language"); s != "" {
		tr.Language = s
	}
	if s := attrTrim(n.Attrs, "rel"); s != "" {
		tr.Rel = s
	}
	it.Transcripts = append(it.Transcripts, tr)
	return true
}

func (p *PSP) buildItem(it *Item) *PSPItem {
	pi := &PSPItem{
		Title:       CData(it.Title),
		Description: CData(it.Description),
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
	if strings.TrimSpace(it.ID) != "" {
		pi.Guid = &RssGuid{ID: it.ID, IsPermaLink: it.IsPermaLink}
	} else {
		pi.Guid = &RssGuid{ID: fallbackItemGuid(it), IsPermaLink: it.IsPermaLink}
	}

	// iTunes item fields (from generic feed where available)
	if it.DurationSeconds > 0 {
		pi.ItunesDuration = fmt.Sprintf("%d", it.DurationSeconds)
	}
	// Optional HTML content via content:encoded (align with RSS behavior)
	if len(it.Content) > 0 {
		pi.Content = &RssContent{Content: it.Content}
	}

	// Map PSP/iTunes item-level extensions into typed fields; keep unknown in Extra
	if len(it.Extensions) > 0 {
		extras := mapItemExtensions(it.Extensions, pi)
		if len(extras) > 0 {
			pi.Extra = append(pi.Extra, extras...)
		}
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

// PSP-specific builder helpers implemented here to avoid adding target-specific code
// to generic files (feed.go, builder.go). These methods wrap WithExtensions to emit
// the correct PSP/iTunes namespace elements when rendering PSP.
// Feed-level helpers:

// WithPSPExplicit sets itunes:explicit at channel scope ("true"/"false").
func (b *FeedBuilder) WithPSPExplicit(explicit bool) *FeedBuilder {
	text := "false"
	if explicit {
		text = "true"
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:explicit", Text: text})
}

// WithPSPFunding sets podcast:funding at channel scope with url attr and label text.
func (b *FeedBuilder) WithPSPFunding(url, label string) *FeedBuilder {
	url = strings.TrimSpace(url)
	label = strings.TrimSpace(label)
	if url == "" && label == "" {
		return b
	}
	attrs := map[string]string{}
	if url != "" {
		attrs["url"] = url
	}
	return b.WithExtensions(ExtensionNode{Name: "podcast:funding", Attrs: attrs, Text: label})
}

// WithPSPLocked sets podcast:locked ("yes"/"no") at channel scope.
func (b *FeedBuilder) WithPSPLocked(locked bool) *FeedBuilder {
	val := "no"
	if locked {
		val = "yes"
	}
	return b.WithExtensions(ExtensionNode{Name: "podcast:locked", Text: val})
}

// WithPSPTXT sets podcast:txt at channel scope with optional purpose attr.
func (b *FeedBuilder) WithPSPTXT(value, purpose string) *FeedBuilder {
	value = strings.TrimSpace(value)
	purpose = strings.TrimSpace(purpose)
	if value == "" {
		return b
	}
	attrs := map[string]string{}
	if purpose != "" {
		attrs["purpose"] = purpose
	}
	return b.WithExtensions(ExtensionNode{Name: "podcast:txt", Attrs: attrs, Text: value})
}

// WithPSPItunesType sets itunes:type ("episodic" or "serial") at channel scope.
func (b *FeedBuilder) WithPSPItunesType(t string) *FeedBuilder {
	t = strings.TrimSpace(strings.ToLower(t))
	switch t {
	case "episodic", "serial":
		return b.WithExtensions(ExtensionNode{Name: "itunes:type", Text: t})
	default:
		// ignore invalid types
		return b
	}
}

// WithPSPItunesComplete sets itunes:complete ("yes") at channel scope when complete is true.
func (b *FeedBuilder) WithPSPItunesComplete(complete bool) *FeedBuilder {
	if !complete {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:complete", Text: "yes"})
}

// WithPSPImageHref sets/overrides itunes:image@href at channel scope.
func (b *FeedBuilder) WithPSPImageHref(href string) *FeedBuilder {
	href = strings.TrimSpace(href)
	if href == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:image", Attrs: map[string]string{"href": href}})
}

// Item-level helpers:

// WithPSPExplicit sets itunes:explicit at item scope ("true"/"false").
func (b *ItemBuilder) WithPSPExplicit(explicit bool) *ItemBuilder {
	text := "false"
	if explicit {
		text = "true"
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:explicit", Text: text})
}

// WithPSPTranscript adds a podcast:transcript node at item scope.
func (b *ItemBuilder) WithPSPTranscript(url, typ, language, rel string) *ItemBuilder {
	url = strings.TrimSpace(url)
	typ = strings.TrimSpace(typ)
	if url == "" || typ == "" {
		return b
	}
	attrs := map[string]string{
		"url":  url,
		"type": typ,
	}
	if s := strings.TrimSpace(language); s != "" {
		attrs["language"] = s
	}
	if s := strings.TrimSpace(rel); s != "" {
		attrs["rel"] = s
	}
	return b.WithExtensions(ExtensionNode{Name: "podcast:transcript", Attrs: attrs})
}

// WithPSPImageHref sets/overrides itunes:image@href at item scope.
func (b *ItemBuilder) WithPSPImageHref(href string) *ItemBuilder {
	href = strings.TrimSpace(href)
	if href == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:image", Attrs: map[string]string{"href": href}})
}

// WithPSPEpisode sets itunes:episode (must be > 0) at item scope.
func (b *ItemBuilder) WithPSPEpisode(n int) *ItemBuilder {
	if n <= 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:episode", Text: strconv.Itoa(n)})
}

// WithPSPSeason sets itunes:season (must be > 0) at item scope.
func (b *ItemBuilder) WithPSPSeason(n int) *ItemBuilder {
	if n <= 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:season", Text: strconv.Itoa(n)})
}

// WithPSPEpisodeType sets itunes:episodeType ("full" | "trailer" | "bonus") at item scope.
func (b *ItemBuilder) WithPSPEpisodeType(t string) *ItemBuilder {
	t = strings.TrimSpace(strings.ToLower(t))
	switch t {
	case "full", "trailer", "bonus":
		return b.WithExtensions(ExtensionNode{Name: "itunes:episodeType", Text: t})
	default:
		return b
	}
}

// WithPSPBlock sets itunes:block ("yes") at item scope when true.
func (b *ItemBuilder) WithPSPBlock(block bool) *ItemBuilder {
	if !block {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "itunes:block", Text: "yes"})
}
