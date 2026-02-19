package gofeedx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
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
type JSONAttachment struct {
	Url      string        `json:"url,omitempty"`
	MIMEType string        `json:"mime_type,omitempty"`
	Title    string        `json:"title,omitempty"`
	Size     int32         `json:"size,omitempty"`
	Duration time.Duration `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (a *JSONAttachment) MarshalJSON() ([]byte, error) {
	type EmbeddedJSONAttachment JSONAttachment
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
	*JSONItemFieldExtensions
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
	Attachments   []JSONAttachment `json:"attachments,omitempty"`
}

type JSONItemFieldExtensions struct {
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
	*JSONFeedFieldExtensions
	Title       string        `json:"title"`
	HomePageUrl string        `json:"home_page_url,omitempty"`
	Description string        `json:"description,omitempty"`
	Authors     []*JSONAuthor `json:"authors,omitempty"` // v1.1
	Items       []*JSONItem   `json:"items,omitempty"`
	Icon        string        `json:"icon,omitempty"`
	Favicon     string        `json:"favicon,omitempty"`
	FeedUrl     string        `json:"feed_url,omitempty"`
}

type JSONFeedFieldExtensions struct {
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

/*
ToJSONString encodes f into a JSON string. Returns an error if marshalling fails.
Use Feed.ToJSONFeed() to get the structured JSONFeed value.
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

// JSONFeed creates a new JSONFeed with a generic Feed struct's data.
func (f *JSON) JSONFeed() *JSONFeed {
	feed := &JSONFeed{
		JSONFeedFieldExtensions: &JSONFeedFieldExtensions{
			Version:  jsonFeedVersion,
			Language: f.Language,
		},
		Title:       f.Title,
		Description: f.Description,
	}

	if f.Link != nil {
		feed.HomePageUrl = f.Link.Href
	}
	if f.FeedURL != "" {
		feed.FeedUrl = f.FeedURL
	}
	if f.Image != nil && f.Image.Url != "" {
		if feed.Icon == "" {
			feed.Icon = f.Image.Url
		}
		if feed.Favicon == "" {
			feed.Favicon = f.Image.Url
		}
	}
	if f.Author != nil {
		author := &JSONAuthor{
			Name: f.Author.Name,
		}
		feed.Authors = []*JSONAuthor{author}
	}
	for _, e := range f.Items {
		feed.Items = append(feed.Items, newJSONItem(e))
	}
	// Copy unified extensions for JSON flattening
	feed.Exts = f.Extensions
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
	// Ensure id is non-empty per JSON Feed spec
	id := i.ID
	link := i.Link
	if id == "" {
		if link != nil && link.Href != "" && (!i.Created.IsZero() || !i.Updated.IsZero()) {
			dateStr := anyTimeFormat("2006-01-02", i.Updated, i.Created)
			host, path := link.Href, "/"
			if u, err := url.Parse(link.Href); err == nil {
				host, path = u.Host, u.Path
			}
			id = fmt.Sprintf("tag:%s,%s:%s", host, dateStr, path)
		} else {
			id = "urn:uuid:" + MustUUIDv4().String()
		}
	}
	item := &JSONItem{
		Id:          id,
		Title:       i.Title,
		Summary:     i.Description,
		ContentHTML: i.Content, // Use HTML when Content present
		JSONItemFieldExtensions: &JSONItemFieldExtensions{
			Exts: i.Extensions,
		},
	}

	if i.Link != nil {
		item.Url = i.Link.Href
	}
	if i.Source != nil {
		item.ExternalUrl = i.Source.Href
	}
	if i.Author != nil {
		author := &JSONAuthor{
			Name: i.Author.Name,
		}
		item.Authors = []*JSONAuthor{author}
	}
	if !i.Created.IsZero() {
		item.PublishedDate = &i.Created
	}
	if !i.Updated.IsZero() {
		item.ModifiedDate = &i.Updated
	}
	// Map enclosure:
	// - If it's an image, map to JSON Feed's "image"
	// - Otherwise, add as an attachment with optional duration
	if i.Enclosure != nil {
		if strings.HasPrefix(i.Enclosure.Type, "image/") {
			item.Image = i.Enclosure.Url
		} else {
			var sz int32
			if i.Enclosure.Length > maxSize {
				sz = maxSize
			} else if i.Enclosure.Length > 0 {
				sz = int32(i.Enclosure.Length)
			}
			att := JSONAttachment{
				Url:      i.Enclosure.Url,
				MIMEType: i.Enclosure.Type,
				Size:     sz,
			}
			if i.DurationSeconds > 0 {
				att.Duration = time.Duration(i.DurationSeconds) * time.Second
			}
			item.Attachments = append(item.Attachments, att)
		}
	}

	return item
}

/*
ToJSONString creates a JSON Feed representation of this feed as a string.
Use ToJSONFeed() if you need the structured JSONFeed value for further processing.
*/
func (f *Feed) ToJSONString() (string, error) {
	j := &JSON{f}
	return j.ToJSONString()
}

/*
ToJSONFeed returns the JSONFeed struct for this feed.
*/
func (f *Feed) ToJSONFeed() (*JSONFeed, error) {
	j := &JSON{f}
	return j.JSONFeed(), nil
}

// WriteJSON writes a JSON representation of this feed to the writer.
func (f *Feed) WriteJSON(w io.Writer) error {
	j := &JSON{f}
	feed := j.JSONFeed()
	return WriteJSON(feed, w)
}

// ValidateJSON enforces JSON Feed 1.1 essentials on the generic Feed.
func (f *Feed) ValidateJSON() error {
	// Top-level required: title (version is set by the writer), items must be present
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("json: feed title required")
	}
	// Writer omits 'items' when empty due to omitempty; enforce at least one item to avoid invalid output
	if len(f.Items) == 0 {
		return errors.New("json: at least one item required")
	}
	// Item-level: id is required by spec
	for i, it := range f.Items {
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("json: item[%d] id required", i)
		}
	}
	return nil
}
