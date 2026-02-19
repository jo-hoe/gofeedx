package gofeedx

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Profiles identify target formats to validate for when building feeds.
type Profile int

const (
	ProfileRSS Profile = iota
	ProfileAtom
	ProfilePSP
	ProfileJSON
)

// FeedBuilder constructs a canonical Feed using a fluent, type-safe API.
// Build() optionally validates the result for one or more target profiles.
type FeedBuilder struct {
	feed     Feed
	items    []*Item
	strict   bool
	profiles []Profile
}

// NewFeed creates a new FeedBuilder with a required title.
// Title must not be empty in strict mode.
func NewFeed(title string) *FeedBuilder {
	return &FeedBuilder{
		feed:   Feed{Title: strings.TrimSpace(title)},
		strict: true,
	}
}

// WithLenient disables strict builder checks (Build still runs selected profile validations if any).
func (b *FeedBuilder) WithLenient() *FeedBuilder {
	b.strict = false
	return b
}

// WithProfiles sets the profiles to validate against on Build.
func (b *FeedBuilder) WithProfiles(p ...Profile) *FeedBuilder {
	b.profiles = append([]Profile{}, p...)
	return b
}

// WithID sets a stable feed ID (used by Atom/JSON and podcast:guid fallback in PSP).
func (b *FeedBuilder) WithID(id string) *FeedBuilder {
	b.feed.ID = strings.TrimSpace(id)
	return b
}

// WithLink sets the primary site/page link.
func (b *FeedBuilder) WithLink(href string) *FeedBuilder {
	href = strings.TrimSpace(href)
	if href == "" {
		b.feed.Link = nil
		return b
	}
	b.feed.Link = &Link{Href: href}
	return b
}

// WithFeedURL sets the canonical feed URL (used by JSON Feed feed_url and PSP atom:link rel=self).
func (b *FeedBuilder) WithFeedURL(url string) *FeedBuilder {
	b.feed.FeedURL = strings.TrimSpace(url)
	return b
}

// WithDescription sets the feed/channel description/subtitle.
func (b *FeedBuilder) WithDescription(desc string) *FeedBuilder {
	b.feed.Description = desc
	return b
}

// WithAuthor sets the feed author.
func (b *FeedBuilder) WithAuthor(name, email string) *FeedBuilder {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" && email == "" {
		b.feed.Author = nil
		return b
	}
	b.feed.Author = &Author{Name: name, Email: email}
	return b
}

// WithUpdated sets the feed updated timestamp.
func (b *FeedBuilder) WithUpdated(t time.Time) *FeedBuilder {
	b.feed.Updated = t
	return b
}

// WithCreated sets the feed created/published timestamp.
func (b *FeedBuilder) WithCreated(t time.Time) *FeedBuilder {
	b.feed.Created = t
	return b
}

// WithCopyright sets the feed copyright/rights.
func (b *FeedBuilder) WithCopyright(c string) *FeedBuilder {
	b.feed.Copyright = c
	return b
}

// WithLanguage sets the feed language (e.g., en-US).
func (b *FeedBuilder) WithLanguage(lang string) *FeedBuilder {
	b.feed.Language = strings.TrimSpace(lang)
	return b
}

// WithImage sets the feed image/artwork/logo.
func (b *FeedBuilder) WithImage(url, title, link string) *FeedBuilder {
	url = strings.TrimSpace(url)
	title = strings.TrimSpace(title)
	link = strings.TrimSpace(link)
	if url == "" && title == "" && link == "" {
		b.feed.Image = nil
		return b
	}
	b.feed.Image = &Image{Url: url, Title: title, Link: link}
	return b
}

// WithCategories replaces the feed categories with the provided list.
func (b *FeedBuilder) WithCategories(categories ...string) *FeedBuilder {
	var out []*Category
	for _, c := range categories {
		if s := strings.TrimSpace(c); s != "" {
			out = append(out, &Category{Text: s})
		}
	}
	b.feed.Categories = out
	return b
}

/*
WithExtensions appends raw extension nodes at feed/channel scope.
This is the single way to add target-specific elements using the builder.
*/
func (b *FeedBuilder) WithExtensions(nodes ...ExtensionNode) *FeedBuilder {
	if len(nodes) == 0 {
		return b
	}
	b.feed.Extensions = append(b.feed.Extensions, nodes...)
	return b
}

// AddItem appends a built item to the feed.
// If ib.Build() returns an error, it is ignored here and handled by profile validation in Build.
func (b *FeedBuilder) AddItem(ib *ItemBuilder) *FeedBuilder {
	if ib == nil {
		return b
	}
	it, _ := ib.Build()
	b.items = append(b.items, it) // it may be nil if ib.Build() failed in lenient mode; filter in Build()
	return b
}

// AddItemFunc creates and configures an item with the supplied function.
func (b *FeedBuilder) AddItemFunc(fn func(*ItemBuilder)) *FeedBuilder {
	if fn == nil {
		return b
	}
	ib := NewItem("") // allow user to set title via fn
	fn(ib)
	return b.AddItem(ib)
}

// WithSort sets a stable sort for items; call before Build.
func (b *FeedBuilder) WithSort(less func(a, b *Item) bool) *FeedBuilder {
	if less == nil {
		return b
	}
	// Sort the builder's items directly using a stable sort
	sort.SliceStable(b.items, func(i, j int) bool {
		return less(b.items[i], b.items[j])
	})
	return b
}

/*
Typed sorting for items.

Define the attribute to sort by using ItemSortField and the direction using SortDir.
Avoids stringly-typed APIs and provides compile-time safety.
*/
type ItemSortField int

const (
	SortByTitle ItemSortField = iota
	SortByID
	SortByCreated
	SortByUpdated
	SortByDuration
	SortByAuthorName
)

type SortDir int

const (
	SortAsc SortDir = iota
	SortDesc
)

/*
WithSortBy sorts items by a typed attribute and direction.
Supported fields: SortByTitle, SortByID, SortByCreated, SortByUpdated, SortByDuration, SortByAuthorName.
Call before Build().
*/
func (b *FeedBuilder) WithSortBy(field ItemSortField, dir SortDir) *FeedBuilder {
	asc := dir != SortDesc

	less := func(a, b *Item) bool {
		switch field {
		case SortByTitle:
			return stringLessCI(a.Title, b.Title, asc)
		case SortByID:
			return stringLessCI(a.ID, b.ID, asc)
		case SortByCreated:
			return timeLess(a.Created, b.Created, asc)
		case SortByUpdated:
			return timeLess(a.Updated, b.Updated, asc)
		case SortByDuration:
			return int64Less(int64(a.DurationSeconds), int64(b.DurationSeconds), asc)
		case SortByAuthorName:
			return stringLessCI(getAuthorName(a.Author), getAuthorName(b.Author), asc)
		default:
			// Fallback to created date
			return timeLess(a.Created, b.Created, asc)
		}
	}
	return b.WithSort(less)
}

// Helpers for typed sorting

func stringLessCI(a, b string, asc bool) bool {
	aa := strings.ToLower(a)
	bb := strings.ToLower(b)
	if asc {
		return aa < bb
	}
	return aa > bb
}

func int64Less(a, b int64, asc bool) bool {
	if asc {
		return a < b
	}
	return a > b
}

func timeLess(a, b time.Time, asc bool) bool {
	if asc {
		if a.Equal(b) {
			return false
		}
		return a.Before(b)
	}
	if a.Equal(b) {
		return false
	}
	return a.After(b)
}

func getLinkHref(l *Link) string {
	if l == nil {
		return ""
	}
	return l.Href
}

func getAuthorName(a *Author) string {
	if a == nil {
		return ""
	}
	return a.Name
}

func getAuthorEmail(a *Author) string {
	if a == nil {
		return ""
	}
	return a.Email
}

func getEnclosureLength(e *Enclosure) int64 {
	if e == nil {
		return 0
	}
	return e.Length
}

func getEnclosureType(e *Enclosure) string {
	if e == nil {
		return ""
	}
	return e.Type
}

func getEnclosureURL(e *Enclosure) string {
	if e == nil {
		return ""
	}
	return e.Url
}

// Build assembles the Feed and validates against selected profiles.
// It also performs minimal defaulting to reduce target-specific failures:
// - For Atom profile: if Updated is zero, use max(items.Updated/Created)
// - For JSON/Atom/PSP profiles: if an item lacks ID, compute a stable fallback
// Returns an error if any selected profile validation fails.
func (b *FeedBuilder) Build() (*Feed, error) {
	// Copy non-nil items
	b.feed.Items = copyNonNilItems(b.items)

	// Basic strict checks
	if b.strict {
		if err := builderStrictChecks(&b.feed); err != nil {
			return nil, err
		}
	}

	// Defaults for Atom Updated
	if containsProfile(b.profiles, ProfileAtom) && b.feed.Updated.IsZero() {
		b.feed.Updated = maxTime(collectItemTimes(b.feed.Items)...)
	}

	// Auto IDs for items when Atom/JSON/PSP targets are selected
	if containsAnyProfile(b.profiles, ProfileAtom, ProfileJSON, ProfilePSP) {
		ensureItemIDs(b.feed.Items)
	}

	// Final profile validations
	if err := runProfileValidations(&b.feed, b.profiles); err != nil {
		return nil, err
	}
	return &b.feed, nil
}

func copyNonNilItems(items []*Item) []*Item {
	var out []*Item
	for _, it := range items {
		if it != nil {
			out = append(out, it)
		}
	}
	return out
}

func builderStrictChecks(f *Feed) error {
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("builder: feed title required")
	}
	if len(f.Items) == 0 {
		return errors.New("builder: at least one item required")
	}
	// enclosure checks delegated to ItemBuilder strict mode; feed-level has none
	return nil
}

func ensureItemIDs(items []*Item) {
	for _, it := range items {
		if strings.TrimSpace(it.ID) == "" {
			it.ID = fallbackItemGuid(it) // stable tag: or uuid v4 urn
			// For RSS/PSP GUID permalink flag is optional; default to "false" when auto-set
			if it.IsPermaLink == "" {
				it.IsPermaLink = "false"
			}
		}
	}
}

func runProfileValidations(f *Feed, profiles []Profile) error {
	var verr error
	for _, p := range profiles {
		switch p {
		case ProfileRSS:
			if err := ValidateRSS(f); err != nil {
				verr = errors.Join(verr, err)
			}
		case ProfileAtom:
			if err := ValidateAtom(f); err != nil {
				verr = errors.Join(verr, err)
			}
		case ProfilePSP:
			if err := ValidatePSP(f); err != nil {
				verr = errors.Join(verr, err)
			}
		case ProfileJSON:
			if err := ValidateJSON(f); err != nil {
				verr = errors.Join(verr, err)
			}
		}
	}
	return verr
}

func containsProfile(set []Profile, p Profile) bool {
	for _, x := range set {
		if x == p {
			return true
		}
	}
	return false
}

func containsAnyProfile(set []Profile, ps ...Profile) bool {
	for _, p := range ps {
		if containsProfile(set, p) {
			return true
		}
	}
	return false
}

func collectItemTimes(items []*Item) []time.Time {
	var ts []time.Time
	for _, it := range items {
		if !it.Updated.IsZero() {
			ts = append(ts, it.Updated)
		}
		if !it.Created.IsZero() {
			ts = append(ts, it.Created)
		}
	}
	return ts
}

func maxTime(times ...time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	// Copy and sort
	tmp := make([]time.Time, 0, len(times))
	for _, t := range times {
		if !t.IsZero() {
			tmp = append(tmp, t)
		}
	}
	if len(tmp) == 0 {
		return time.Time{}
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].Before(tmp[j]) })
	return tmp[len(tmp)-1]
}

// ItemBuilder constructs a canonical Item using a fluent API.
type ItemBuilder struct {
	item   Item
	strict bool
}

// NewItem creates a new ItemBuilder with an optional title.
func NewItem(title string) *ItemBuilder {
	return &ItemBuilder{
		item:   Item{Title: strings.TrimSpace(title)},
		strict: true,
	}
}

// WithLenient disables strict item checks (Build errors relaxed).
func (b *ItemBuilder) WithLenient() *ItemBuilder {
	b.strict = false
	return b
}

// WithTitle sets the item title.
func (b *ItemBuilder) WithTitle(title string) *ItemBuilder {
	b.item.Title = strings.TrimSpace(title)
	return b
}

// WithID sets the item ID/guid (Atom/JSON id, RSS/PSP guid).
func (b *ItemBuilder) WithID(id string) *ItemBuilder {
	b.item.ID = strings.TrimSpace(id)
	return b
}

// WithGUID sets the RSS/PSP guid with isPermaLink flag.
func (b *ItemBuilder) WithGUID(id string, isPermaLink string) *ItemBuilder {
	b.item.ID = strings.TrimSpace(id)
	// isPermaLink must be "true" or "false" or omitted (we store raw and let encoders omit if empty)
	switch strings.TrimSpace(strings.ToLower(isPermaLink)) {
	case "true", "false":
		b.item.IsPermaLink = isPermaLink
	default:
		// keep as-is; encoders may omit
		b.item.IsPermaLink = strings.TrimSpace(isPermaLink)
	}
	return b
}

// WithLink sets the item link.
func (b *ItemBuilder) WithLink(href string) *ItemBuilder {
	href = strings.TrimSpace(href)
	if href == "" {
		b.item.Link = nil
		return b
	}
	b.item.Link = &Link{Href: href}
	return b
}

// WithSource sets a related/alternate source link.
func (b *ItemBuilder) WithSource(href string) *ItemBuilder {
	href = strings.TrimSpace(href)
	if href == "" {
		b.item.Source = nil
		return b
	}
	b.item.Source = &Link{Href: href}
	return b
}

// WithAuthor sets the item author.
func (b *ItemBuilder) WithAuthor(name, email string) *ItemBuilder {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" && email == "" {
		b.item.Author = nil
		return b
	}
	b.item.Author = &Author{Name: name, Email: email}
	return b
}

// WithDescription sets the item description/summary.
func (b *ItemBuilder) WithDescription(d string) *ItemBuilder {
	b.item.Description = d
	return b
}

// WithContentHTML sets the item HTML content.
func (b *ItemBuilder) WithContentHTML(html string) *ItemBuilder {
	b.item.Content = html
	return b
}

// WithCreated sets the item published date.
func (b *ItemBuilder) WithCreated(t time.Time) *ItemBuilder {
	b.item.Created = t
	return b
}

// WithUpdated sets the item updated date.
func (b *ItemBuilder) WithUpdated(t time.Time) *ItemBuilder {
	b.item.Updated = t
	return b
}

// WithEnclosure sets the item enclosure/media.
func (b *ItemBuilder) WithEnclosure(url string, length int64, mime string) *ItemBuilder {
	url = strings.TrimSpace(url)
	mime = strings.TrimSpace(mime)
	if url == "" && mime == "" && length <= 0 {
		b.item.Enclosure = nil
		return b
	}
	b.item.Enclosure = &Enclosure{Url: url, Length: length, Type: mime}
	return b
}

// WithDurationSeconds sets the item duration (seconds) for PSP and JSON attachments.
func (b *ItemBuilder) WithDurationSeconds(sec int) *ItemBuilder {
	if sec < 0 {
		sec = 0
	}
	b.item.DurationSeconds = sec
	return b
}

/*
WithExtensions appends raw extension nodes at item/entry scope.
This is the single way to add target-specific elements using the builder.
*/
func (b *ItemBuilder) WithExtensions(nodes ...ExtensionNode) *ItemBuilder {
	if len(nodes) == 0 {
		return b
	}
	b.item.Extensions = append(b.item.Extensions, nodes...)
	return b
}

// Build finalizes the item with minimal strict checks:
// - title or description must be present in strict mode
func (b *ItemBuilder) Build() (*Item, error) {
	if b.strict {
		if strings.TrimSpace(b.item.Title) == "" && strings.TrimSpace(b.item.Description) == "" {
			return nil, errors.New("builder: item requires a title or description")
		}
		if b.item.Enclosure != nil {
			// enclosure length/type/url must be valid for compliant RSS/PSP
			if strings.TrimSpace(b.item.Enclosure.Url) == "" || strings.TrimSpace(b.item.Enclosure.Type) == "" || b.item.Enclosure.Length <= 0 {
				return nil, errors.New("builder: item enclosure requires url, type and positive length")
			}
		}
	}
	return &b.item, nil
}

// Convenience helpers for rendering using the selected target profiles.

// ToRSS renders the feed to an RSS 2.0 string after validating ProfileRSS.
func ToRSS(feed *Feed) (string, error) {
	if feed == nil {
		return "", errors.New("nil feed")
	}
	return ToXML(&Rss{feed})
}

// ToAtom renders the feed to an Atom 1.0 string after validating ProfileAtom.
func ToAtom(feed *Feed) (string, error) {
	if feed == nil {
		return "", errors.New("nil feed")
	}
	return ToXML(&Atom{feed})
}

// ToPSP renders the feed to a PSP-1 compliant RSS string after validating ProfilePSP.
func ToPSP(feed *Feed) (string, error) {
	if feed == nil {
		return "", errors.New("nil feed")
	}
	return ToXML(&PSP{feed})
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

// String returns a short printable representation of the builder (for debugging).
func (b *FeedBuilder) String() string {
	return fmt.Sprintf("&FeedBuilder{Title:%q, Items:%d, Strict:%v, Profiles:%v}", b.feed.Title, len(b.items), b.strict, b.profiles)
}
