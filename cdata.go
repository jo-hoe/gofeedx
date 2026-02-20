package gofeedx

import (
	"encoding/xml"
	"strings"
)

// Global toggle for emitting CDATA when encoding XML content.
// Default: enabled (true) following common patterns where rich text fields
// are wrapped in CDATA to avoid escaping HTML.
var xmlCDATAEnabled = true

// SetXMLCDATAEnabled toggles whether CDATA is emitted for eligible text fields.
func SetXMLCDATAEnabled(enabled bool) {
	xmlCDATAEnabled = enabled
}

// XMLCDATAEnabled reports whether CDATA emission is currently enabled.
func XMLCDATAEnabled() bool {
	return xmlCDATAEnabled
}

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
	// HTML-escaped CDATA wrapper (rare): <![CDATA[ ... ]]>
	if strings.HasPrefix(trimmed, "<![CDATA[") && strings.HasSuffix(trimmed, "]]>") {
		return trimmed[len("<![CDATA[") : len(trimmed)-len("]]>")]
	}
	return s
}

// CData encodes its value as element text, using CDATA when enabled.
// Use this for fields that may contain rich text (HTML) to avoid escaping.
type CData string

// needsCDATA decides if CDATA is beneficial based on content containing characters
// that would otherwise be escaped (e.g., '<' or '&').
func needsCDATA(s string) bool {
	if s == "" {
		return false
	}
	return strings.ContainsAny(s, "<&")
}

// MarshalXML emits the element using CDATA when enabled and content needs it,
// otherwise normal character data.
func (c CData) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	value := UnwrapCDATA(string(c))
	if XMLCDATAEnabled() && needsCDATA(value) {
		// Encode element with CDATA content via a temporary struct that uses the ",cdata" tag.
		tmp := struct {
			XMLName xml.Name
			Value   string `xml:",cdata"`
		}{
			XMLName: start.Name,
			Value:   value,
		}
		return e.Encode(tmp)
	}
	// Fallback to normal element text
	return e.EncodeElement(value, start)
}

// encodeElementCDATA encodes name=value as an element. It delegates to CharOrCDATA
// so that when CDATA is enabled (and content needs it), CDATA is emitted; otherwise
// normal character data is emitted. Idempotent with already-wrapped CDATA input.
func encodeElementCDATA(e *xml.Encoder, name string, value string) error {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil
	}
	s = UnwrapCDATA(s)
	return e.EncodeElement(CData(s), xml.StartElement{Name: xml.Name{Local: name}})
}
