package gofeedx

import (
	"encoding/xml"
	"strings"
)

// UnwrapCDATA removes a single top-level <![CDATA[ ... ]]> wrapper when present.
// Idempotent: returns the original string when no wrapper exists.
func UnwrapCDATA(s string) string {
	if s == "" {
		return s
	}
	trimmed := strings.TrimSpace(s)
	// Literal CDATA wrapper: <![CDATA[ ... ]]>
	if strings.HasPrefix(trimmed, "<![CDATA[") && strings.HasSuffix(trimmed, "]]>") {
		return trimmed[len("<![CDATA[") : len(trimmed)-len("]]>")]
	}
	return s
}

// CData is a string alias used by writer structs. Its MarshalXML intentionally
// avoids any CDATA decisions so that writers can control CDATA emission explicitly
// without relying on package-global state.
type CData string

// MarshalXML encodes as normal element text (escaped as needed).
// Writers should call encodeElementCDATA to control CDATA behavior.
func (c CData) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(string(c), start)
}

// needsCDATA reports whether CDATA is beneficial based on content containing
// characters that would otherwise be escaped (e.g., '<' or '&').
func needsCDATA(s string) bool {
	if s == "" {
		return false
	}
	return strings.ContainsAny(s, "<&")
}

// encodeElementCDATA encodes name=value as an element, emitting CDATA when useCDATA is true
// and the content needs it; otherwise normal character data. Idempotent with already-wrapped input.
func encodeElementCDATA(e *xml.Encoder, name string, value string, useCDATA bool) error {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil
	}
	s = UnwrapCDATA(s)
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if useCDATA && needsCDATA(s) {
		tmp := struct {
			XMLName xml.Name
			Value   string `xml:",cdata"`
		}{
			XMLName: start.Name,
			Value:   s,
		}
		return e.Encode(tmp)
	}
	return e.EncodeElement(s, start)
}

// UseCDATAFromExtensions returns the CDATA preference from a list of extensions.
// Default: true (enabled). Overridden when an "_xml:cdata" node with "true"/"false" is present.
func UseCDATAFromExtensions(exts []ExtensionNode) bool {
	use := true
	for _, n := range exts {
		if strings.EqualFold(strings.TrimSpace(n.Name), "_xml:cdata") {
			t := strings.ToLower(strings.TrimSpace(n.Text))
			if t == "false" {
				return false
			}
			if t == "true" {
				return true
			}
		}
	}
	return use
}

// CDATAUseForItem returns the CDATA preference for an item scope, deriving from the parent
// when no explicit override exists in the provided extensions.
func CDATAUseForItem(parentUse bool, exts []ExtensionNode) bool {
	for _, n := range exts {
		if strings.EqualFold(strings.TrimSpace(n.Name), "_xml:cdata") {
			return UseCDATAFromExtensions(exts)
		}
	}
	return parentUse
}

// WithCDATAOverride returns a new extensions slice with a CDATA preference override appended.
func WithCDATAOverride(exts []ExtensionNode, use bool) []ExtensionNode {
	out := append([]ExtensionNode{}, exts...)
	val := "true"
	if !use {
		val = "false"
	}
	return append(out, ExtensionNode{Name: "_xml:cdata", Text: val})
}

// UseCDATAForFeed returns the CDATA preference for a feed:
// - Default: true (enabled)
// - Can be overridden by a feed-level extension node: Name "_xml:cdata" with Text "true"/"false"
func UseCDATAForFeed(f *Feed) bool {
	if f == nil {
		return true
	}
	return UseCDATAFromExtensions(f.Extensions)
}
