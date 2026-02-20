package gofeedx

// RSS 2.0 encoder (with optional content:encoded for HTML content)
import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RssFeedXml is the <rss> root wrapper.
type RssFeedXml struct {
	XMLName          xml.Name `xml:"rss"`
	Version          string   `xml:"version,attr"`
	ContentNamespace string   `xml:"xmlns:content,attr,omitempty"`
	Channel          *RssFeed `xml:"channel"`
}

 // RssContent holds HTML content for content:encoded.
 type RssContent struct {
 	XMLName xml.Name `xml:"content:encoded"`
 	Content string
 }

type RssImage struct {
	XMLName xml.Name `xml:"image"`
	Url     string   `xml:"url"`
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	Width   int      `xml:"width,omitempty"`
	Height  int      `xml:"height,omitempty"`
}

type RssEnclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Length  string   `xml:"length,attr"`
	Type    string   `xml:"type,attr"`
}

type RssGuid struct {
	XMLName     xml.Name `xml:"guid"`
	ID          string   `xml:",chardata"`
	IsPermaLink string   `xml:"isPermaLink,attr,omitempty"` // "true", "false", or omitted
}

type RssItem struct {
	Title       CData `xml:"title"` // optional (spec requires title or description)
	Link        string      `xml:"link"`  // optional
	Source      string      `xml:"source,omitempty"`
	Author      CData `xml:"author,omitempty"`
	Description CData `xml:"description"` // optional
	Content     *RssContent `xml:"content:encoded,omitempty"`
	Guid        *RssGuid
	PubDate     string `xml:"pubDate,omitempty"`
	Enclosure   *RssEnclosure
	XMLName     xml.Name        `xml:"item"`
	Category    CData     `xml:"category,omitempty"`
	Comments    CData     `xml:"comments,omitempty"`
	Extra       []ExtensionNode `xml:",any"` // custom nodes at item scope
}


// RssFeed represents the RSS channel.
type RssFeed struct {
	Title          CData `xml:"title"`       // required
	Link           string      `xml:"link"`        // required
	Description    CData `xml:"description"` // required
	ManagingEditor CData `xml:"managingEditor,omitempty"`
	LastBuildDate  string      `xml:"lastBuildDate,omitempty"`
	PubDate        string      `xml:"pubDate,omitempty"`
	Items          []*RssItem  `xml:"item"`
	Copyright      CData `xml:"copyright,omitempty"`
	Image          *RssImage   `xml:"image,omitempty"`
	Language       string      `xml:"language,omitempty"`
	Category       CData `xml:"category,omitempty"`

	XMLName   xml.Name        `xml:"channel"`
	WebMaster CData     `xml:"webMaster,omitempty"`
	Generator CData     `xml:"generator,omitempty"`
	Docs      CData     `xml:"docs,omitempty"`
	Cloud     CData     `xml:"cloud,omitempty"`
	Ttl       int             `xml:"ttl,omitempty"`
	Rating    CData     `xml:"rating,omitempty"`
	SkipHours CData     `xml:"skipHours,omitempty"`
	SkipDays  CData     `xml:"skipDays,omitempty"`
	Extra     []ExtensionNode `xml:",any"` // custom nodes at channel scope
}


// Rss is a wrapper to marshal a Feed as RSS 2.0.
type Rss struct {
	*Feed
}


// FeedXml returns an XML-Ready object for an Rss object.
func (r *Rss) FeedXml() interface{} {
	return r.RssFeed().FeedXml()
}

// Internal helpers to reduce cyclomatic complexity.

// rssAuthorString builds the RSS author string (email with optional name in parens).
func rssAuthorString(a *Author) string {
	if a == nil {
		return ""
	}
	if s := strings.TrimSpace(a.Name); s != "" && strings.TrimSpace(a.Email) != "" {
		return fmt.Sprintf("%s (%s)", a.Email, s)
	}
	return a.Email
}

type rssChannelExtras struct {
	imgW, imgH                        int
	ttl                               int
	catOverride                       string
	webMaster, generator, docs, cloud string
	rating, skipHours, skipDays       string
	nonRSSExtras                      []ExtensionNode
}

func parsePositiveInt(s string) (int, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, false
	}
	v, err := strconv.Atoi(t)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

type rssChannelHandler func(*rssChannelExtras, ExtensionNode)

func handleRSSImageSize(out *rssChannelExtras, n ExtensionNode) {
	if n.Attrs == nil {
		return
	}
	if s, ok := n.Attrs["width"]; ok {
		if v, ok2 := parsePositiveInt(s); ok2 {
			out.imgW = v
		}
	}
	if s, ok := n.Attrs["height"]; ok {
		if v, ok2 := parsePositiveInt(s); ok2 {
			out.imgH = v
		}
	}
}

func handleRSSTTL(out *rssChannelExtras, n ExtensionNode) {
	if v, ok := parsePositiveInt(n.Text); ok {
		out.ttl = v
	}
}

func handleRSSCategory(out *rssChannelExtras, n ExtensionNode) {
	out.catOverride = strings.TrimSpace(n.Text)
}
func handleRSSWebMaster(out *rssChannelExtras, n ExtensionNode) {
	out.webMaster = strings.TrimSpace(n.Text)
}
func handleRSSGenerator(out *rssChannelExtras, n ExtensionNode) {
	out.generator = strings.TrimSpace(n.Text)
}
func handleRSSDocs(out *rssChannelExtras, n ExtensionNode)   { out.docs = strings.TrimSpace(n.Text) }
func handleRSSCloud(out *rssChannelExtras, n ExtensionNode)  { out.cloud = strings.TrimSpace(n.Text) }
func handleRSSRating(out *rssChannelExtras, n ExtensionNode) { out.rating = strings.TrimSpace(n.Text) }
func handleRSSSkipHours(out *rssChannelExtras, n ExtensionNode) {
	out.skipHours = strings.TrimSpace(n.Text)
}
func handleRSSSkipDays(out *rssChannelExtras, n ExtensionNode) {
	out.skipDays = strings.TrimSpace(n.Text)
}

func extractRSSChannelExtras(exts []ExtensionNode) rssChannelExtras {
	var out rssChannelExtras
	if len(exts) == 0 {
		return out
	}
	handlers := map[string]rssChannelHandler{
		"_rss:imageSize": handleRSSImageSize,
		"_rss:ttl":       handleRSSTTL,
		"_rss:category":  handleRSSCategory,
		"_rss:webMaster": handleRSSWebMaster,
		"_rss:generator": handleRSSGenerator,
		"_rss:docs":      handleRSSDocs,
		"_rss:cloud":     handleRSSCloud,
		"_rss:rating":    handleRSSRating,
		"_rss:skipHours": handleRSSSkipHours,
		"_rss:skipDays":  handleRSSSkipDays,
	}
	for _, n := range exts {
		if h, ok := handlers[n.Name]; ok {
			h(&out, n)
		} else {
			out.nonRSSExtras = append(out.nonRSSExtras, n)
		}
	}
	return out
}

func rssImageFromFeed(img *Image, w, h int) *RssImage {
	if img == nil {
		return nil
	}
	return &RssImage{
		Url:    img.Url,
		Title:  img.Title,
		Link:   img.Link,
		Width:  w,
		Height: h,
	}
}

func resolveChannelCategory(f *Feed, override string) string {
	if s := strings.TrimSpace(override); s != "" {
		return s
	}
	if len(f.Categories) > 0 && f.Categories[0] != nil && strings.TrimSpace(f.Categories[0].Text) != "" {
		return strings.TrimSpace(f.Categories[0].Text)
	}
	return ""
}

func itemRSSExtensions(exts []ExtensionNode) (category, comments string, extras []ExtensionNode) {
	for _, n := range exts {
		switch n.Name {
		case "_rss:itemCategory":
			if s := strings.TrimSpace(n.Text); s != "" {
				category = s
			} else {
				extras = append(extras, n)
			}
		case "_rss:comments":
			if s := strings.TrimSpace(n.Text); s != "" {
				comments = s
			} else {
				extras = append(extras, n)
			}
		default:
			extras = append(extras, n)
		}
	}
	return
}

// RssFeed builds the channel structure from the generic Feed.
func (r *Rss) RssFeed() *RssFeed {
	pub := anyTimeFormat(time.RFC1123Z, r.Created, r.Updated)
	build := anyTimeFormat(time.RFC1123Z, r.Updated)
	author := rssAuthorString(r.Author)

	// Extract unified RSS builder markers from feed extensions
	extras := extractRSSChannelExtras(r.Extensions)

	var href string
	if r.Link != nil {
		href = r.Link.Href
	}
	channel := &RssFeed{
		Title:          CData(r.Title),
		Link:           href,
		Description:    CData(r.Description),
		ManagingEditor: CData(author),
		PubDate:        pub,
		LastBuildDate:  build,
		Copyright:      CData(r.Copyright),
		Image:          rssImageFromFeed(r.Image, extras.imgW, extras.imgH),
		Language:       r.Language,
		WebMaster:      CData(extras.webMaster),
		Generator:      CData(extras.generator),
		Docs:           CData(extras.docs),
		Cloud:          CData(extras.cloud),
		Ttl:            extras.ttl,
		Rating:         CData(extras.rating),
		SkipHours:      CData(extras.skipHours),
		SkipDays:       CData(extras.skipDays),
	}

	// Category override or generic mapping
	channel.Category = CData(resolveChannelCategory(r.Feed, extras.catOverride))

	// append items
	for _, it := range r.Items {
		channel.Items = append(channel.Items, newRssItem(it))
	}

	// append non-RSS builder extensions
	if len(extras.nonRSSExtras) > 0 {
		channel.Extra = append(channel.Extra, extras.nonRSSExtras...)
	}
	return channel
}

// FeedXml returns an XML-ready object for an RssFeed object (wrapped with <rss>).
func (r *RssFeed) FeedXml() interface{} {
	// Only add content namespace if any item has content:encoded
	contentNS := ""
	for _, it := range r.Items {
		if it.Content != nil && it.Content.Content != "" {
			contentNS = "http://purl.org/rss/1.0/modules/content/"
			break
		}
	}
	return &RssFeedXml{
		Version:          "2.0",
		Channel:          r,
		ContentNamespace: contentNS,
	}
}

func newRssItem(i *Item) *RssItem {
	item := &RssItem{
		Title:       CData(i.Title),
		Description: CData(i.Description),
		PubDate:     anyTimeFormat(time.RFC1123Z, i.Created, i.Updated),
	}
	if i.ID != "" {
		item.Guid = &RssGuid{ID: i.ID, IsPermaLink: i.IsPermaLink}
	}
	if i.Link != nil {
		item.Link = i.Link.Href
	}
	if len(i.Content) > 0 {
		item.Content = &RssContent{Content: i.Content}
	}
	if i.Source != nil {
		item.Source = i.Source.Href
	}
	if i.Enclosure != nil && i.Enclosure.Type != "" && i.Enclosure.Url != "" && i.Enclosure.Length > 0 {
		item.Enclosure = &RssEnclosure{
			Url:    i.Enclosure.Url,
			Type:   i.Enclosure.Type,
			Length: strconv.FormatInt(i.Enclosure.Length, 10),
		}
	}
	if i.Author != nil {
		author := i.Author.Email
		if i.Author.Name != "" {
			author = fmt.Sprintf("%s (%s)", i.Author.Email, i.Author.Name)
		}
		item.Author = CData(author)
	}
	// append extensions
	if len(i.Extensions) > 0 {
		cat, comments, extras := itemRSSExtensions(i.Extensions)
		item.Category = CData(cat)
		item.Comments = CData(comments)
		if len(extras) > 0 {
			item.Extra = append(item.Extra, extras...)
		}
	}
	return item
}

// MarshalXML customizes RSS item encoding to emit CDATA based on extensions (default on).
func (it *RssItem) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Force correct element name regardless of caller-provided start
	start.Name.Local = "item"
	itemUse := UseCDATAFromExtensions(it.Extra)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	// Title
	_ = encodeElementCDATA(e, "title", string(it.Title), itemUse)
	// Link
	if err := encodeElementIfSet(e, "link", it.Link); err != nil {
		return err
	}
	// Source
	if err := encodeElementIfSet(e, "source", it.Source); err != nil {
		return err
	}
	// Author
	_ = encodeElementCDATA(e, "author", string(it.Author), itemUse)
	// Description
	_ = encodeElementCDATA(e, "description", string(it.Description), itemUse)
	// content:encoded
	if it.Content != nil && strings.TrimSpace(it.Content.Content) != "" {
		_ = encodeElementCDATA(e, "content:encoded", it.Content.Content, itemUse)
	}
	// Guid
	if it.Guid != nil {
		if err := e.Encode(it.Guid); err != nil {
			return err
		}
	}
	// PubDate
	if err := encodeElementIfSet(e, "pubDate", it.PubDate); err != nil {
		return err
	}
	// Enclosure
	if it.Enclosure != nil {
		if err := e.Encode(it.Enclosure); err != nil {
			return err
		}
	}
	// Category, Comments
	_ = encodeElementCDATA(e, "category", string(it.Category), itemUse)
	_ = encodeElementCDATA(e, "comments", string(it.Comments), itemUse)
	// Extra nodes
	for _, n := range it.Extra {
		if err := e.Encode(n); err != nil {
			return err
		}
	}
	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// MarshalXML customizes RSS channel encoding to emit CDATA based on extensions (default on).
func (ch *RssFeed) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Force correct element name regardless of caller-provided start
	start.Name.Local = "channel"
	chUse := UseCDATAFromExtensions(ch.Extra)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	// Core fields
	_ = encodeElementCDATA(e, "title", string(ch.Title), chUse)
	if err := encodeElementIfSet(e, "link", ch.Link); err != nil {
		return err
	}
	_ = encodeElementCDATA(e, "description", string(ch.Description), chUse)

	_ = encodeElementCDATA(e, "managingEditor", string(ch.ManagingEditor), chUse)
	if err := encodeElementIfSet(e, "lastBuildDate", ch.LastBuildDate); err != nil {
		return err
	}
	if err := encodeElementIfSet(e, "pubDate", ch.PubDate); err != nil {
		return err
	}
	for _, it := range ch.Items {
		if it == nil {
			continue
		}
		// Cascade channel preference to item (item may override via its own _xml:cdata extension)
		itemUse := CDATAUseForItem(chUse, it.Extra)
		tmp := *it
		tmp.Extra = WithCDATAOverride(it.Extra, itemUse)
		if err := tmp.MarshalXML(e, xml.StartElement{Name: xml.Name{Local: "item"}}); err != nil {
			return err
		}
	}
	_ = encodeElementCDATA(e, "copyright", string(ch.Copyright), chUse)
	if ch.Image != nil {
		if err := e.Encode(ch.Image); err != nil {
			return err
		}
	}
	if err := encodeElementIfSet(e, "language", ch.Language); err != nil {
		return err
	}
	_ = encodeElementCDATA(e, "category", string(ch.Category), chUse)

	_ = encodeElementCDATA(e, "webMaster", string(ch.WebMaster), chUse)
	_ = encodeElementCDATA(e, "generator", string(ch.Generator), chUse)
	_ = encodeElementCDATA(e, "docs", string(ch.Docs), chUse)
	_ = encodeElementCDATA(e, "cloud", string(ch.Cloud), chUse)
	if err := encodeIntElementIfPositive(e, "ttl", ch.Ttl); err != nil {
		return err
	}
	_ = encodeElementCDATA(e, "rating", string(ch.Rating), chUse)
	_ = encodeElementCDATA(e, "skipHours", string(ch.SkipHours), chUse)
	_ = encodeElementCDATA(e, "skipDays", string(ch.SkipDays), chUse)

	for _, n := range ch.Extra {
		if err := e.Encode(n); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// ValidateRSS enforces basic RSS 2.0.1 requirements on the generic Feed.
func ValidateRSS(f *Feed) error {
	// Channel-level required fields per RSS 2.0.1
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("rss: channel title required")
	}
	if f.Link == nil || strings.TrimSpace(f.Link.Href) == "" {
		return errors.New("rss: channel link required")
	}
	if strings.TrimSpace(f.Description) == "" {
		return errors.New("rss: channel description required")
	}

	for i, it := range f.Items {
		// An item should have at least a title or a description
		if strings.TrimSpace(it.Title) == "" && strings.TrimSpace(it.Description) == "" {
			return fmt.Errorf("rss: item[%d] must include a title or a description", i)
		}
		// If enclosure present, ensure required attributes are valid
		if it.Enclosure != nil {
			if strings.TrimSpace(it.Enclosure.Url) == "" || strings.TrimSpace(it.Enclosure.Type) == "" || it.Enclosure.Length <= 0 {
				return fmt.Errorf("rss: item[%d] enclosure url/type/length required when enclosure present", i)
			}
		}
		// RSS 2.0 author should be an email address when present
		if it.Author != nil && strings.TrimSpace(it.Author.Email) == "" {
			return fmt.Errorf("rss: item[%d] author must be an email address", i)
		}
	}
	return nil
}

// RSS-specific builder helpers implemented here without touching generic files.
// Feed-level helpers:

func (b *FeedBuilder) WithRSSTTL(ttl int) *FeedBuilder {
	if ttl <= 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:ttl", Text: strconv.Itoa(ttl)})
}

func (b *FeedBuilder) WithRSSImageSize(width, height int) *FeedBuilder {
	attrs := map[string]string{}
	if width > 0 {
		attrs["width"] = strconv.Itoa(width)
	}
	if height > 0 {
		attrs["height"] = strconv.Itoa(height)
	}
	if len(attrs) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:imageSize", Attrs: attrs})
}

func (b *FeedBuilder) WithRSSCategory(category string) *FeedBuilder {
	category = strings.TrimSpace(category)
	if category == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:category", Text: category})
}

func (b *FeedBuilder) WithRSSWebMaster(email string) *FeedBuilder {
	email = strings.TrimSpace(email)
	if email == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:webMaster", Text: email})
}

func (b *FeedBuilder) WithRSSGenerator(gen string) *FeedBuilder {
	gen = strings.TrimSpace(gen)
	if gen == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:generator", Text: gen})
}

func (b *FeedBuilder) WithRSSDocs(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:docs", Text: url})
}

func (b *FeedBuilder) WithRSSCloud(cloud string) *FeedBuilder {
	cloud = strings.TrimSpace(cloud)
	if cloud == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:cloud", Text: cloud})
}

func (b *FeedBuilder) WithRSSRating(rating string) *FeedBuilder {
	rating = strings.TrimSpace(rating)
	if rating == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:rating", Text: rating})
}

func (b *FeedBuilder) WithRSSSkipHours(hours string) *FeedBuilder {
	hours = strings.TrimSpace(hours)
	if hours == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:skipHours", Text: hours})
}

func (b *FeedBuilder) WithRSSSkipDays(days string) *FeedBuilder {
	days = strings.TrimSpace(days)
	if days == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:skipDays", Text: days})
}

// Item-level helpers:

func (b *ItemBuilder) WithRSSItemCategory(category string) *ItemBuilder {
	category = strings.TrimSpace(category)
	if category == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:itemCategory", Text: category})
}

func (b *ItemBuilder) WithRSSComments(url string) *ItemBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_rss:comments", Text: url})
}
