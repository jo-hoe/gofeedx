package gofeedx

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeFeedURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/":           "example.com",
		"http://example.com/foo/":        "example.com/foo",
		"feed://example.com/bar//":       "example.com/bar",
		"  https://example.com/x/y///  ": "example.com/x/y",
		"example.com/trailing////":       "example.com/trailing",
		"HTTPS://example.com/Cased/":     "HTTPS://example.com/Cased", // TrimPrefix is case-sensitive; URL passed as-is in our normalizer
	}
	for in, want := range cases {
		if got := normalizeFeedURL(in); got != want {
			t.Errorf("normalizeFeedURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComputePodcastGuidMatchesV5OfNormalizedURL(t *testing.T) {
	in := "https://example.com/podcast.rss"
	want := UUIDv5(PodcastNamespaceUUID, []byte(normalizeFeedURL(in))).String()
	got := computePodcastGuid(in)
	if got != want {
		t.Errorf("computePodcastGuid mismatch: got %q, want %q", got, want)
	}
}

func TestFallbackItemGuid_TagAndURN(t *testing.T) {
	// Tag URI path
	it := &Item{
		Link:    &Link{Href: "https://example.com/ep/1"},
		Created: time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC),
	}
	tag := fallbackItemGuid(it)
	if !strings.HasPrefix(tag, "tag:example.com,2024-02-03:/ep/1") {
		t.Errorf("unexpected tag guid: %q", tag)
	}
	// URN fallback when no link/time
	it2 := &Item{}
	urn := fallbackItemGuid(it2)
	if !strings.HasPrefix(urn, "urn:uuid:") {
		t.Errorf("expected urn:uuid: prefix, got %q", urn)
	}
}