// Package fb2encoding provides character encoding detection for FB2 files.
//
// It handles BOM detection, XML/HTML encoding declarations, and provides
// robust fallback mechanisms for malformed files.
package fb2encoding

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/unicode"
)

// Common encoding aliases
var encodingAliases = map[string]string{
	"macintosh":        "mac-roman",
	"x-sjis":           "shift-jis",
	"mac-centraleurope": "cp1250",
	"gb2312":           "gbk", // Microsoft Word bug workaround
	"chinese":          "gbk",
	"csiso58gb231280":  "gbk",
	"euc-cn":           "gbk",
	"euccn":            "gbk",
	"eucgb2312-cn":     "gbk",
	"gb2312-1980":      "gbk",
	"gb2312-80":        "gbk",
	"iso-ir-58":        "gbk",
	"ascii":            "utf-8",
}

// BOM markers for different encodings
var boms = []struct {
	bom      []byte
	encoding string
}{
	{[]byte{0xEF, 0xBB, 0xBF}, "utf-8"},        // UTF-8
	{[]byte{0xFF, 0xFE}, "utf-16le"},          // UTF-16 LE
	{[]byte{0xFE, 0xFF}, "utf-16be"},          // UTF-16 BE
	{[]byte{0xFF, 0xFE, 0x00, 0x00}, "utf-32le"}, // UTF-32 LE
	{[]byte{0x00, 0x00, 0xFE, 0xFF}, "utf-32be"}, // UTF-32 BE
}

// Regex patterns for encoding declarations
var encodingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`<\?[^<>]+encoding\s*=\s*['"]([^'"]+)['"][^<>]*\?>`), // XML declaration
	regexp.MustCompile(`(?i)<meta\s+charset=['"]([^'"]+)['"][^<>]*>`),         // HTML5 charset
	regexp.MustCompile(`(?i)<meta\s+?[^<>]*?content\s*=\s*['"][^'"]*?charset=([^'\">]+)[^'\">]*?['"][^<>]*>`), // HTML4 pragma
}

// DetectResult contains the detected encoding and confidence level.
type DetectResult struct {
	Encoding   string
	Confidence float64 // 0.0 to 1.0
	BOM        bool
	Declared   bool // From XML/HTML declaration
}

// Detect detects the character encoding of raw bytes.
// It checks BOM, XML/HTML declarations, and falls back to heuristics.
func Detect(raw []byte) *DetectResult {
	if len(raw) == 0 {
		return &DetectResult{Encoding: "utf-8", Confidence: 0.5}
	}

	// Check for BOM
	for _, bom := range boms {
		if bytes.HasPrefix(raw, bom.bom) {
			return &DetectResult{
				Encoding:   bom.encoding,
				Confidence: 1.0,
				BOM:        true,
			}
		}
	}

	// Look for encoding declaration in first 50KB
	prefix := raw
	if len(prefix) > 50*1024 {
		prefix = prefix[:50*1024]
	}

	// Try to find XML/HTML encoding declaration
	if enc := findEncodingDeclaration(prefix); enc != "" {
		normalized := normalizeEncoding(enc)
		return &DetectResult{
			Encoding:   normalized,
			Confidence: 0.9,
			Declared:   true,
		}
	}

	// Fallback: try to detect heuristically
	return detectHeuristic(raw)
}

// findEncodingDeclaration searches for encoding in XML/HTML declarations.
func findEncodingDeclaration(data []byte) string {
	for _, pat := range encodingPatterns {
		matches := pat.FindSubmatch(data)
		if len(matches) > 1 {
			return string(matches[1])
		}
	}
	return ""
}

// normalizeEncoding converts encoding names to canonical form.
func normalizeEncoding(enc string) string {
	enc = strings.ToLower(strings.TrimSpace(enc))

	// Check aliases first
	if alias, ok := encodingAliases[enc]; ok {
		return alias
	}

	// Normalize common variations
	enc = strings.ReplaceAll(enc, "utf8", "utf-8")
	enc = strings.ReplaceAll(enc, "utf16", "utf-16")
	enc = strings.ReplaceAll(enc, "_", "-")  // Standardize on hyphens

	return enc
}

// detectHeuristic uses heuristics to detect encoding when no declaration is found.
func detectHeuristic(raw []byte) *DetectResult {
	// If all bytes are valid UTF-8, assume UTF-8
	if utf8.Valid(raw) {
		return &DetectResult{
			Encoding:   "utf-8",
			Confidence: 0.8,
		}
	}

	// Check for UTF-16 LE/BE patterns
	if looksLikeUTF16LE(raw) {
		return &DetectResult{
			Encoding:   "utf-16le",
			Confidence: 0.6,
		}
	}

	if looksLikeUTF16BE(raw) {
		return &DetectResult{
			Encoding:   "utf-16be",
			Confidence: 0.6,
		}
	}

	// Last resort: assume UTF-8 with replacement
	return &DetectResult{
		Encoding:   "utf-8",
		Confidence: 0.3,
	}
}

// looksLikeUTF16LE checks if data looks like UTF-16 Little Endian.
func looksLikeUTF16LE(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	// UTF-16 LE should have even length and zero bytes at odd positions for ASCII
	if len(data)%2 != 0 {
		return false
	}
	// Check if most odd positions are null bytes (typical for ASCII in UTF-16 LE)
	nullCount := 0
	for i := 1; i < len(data) && i < 100; i += 2 {
		if data[i] == 0 {
			nullCount++
		}
	}
	return float64(nullCount)/float64(min(len(data), 100)/2) > 0.7
}

// looksLikeUTF16BE checks if data looks like UTF-16 Big Endian.
func looksLikeUTF16BE(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if len(data)%2 != 0 {
		return false
	}
	// Check if most even positions are null bytes (typical for ASCII in UTF-16 BE)
	nullCount := 0
	for i := 0; i < len(data) && i < 100; i += 2 {
		if data[i] == 0 {
			nullCount++
		}
	}
	return float64(nullCount)/float64(min(len(data), 100)/2) > 0.7
}

// ToUTF8 converts raw bytes to a UTF-8 string using the detected encoding.
func ToUTF8(raw []byte) (string, error) {
	result := Detect(raw)
	return toUTF8WithEncoding(raw, result.Encoding)
}

// ToUTF8WithStrip converts raw bytes to UTF-8, stripping encoding declarations.
func ToUTF8WithStrip(raw []byte, stripPatterns bool) (string, string, error) {
	result := Detect(raw)

	str, err := toUTF8WithEncoding(raw, result.Encoding)
	if err != nil {
		return "", "", err
	}

	if stripPatterns {
		str = stripEncodingDeclarations(str)
	}

	return str, result.Encoding, nil
}

// toUTF8WithEncoding converts raw bytes to UTF-8 using a specific encoding.
func toUTF8WithEncoding(raw []byte, enc string) (string, error) {
	// Remove BOM if present
	for _, bom := range boms {
		if bytes.HasPrefix(raw, bom.bom) {
			raw = raw[len(bom.bom):]
			break
		}
	}

	// Handle UTF-8 directly
	if enc == "utf-8" || enc == "utf8" {
		if !utf8.Valid(raw) {
			// Use replacement character for invalid UTF-8
			return strings.ToValidUTF8(string(raw), "�"), nil
		}
		return string(raw), nil
	}

	// For UTF-16 variants, use Go's unicode package
	switch enc {
	case "utf-16le", "utf16le", "utf-16-le":
		return decodeUTF16(raw, unicode.LittleEndian)
	case "utf-16be", "utf16be", "utf-16-be":
		return decodeUTF16(raw, unicode.BigEndian)
	}

	// For other encodings, we'd typically use golang.org/x/text/encoding
	// For now, return an error for unsupported encodings
	return "", fmt.Errorf("unsupported encoding: %s (you may need to add encoding support)", enc)
}

// decodeUTF16 decodes UTF-16 data with specified byte order.
func decodeUTF16(data []byte, bo unicode.Endianness) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("invalid UTF-16 data: odd length")
	}

	// Use UTF16 decoder from golang.org/x/text
	decoder := unicode.UTF16(bo, unicode.UseBOM).NewDecoder()

	result, err := decoder.Bytes(data)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// StripEncodingDeclarations removes encoding declarations from XML/HTML.
func StripEncodingDeclarations(data string) string {
	return stripEncodingDeclarations(data)
}

// stripEncodingDeclarations removes XML/HTML encoding declarations.
func stripEncodingDeclarations(data string) string {
	for _, pat := range encodingPatterns {
		data = pat.ReplaceAllString(data, "")
	}
	return data
}

// ReplaceEncodingInDeclaration replaces encoding in XML/HTML declarations.
func ReplaceEncodingInDeclaration(data string, newEncoding string) (string, bool) {
	changed := false
	for _, pat := range encodingPatterns {
		newData := pat.ReplaceAllStringFunc(data, func(match string) string {
			// Check if encoding is different
			matches := pat.FindStringSubmatch(match)
			if len(matches) > 1 && strings.ToLower(matches[1]) != strings.ToLower(newEncoding) {
				changed = true
				return strings.Replace(match, matches[1], newEncoding, 1)
			}
			return match
		})
		data = newData
	}
	return data, changed
}

// FindXMLEncoding extracts the encoding from an XML declaration.
func FindXMLEncoding(data []byte) string {
	prefix := data
	if len(prefix) > 1024 {
		prefix = prefix[:1024]
	}

	// Parse XML declaration
	if len(prefix) > 5 && bytes.HasPrefix(prefix, []byte("<?xml")) {
		// Find the end of the declaration
		end := bytes.Index(prefix, []byte("?>"))
		if end > 0 {
			decl := string(prefix[:end+2])
			// Try to parse as XML
			_ = xml.NewDecoder(bytes.NewReader([]byte(decl)))
			// The XML parser will handle the encoding declaration
			// We just need to extract it
			if enc := findEncodingDeclaration([]byte(decl)); enc != "" {
				return normalizeEncoding(enc)
			}
		}
	}

	return ""
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
