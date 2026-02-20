package gofeedx

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	jsonFeedVersion = "https://jsonfeed.org/version/1.1"
	maxSize         = 2147483647
)

// JSONAuthor represents the author of the feed or of an individual item
type JSONAuthor struct {
	Name   string `json:"name,omitempty"`
	Url    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// JSONAttachment represents a related resource. (Kept for future expansion)
type jsonAttachment struct {
	Url      string        `json:"url,omitempty"`
	MIMEType string        `json:"mime_type,omitempty"`
	Title    string        `json:"title,omitempty"`
	Size     int32         `json:"size,omitempty"`
	Duration time.Duration `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (a *jsonAttachment) MarshalJSON() ([]byte, error) {
	type EmbeddedJSONAttachment jsonAttachment
	type out struct {
		Duration float64 `json:"duration_in_seconds,omitempty"`
		*EmbeddedJSONAttachment
	}
	o := out{
		EmbeddedJSONAttachment: (*EmbeddedJSONAttachment)(a),
	}
	if a.Duration > 0 {
		o.Duration = a.Duration.Seconds()
	}
	return json.Marshal(o)
}

// JSONItem represents a single entry/post for the feed.
type JSONItem struct {
	Title         string           `json:"title,omitempty"`
	Url           string           `json:"url,omitempty"`
	ExternalUrl   string           `json:"external_url,omitempty"`
	Authors       []*JSONAuthor    `json:"authors,omitempty"` // v1.1
	Summary       string           `json:"summary,omitempty"`
	ContentHTML   string           `json:"content_html,omitempty"`
	Id            string           `json:"id"`
	ModifiedDate  *time.Time       `json:"date_modified,omitempty"`
	PublishedDate *time.Time       `json:"date_published,omitempty"`
	Image         string           `json:"image,omitempty"`
	Attachments   []jsonAttachment `json:"attachments,omitempty"`

	ContentText string          `json:"content_text,omitempty"`
	BannerImage string          `json:"banner_image,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Exts        []ExtensionNode `json:"-"`
}

// JSONHub describes an endpoint that can be used to subscribe to real-time notifications.
type JSONHub struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

// JSONFeed represents a syndication feed in the JSON Feed Version 1.1 format.
type JSONFeed struct {
	Title       string        `json:"title"`
	HomePageUrl string        `json:"home_page_url,omitempty"`
	Description string        `json:"description,omitempty"`
	Authors     []*JSONAuthor `json:"authors,omitempty"` // v1.1
	Items       []*JSONItem   `json:"items,omitempty"`
	Icon        string        `json:"icon,omitempty"`
	Favicon     string        `json:"favicon,omitempty"`
	FeedUrl     string        `json:"feed_url,omitempty"`

	Version     string          `json:"version"`
	Language    string          `json:"language,omitempty"`
	UserComment string          `json:"user_comment,omitempty"`
	NextUrl     string          `json:"next_url,omitempty"`
	Expired     *bool           `json:"expired,omitempty"`
	Hubs        []*JSONHub      `json:"hubs,omitempty"`
	Exts        []ExtensionNode `json:"-"`
}

// JSON is used to convert a generic Feed to a JSONFeed.
type JSON struct {
	*Feed
}

// ToJSON renders the feed to a JSON Feed 1.1 string after validating ProfileJSON.
// Note: JSONFeed writer requires each item to have an ID. If missing, consider
// building with ProfileJSON and letting the builder supply a fallback.
func ToJSON(feed *Feed) (string, error) {
	if feed == nil {
		return "", errors.New("nil feed")
	}
	j := &JSON{Feed: feed}
	return j.ToJSONString()
}

/*
ToJSONString encodes f into a JSON string. Returns an error if marshalling fails.
Use JSON.JSONFeed() to get the structured JSONFeed value.
*/
func (f *JSON) ToJSONString() (string, error) {
	return f.JSONFeed().ToJSONString()
}

/*
ToJSONString encodes f into a JSON string. Returns an error if marshalling fails.
*/
func (f *JSONFeed) ToJSONString() (string, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MarshalJSON implements custom JSON serialization to include flattened extensions
func (f *JSONFeed) MarshalJSON() ([]byte, error) {
	// Marshal known fields first
	type Alias JSONFeed
	a := (*Alias)(f)
	base, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	// Convert to map to inject custom keys
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	// Flatten extensions: name -> text (attributes/children ignored)
	for _, n := range f.Exts {
		if n.Name == "" || n.Text == "" {
			continue
		}
		m[n.Name] = n.Text
	}
	return json.Marshal(m)
}

// Internal helpers to reduce cyclomatic complexity for JSON conversion.

func jsonFeedBaseFromFeed(f *Feed) *JSONFeed {
	feed := &JSONFeed{
		Version:     jsonFeedVersion,
		Language:    f.Language,
		Title:       f.Title,
		Description: f.Description,
	}
	if f.Link != nil {
		feed.HomePageUrl = f.Link.Href
	}
	if f.FeedURL != "" {
		feed.FeedUrl = f.FeedURL
	}
	if f.Author != nil {
		feed.Authors = jsonAuthorsFromAuthor(f.Author)
	}
	applyFeedIconsFromImage(feed, f.Image)
	return feed
}

func jsonAuthorsFromAuthor(a *Author) []*JSONAuthor {
	if a == nil {
		return nil
	}
	return []*JSONAuthor{{Name: a.Name}}
}

func applyFeedIconsFromImage(feed *JSONFeed, img *Image) {
	if img == nil || strings.TrimSpace(img.Url) == "" {
		return
	}
	if strings.TrimSpace(feed.Icon) == "" {
		feed.Icon = img.Url
	}
	if strings.TrimSpace(feed.Favicon) == "" {
		feed.Favicon = img.Url
	}
}

func mapFeedExtensionsToJSON(feed *JSONFeed, exts []ExtensionNode) {
	if len(exts) == 0 {
		return
	}
	type handler func(*JSONFeed, ExtensionNode) bool
	handlers := map[string]handler{
		"_json:user_comment": func(f *JSONFeed, n ExtensionNode) bool {
			if s := strings.TrimSpace(n.Text); s != "" {
				f.UserComment = s
				return true
			}
			return false
		},
		"_json:next_url": func(f *JSONFeed, n ExtensionNode) bool {
			if s := strings.TrimSpace(n.Text); s != "" {
				f.NextUrl = s
				return true
			}
			return false
		},
		"_json:expired": func(f *JSONFeed, n ExtensionNode) bool {
			switch strings.ToLower(strings.TrimSpace(n.Text)) {
			case "true":
				v := true
				f.Expired = &v
				return true
			case "false":
				v := false
				f.Expired = &v
				return true
			default:
				return false
			}
		},
		"_json:hub": func(f *JSONFeed, n ExtensionNode) bool {
			var ht, hu string
			if n.Attrs != nil {
				ht = strings.TrimSpace(n.Attrs["type"])
				hu = strings.TrimSpace(n.Attrs["url"])
			}
			if ht != "" && hu != "" {
				f.Hubs = append(f.Hubs, &JSONHub{Type: ht, Url: hu})
				return true
			}
			return false
		},
		"_json:icon": func(f *JSONFeed, n ExtensionNode) bool {
			if s := strings.TrimSpace(n.Text); s != "" {
				f.Icon = s
				return true
			}
			return false
		},
		"_json:favicon": func(f *JSONFeed, n ExtensionNode) bool {
			if s := strings.TrimSpace(n.Text); s != "" {
				f.Favicon = s
				return true
			}
			return false
		},
	}
	var extras []ExtensionNode
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		if h, ok := handlers[name]; ok {
			if h(feed, n) {
				continue
			}
		}
		extras = append(extras, n)
	}
	feed.Exts = extras
}

func jsonItemBase(i *Item) *JSONItem {
	id := strings.TrimSpace(i.ID)
	if id == "" {
		id = fallbackItemGuid(i)
	}
	item := &JSONItem{
		Id:          id,
		Title:       i.Title,
		Summary:     i.Description,
		ContentHTML: i.Content,
	}
	if i.Link != nil {
		item.Url = i.Link.Href
	}
	if i.Source != nil {
		item.ExternalUrl = i.Source.Href
	}
	if i.Author != nil {
		item.Authors = jsonAuthorsFromAuthor(i.Author)
	}
	if !i.Created.IsZero() {
		item.PublishedDate = &i.Created
	}
	if !i.Updated.IsZero() {
		item.ModifiedDate = &i.Updated
	}
	return item
}

func addItemEnclosure(j *JSONItem, i *Item) {
	if i.Enclosure == nil {
		return
	}
	// If it's an image, map to JSON Feed's "image"
	if strings.HasPrefix(i.Enclosure.Type, "image/") {
		j.Image = i.Enclosure.Url
		return
	}
	// Otherwise, add as an attachment with optional duration
	var sz int32
	if i.Enclosure.Length > maxSize {
		sz = maxSize
	} else if i.Enclosure.Length > 0 {
		sz = int32(i.Enclosure.Length)
	}
	att := jsonAttachment{
		Url:      i.Enclosure.Url,
		MIMEType: i.Enclosure.Type,
		Size:     sz,
	}
	if i.DurationSeconds > 0 {
		att.Duration = time.Duration(i.DurationSeconds) * time.Second
	}
	j.Attachments = append(j.Attachments, att)
}

func mapItemExtensionsToJSON(ji *JSONItem, exts []ExtensionNode) {
	if len(exts) == 0 {
		return
	}
	var extras []ExtensionNode
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		switch name {
		case "_json:content_text":
			if s := strings.TrimSpace(n.Text); s != "" {
				ji.ContentText = s
			} else {
				extras = append(extras, n)
			}
		case "_json:banner_image":
			if s := strings.TrimSpace(n.Text); s != "" {
				ji.BannerImage = s
			} else {
				extras = append(extras, n)
			}
		case "_json:tags":
			if s := strings.TrimSpace(n.Text); s != "" {
				parts := strings.Split(s, ",")
				for _, p := range parts {
					if t := strings.TrimSpace(p); t != "" {
						ji.Tags = append(ji.Tags, t)
					}
				}
			} else {
				extras = append(extras, n)
			}
		case "_json:tag":
			if s := strings.TrimSpace(n.Text); s != "" {
				ji.Tags = append(ji.Tags, s)
			} else {
				extras = append(extras, n)
			}
		case "_json:image":
			if s := strings.TrimSpace(n.Text); s != "" {
				ji.Image = s
			} else {
				extras = append(extras, n)
			}
		default:
			extras = append(extras, n)
		}
	}
	ji.Exts = extras
}

// JSONFeed creates a new JSONFeed with a generic Feed struct's data.
func (f *JSON) JSONFeed() *JSONFeed {
	feed := jsonFeedBaseFromFeed(f.Feed)

	// Items
	for _, e := range f.Items {
		ji := newJSONItem(e)
		feed.Items = append(feed.Items, ji)
	}

	// Extensions mapping and flattening extras
	mapFeedExtensionsToJSON(feed, f.Extensions)
	return feed
}

func (ji *JSONItem) MarshalJSON() ([]byte, error) {
	// Marshal known fields first
	type Alias JSONItem
	a := (*Alias)(ji)
	base, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	// Convert to map to inject custom keys
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	// Flatten extensions: name -> text (attributes/children ignored)
	for _, n := range ji.Exts {
		if n.Name == "" || n.Text == "" {
			continue
		}
		m[n.Name] = n.Text
	}
	return json.Marshal(m)
}

func newJSONItem(i *Item) *JSONItem {
	item := jsonItemBase(i)
	addItemEnclosure(item, i)
	mapItemExtensionsToJSON(item, i.Extensions)
	return item
}

// ValidateJSON enforces JSON Feed 1.1 essentials on the generic Feed.
func ValidateJSON(f *Feed) error {
	// Top-level required: title (version is set by the writer), items must be present
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("json: feed title required")
	}

	// Item-level: id is required by spec
	for i, it := range f.Items {
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("json: item[%d] id required", i)
		}
	}
	return nil
}

// JSON-specific builder helpers implemented here without touching generic files.
// Feed-level helpers:

// WithJSONUserComment sets feed-level user_comment.
func (b *FeedBuilder) WithJSONUserComment(text string) *FeedBuilder {
	text = strings.TrimSpace(text)
	if text == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:user_comment", Text: text})
}

// WithJSONNextURL sets feed-level next_url.
func (b *FeedBuilder) WithJSONNextURL(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:next_url", Text: url})
}

// WithJSONExpired sets feed-level expired flag.
func (b *FeedBuilder) WithJSONExpired(expired bool) *FeedBuilder {
	val := "false"
	if expired {
		val = "true"
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:expired", Text: val})
}

// WithJSONHub adds a PubSub hub.
func (b *FeedBuilder) WithJSONHub(hubType, url string) *FeedBuilder {
	hubType = strings.TrimSpace(hubType)
	url = strings.TrimSpace(url)
	if hubType == "" || url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:hub", Attrs: map[string]string{"type": hubType, "url": url}})
}

// WithJSONIcon overrides feed icon.
func (b *FeedBuilder) WithJSONIcon(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:icon", Text: url})
}

// WithJSONFavicon overrides feed favicon.
func (b *FeedBuilder) WithJSONFavicon(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:favicon", Text: url})
}

// Item-level helpers:

// WithJSONContentText sets item content_text.
func (b *ItemBuilder) WithJSONContentText(text string) *ItemBuilder {
	text = strings.TrimSpace(text)
	if text == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:content_text", Text: text})
}

// WithJSONBannerImage sets item banner_image.
func (b *ItemBuilder) WithJSONBannerImage(url string) *ItemBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:banner_image", Text: url})
}

// WithJSONTags sets item tags from a list.
func (b *ItemBuilder) WithJSONTags(tags ...string) *ItemBuilder {
	if len(tags) == 0 {
		return b
	}
	trimmed := make([]string, 0, len(tags))
	for _, t := range tags {
		if s := strings.TrimSpace(t); s != "" {
			trimmed = append(trimmed, s)
		}
	}
	if len(trimmed) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:tags", Text: strings.Join(trimmed, ",")})
}

// WithJSONTag appends a single tag.
func (b *ItemBuilder) WithJSONTag(tag string) *ItemBuilder {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:tag", Text: tag})
}

// WithJSONImage overrides item image.
func (b *ItemBuilder) WithJSONImage(url string) *ItemBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_json:image", Text: url})
}
