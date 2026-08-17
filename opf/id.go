// Deterministic per-book identifiers shared by the output writers.
package opf

import (
	"crypto/sha1" //nolint:gosec // UUIDv5 mandates SHA-1; not used for security
	"fmt"
	"strings"
)

// fb2cNamespace is the UUIDv5 namespace for fb2c-generated identifiers:
// UUIDv5(NAMESPACE_DNS, "github.com/htol/fb2c"). Fixed so book IDs are stable
// across fb2c releases.
var fb2cNamespace = mustParseUUID("47c7651a-de61-58d9-ac07-568c89c97043")

func mustParseUUID(s string) [16]byte {
	var u [16]byte
	hex := strings.ReplaceAll(s, "-", "")
	for i := 0; i < 16; i++ {
		u[i] = hexByte(hex[2*i])<<4 | hexByte(hex[2*i+1])
	}
	return u
}

func hexByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return 0
	}
}

// normalizeIDName normalizes a string for use in identifier derivation:
// case-insensitive and insensitive to whitespace runs.
func normalizeIDName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// BookID returns a UUIDv5 identifier for the book: SHA-1 of the
// fb2c namespace plus the normalized title and authors. Deterministic for the
// same book, distinct between different books, so output is byte-stable.
// EPUB uses it as the package identifier (urn:uuid: form); MOBI mirrors it
// in EXTH 113 (ASIN), where the reference file carries a per-book UUID.
func BookID(book *OEBBook) string {
	var name strings.Builder
	name.WriteString(normalizeIDName(book.Metadata.Title))
	for _, author := range book.Metadata.Authors {
		name.WriteString("\x1f") // field separator: not produced by normalizeIDName
		name.WriteString(normalizeIDName(author.FullName))
	}

	h := sha1.New() //nolint:gosec // UUIDv5 mandates SHA-1; not used for security
	h.Write(fb2cNamespace[:])
	h.Write([]byte(name.String()))
	sum := h.Sum(nil)

	// Set UUID version 5 and RFC 4122 variant bits
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
