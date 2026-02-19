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

// MarshalXML customizes channel XML to avoid emitting untagged struct fields and to include extension nodes.
func (ch *PSPChannel) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Ensure we start with <channel> element
	if start.Name.Local == "" {
		start.Name.Local = "channel"
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if err := ch.encodeLanguage(e); err != nil {
		return err
	}
	if err := ch.encodeAtomSelf(e); err != nil {
		return err
	}
	if err := ch.encodeCoreText(e); err != nil {
		return err
	}
	if err := ch.encodeDates(e); err != nil {
		return err
	}
	if err := ch.encodeItunesAuthor(e); err != nil {
		return err
	}
	if err := ch.encodeItunesExplicit(e); err != nil {
		return err
	}
	if err := ch.encodeItunesType(e); err != nil {
		return err
	}
	if err := ch.encodeItunesComplete(e); err != nil {
		return err
	}
	if err := ch.encodePodcastLocked(e); err != nil {
		return err
	}
	if err := ch.encodePodcastTXT(e); err != nil {
		return err
	}
	if err := ch.encodePodcastFunding(e); err != nil {
		return err
	}
	if err := ch.encodeItems(e); err != nil {
		return err
	}
	if err := ch.encodeItunesImage(e); err != nil {
		return err
	}
	if err := ch.encodeItunesCategories(e); err != nil {
		return err
	}
	if err := ch.encodeExtensions(e); err != nil {
		return err
	}

	// Close <channel>
	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// Internal helpers to reduce cyclomatic complexity of MarshalXML.

func (ch *PSPChannel) encodeTextIfSet(e *xml.Encoder, name, value string) error {
	if s := strings.TrimSpace(value); s != "" {
		return e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: name}})
	}
	return nil
}

func (ch *PSPChannel) encodeLanguage(e *xml.Encoder) error {
	return ch.encodeTextIfSet(e, "language", ch.Language)
}

func (ch *PSPChannel) encodeAtomSelf(e *xml.Encoder) error {
	if ch.AtomSelf != nil {
		return e.Encode(ch.AtomSelf)
	}
	return nil
}

func (ch *PSPChannel) encodeCoreText(e *xml.Encoder) error {
	if err := ch.encodeTextIfSet(e, "title", ch.Title); err != nil {
		return err
	}
	if err := ch.encodeTextIfSet(e, "link", ch.Link); err != nil {
		return err
	}
	return ch.encodeTextIfSet(e, "description", ch.Description)
}

func (ch *PSPChannel) encodeDates(e *xml.Encoder) error {
	if err := ch.encodeTextIfSet(e, "pubDate", ch.PubDate); err != nil {
		return err
	}
	return ch.encodeTextIfSet(e, "lastBuildDate", ch.LastBuildDate)
}

func (ch *PSPChannel) encodeItunesAuthor(e *xml.Encoder) error {
	return ch.encodeTextIfSet(e, "itunes:author", ch.ItunesAuthor)
}

func (ch *PSPChannel) encodeItunesExplicit(e *xml.Encoder) error {
	if ch.ItunesExplicit == nil {
		return nil
	}
	val := "false"
	if *ch.ItunesExplicit {
		val = "true"
	}
	return e.EncodeElement(val, xml.StartElement{Name: xml.Name{Local: "itunes:explicit"}})
}

func (ch *PSPChannel) encodeItunesType(e *xml.Encoder) error {
	return ch.encodeTextIfSet(e, "itunes:type", ch.ItunesType)
}

func (ch *PSPChannel) encodeItunesComplete(e *xml.Encoder) error {
	if ch.ItunesComplete {
		return e.EncodeElement("yes", xml.StartElement{Name: xml.Name{Local: "itunes:complete"}})
	}
	return nil
}

func (ch *PSPChannel) encodePodcastLocked(e *xml.Encoder) error {
	if ch.PodcastLocked == nil {
		return nil
	}
	val := "no"
	if *ch.PodcastLocked {
		val = "yes"
	}
	return e.EncodeElement(val, xml.StartElement{Name: xml.Name{Local: "podcast:locked"}})
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
	Title          string        `xml:"title"`                     // required
	Link           string        `xml:"link,omitempty"`            // recommended
	Description    string        `xml:"description,omitempty"`     // recommended (wrap HTML in CDATA)
	Guid           *RssGuid      `xml:"guid"`                      // required
	PubDate        string        `xml:"pubDate,omitempty"`         // recommended RFC2822
	Enclosure      *RssEnclosure `xml:"enclosure"`                 // required
	ItunesDuration string        `xml:"itunes:duration,omitempty"` // seconds

	XMLName xml.Name    `xml:"item"`
	Content *RssContent `xml:"content:encoded,omitempty"` // optional HTML content in CDATA (content namespace)
	// Extra custom nodes
	Extra []ExtensionNode `xml:",any"`
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

type channelExtHandler func(*PSPChannel, ExtensionNode) bool

func mapChannelExtensions(exts []ExtensionNode, ch *PSPChannel) {
	if len(exts) == 0 {
		return
	}
	handlers := map[string]channelExtHandler{
		"itunes:explicit": handleExtItunesExplicit,
		"itunes:type":     handleExtItunesType,
		"itunes:complete": handleExtItunesComplete,
		"itunes:image":    handleExtItunesImage,
		"podcast:locked":  handleExtPodcastLocked,
		"podcast:txt":     handleExtPodcastTXT,
		"podcast:funding": handleExtPodcastFunding,
	}
	var extras []ExtensionNode
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		if h, ok := handlers[name]; ok {
			if h(ch, n) {
				continue
			}
		}
		extras = append(extras, n)
	}
	if len(extras) > 0 {
		ch.Extra = append(ch.Extra, extras...)
	}
}

func handleExtItunesExplicit(ch *PSPChannel, n ExtensionNode) bool {
	t := strings.ToLower(strings.TrimSpace(n.Text))
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
	t := strings.ToLower(strings.TrimSpace(n.Text))
	if t == "episodic" || t == "serial" {
		ch.ItunesType = t
		return true
	}
	return false
}

func handleExtItunesComplete(ch *PSPChannel, n ExtensionNode) bool {
	if strings.EqualFold(strings.TrimSpace(n.Text), "yes") {
		ch.ItunesComplete = true
		return true
	}
	return false
}

func handleExtItunesImage(ch *PSPChannel, n ExtensionNode) bool {
	if n.Attrs != nil {
		if href, ok := n.Attrs["href"]; ok && strings.TrimSpace(href) != "" {
			ch.ItunesImageHref = strings.TrimSpace(href)
			return true
		}
	}
	return false
}

func handleExtPodcastLocked(ch *PSPChannel, n ExtensionNode) bool {
	t := strings.ToLower(strings.TrimSpace(n.Text))
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
		if purpose, ok := n.Attrs["purpose"]; ok {
			pt.Purpose = strings.TrimSpace(purpose)
		}
	}
	ch.PodcastTXT = pt
	return true
}

func handleExtPodcastFunding(ch *PSPChannel, n ExtensionNode) bool {
	var href string
	if n.Attrs != nil {
		href = strings.TrimSpace(n.Attrs["url"])
	}
	if href != "" || strings.TrimSpace(n.Text) != "" {
		ch.PodcastFunding = &PodcastFunding{Url: href, Text: n.Text}
		return true
	}
	return false
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
		pi.Guid = &RssGuid{ID: it.ID}
	} else {
		pi.Guid = &RssGuid{ID: fallbackItemGuid(it)}
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
