package gofeedx

import (
	"regexp"
	"testing"
)

func TestUUIDv4_StringFormatAndBits(t *testing.T) {
	u, err := NewUUIDv4()
	if err != nil {
		t.Fatalf("NewUUIDv4() error: %v", err)
	}
	// Check string format
	s := u.String()
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(s) {
		t.Errorf("UUID v4 string not in canonical form: %q", s)
	}
	// Check version and variant bits
	if (u[6]>>4)&0x0f != 4 {
		t.Errorf("expected version 4, got byte %02x", u[6])
	}
	if (u[8] & 0xc0) != 0x80 {
		t.Errorf("expected RFC4122 variant, got byte %02x", u[8])
	}
}

func TestUUIDv5_DeterministicAndBits(t *testing.T) {
	ns := PodcastNamespaceUUID
	name := []byte("example.com/podcast.rss")
	u1 := UUIDv5(ns, name)
	u2 := UUIDv5(ns, name)
	if u1 != u2 {
		t.Errorf("UUIDv5 should be deterministic for same namespace+name")
	}
	if (u1[6]>>4)&0x0f != 5 {
		t.Errorf("expected version 5, got byte %02x", u1[6])
	}
	if (u1[8] & 0xc0) != 0x80 {
		t.Errorf("expected RFC4122 variant, got byte %02x", u1[8])
	}
}

func TestMustUUIDv4_StringFormat(t *testing.T) {
	// MustUUIDv4 should return a valid v4 UUID string (and not panic under normal circumstances)
	u := MustUUIDv4()
	s := u.String()
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(s) {
		t.Errorf("MustUUIDv4 string not in canonical v4 form: %q", s)
	}
}
