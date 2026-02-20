package gofeedx

import (
	"encoding/xml"
	"sort"
	"strings"
)

// ExtensionNode represents a generic extension element that can be injected into channel/feed
// or item/entry scopes for RSS/PSP/Atom outputs. It avoids external dependencies
// and uses encoding/xml for safe construction.
//
// Notes:
//   - Name may include a prefix (e.g., "itunes:image", "podcast:funding").
//   - Attrs keys may include prefixes as well (e.g., "href", "podcast:role").
//   - Text is encoded as character data (escaped as needed).
//   - Children are encoded recursively.
//   - CDATA and raw inner XML are intentionally not supported by default because
//     encoding/xml does not expose an easy CDATA API. If you need embedded HTML,
//     prefer standard fields (e.g., Content/Description which are supported in
//     RSS via content:encoded and CDATA) or submit a feature request to extend this.
type ExtensionNode struct {
	// Name is the element name, may include a namespace prefix (e.g., "itunes:image").
	Name string
	// Attrs contains element attributes as a map of name -> value. Names may include prefixes.
	Attrs map[string]string
	// Text is text content for the node (escaped).
	Text string
	// Children are nested ExtensionNodes.
	Children []ExtensionNode
}

// MarshalXML implements xml.Marshaler to encode XMLNode as arbitrary XML.
func (n ExtensionNode) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: xml.Name{Local: n.Name},
	}

	// Stable attribute order for deterministic output
	if len(n.Attrs) > 0 {
		keys := make([]string, 0, len(n.Attrs))
		for k := range n.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			start.Attr = append(start.Attr, xml.Attr{
				Name:  xml.Name{Local: k},
				Value: n.Attrs[k],
			})
		}
	}

	// Write start tag
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// Write text content if any
	if n.Text != "" {
		if err := e.EncodeToken(xml.CharData([]byte(n.Text))); err != nil {
			return err
		}
	}

	// Write children
	for _, c := range n.Children {
		if err := e.Encode(c); err != nil {
			return err
		}
	}

	// Close element
	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}
	return e.Flush()
}

// encodeElementIfSet encodes an element <name>value</name> when value is non-empty (after trimming).
func encodeElementIfSet(e *xml.Encoder, name, value string) error {
	if s := strings.TrimSpace(value); s != "" {
		return e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: name}})
	}
	return nil
}

// encodeIntElementIfPositive encodes an element <name>n</name> when n > 0.
func encodeIntElementIfPositive(e *xml.Encoder, name string, n int) error {
	if n > 0 {
		return e.EncodeElement(n, xml.StartElement{Name: xml.Name{Local: name}})
	}
	return nil
}
