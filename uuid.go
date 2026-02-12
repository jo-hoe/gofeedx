package gofeedx

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
)

// UUID is a 16-byte RFC 4122 universally unique identifier.
type UUID [16]byte

// String returns the canonical RFC 4122 string form: 8-4-4-4-12 lowercase hex.
func (u UUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(u[0])<<24|uint32(u[1])<<16|uint32(u[2])<<8|uint32(u[3]),
		uint16(u[4])<<8|uint16(u[5]),
		uint16(u[6])<<8|uint16(u[7]),
		uint16(u[8])<<8|uint16(u[9]),
		uint64(u[10])<<40|uint64(u[11])<<32|uint64(u[12])<<24|uint64(u[13])<<16|uint64(u[14])<<8|uint64(u[15]),
	)
}

// NewUUIDv4 generates a random (version 4) UUID using crypto/rand.
func NewUUIDv4() (UUID, error) {
	var u UUID
	_, err := rand.Read(u[:])
	if err != nil {
		return UUID{}, err
	}
	// Set version (4) and variant (RFC 4122)
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u, nil
}

// UUIDv5 generates a name-based (version 5) UUID using SHA-1 over the namespace
// UUID and name per RFC 4122 section 4.3.
func UUIDv5(namespace UUID, name []byte) UUID {
	h := sha1.New()
	_, _ = h.Write(namespace[:])
	_, _ = h.Write(name)
	sum := h.Sum(nil)

	var u UUID
	copy(u[:], sum[:16])
	// Set version (5) and variant (RFC 4122)
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

// MustUUIDv4 is a helper that panics if NewUUIDv4 fails (should not happen).
func MustUUIDv4() UUID {
	u, err := NewUUIDv4()
	if err != nil {
		panic(err)
	}
	return u
}