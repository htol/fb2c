// Package encoding provides character encoding detection for FB2 files.
//
// It handles BOM detection, XML/HTML encoding declarations, and provides
// robust fallback mechanisms for malformed files.
package encoding

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Canonical encoding names shared by the alias map, the BOM table and the
// decoder switch — the three structures must agree, constants make that
// contract compile-checked.
const (
	encUTF8    = "utf-8"
	encUTF16LE = "utf-16le"
	encUTF16BE = "utf-16be"
	encCP1250  = "cp1250"
	encCP1251  = "cp1251"
	encCP1252  = "cp1252"
	encKOI8R   = "koi8-r"
	encGBK     = "gbk"
)

// Common encoding aliases
var encodingAliases = map[string]string{
	"macintosh":         "mac-roman",
	"x-sjis":            "shift-jis",
	"mac-centraleurope": encCP1250,
	"gb2312":            encGBK, // Microsoft Word bug workaround
	"chinese":           encGBK,
	"csiso58gb231280":   encGBK,
	"euc-cn":            encGBK,
	"euccn":             encGBK,
	"eucgb2312-cn":      encGBK,
	"gb2312-1980":       encGBK,
	"gb2312-80":         encGBK,
	"iso-ir-58":         encGBK,
	"ascii":             encUTF8,
	// Windows codepages
	"windows-1250": encCP1250,
	"windows-1251": encCP1251,
	"windows-1252": encCP1252,
	"cp1250":       encCP1250,
	"cp1251":       encCP1251,
	"cp1252":       encCP1252,
	// KOI8-R (Russian)
	"koi8-r":  encKOI8R,
	"koi8r":   encKOI8R,
	"cskoi8r": encKOI8R,
}

// BOM markers for different encodings
var boms = []struct {
	bom      []byte
	encoding string
}{
	{[]byte{0xEF, 0xBB, 0xBF}, encUTF8},          // UTF-8
	{[]byte{0xFF, 0xFE}, encUTF16LE},             // UTF-16 LE
	{[]byte{0xFE, 0xFF}, encUTF16BE},             // UTF-16 BE
	{[]byte{0xFF, 0xFE, 0x00, 0x00}, "utf-32le"}, // UTF-32 LE
	{[]byte{0x00, 0x00, 0xFE, 0xFF}, "utf-32be"}, // UTF-32 BE
}

// Regex patterns for encoding declarations
var encodingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`<\?[^<>]+encoding\s*=\s*['"]([^'"]+)['"][^<>]*\?>`),                                   // XML declaration
	regexp.MustCompile(`(?i)<meta\s+charset=['"]([^'"]+)['"][^<>]*>`),                                         // HTML5 charset
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
		return &DetectResult{Encoding: encUTF8, Confidence: 0.5}
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

	if enc := findEncodingDeclaration(prefix); enc != "" {
		normalized := normalizeEncoding(enc)
		return &DetectResult{
			Encoding:   normalized,
			Confidence: 0.9,
			Declared:   true,
		}
	}

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

	if alias, ok := encodingAliases[enc]; ok {
		return alias
	}

	enc = strings.ReplaceAll(enc, "utf8", "utf-8")
	enc = strings.ReplaceAll(enc, "utf16", "utf-16")
	enc = strings.ReplaceAll(enc, "_", "-") // Standardize on hyphens

	return enc
}

// detectHeuristic uses heuristics to detect encoding when no declaration is found.
func detectHeuristic(raw []byte) *DetectResult {
	if utf8.Valid(raw) {
		return &DetectResult{
			Encoding:   encUTF8,
			Confidence: 0.8,
		}
	}

	if looksLikeUTF16LE(raw) {
		return &DetectResult{
			Encoding:   encUTF16LE,
			Confidence: 0.6,
		}
	}

	if looksLikeUTF16BE(raw) {
		return &DetectResult{
			Encoding:   encUTF16BE,
			Confidence: 0.6,
		}
	}

	return &DetectResult{
		Encoding:   encUTF8,
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
	return float64(nullCount)/float64(encMin(len(data), 100)/2) > 0.7
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
	return float64(nullCount)/float64(encMin(len(data), 100)/2) > 0.7
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

	if enc == encUTF8 || enc == "utf8" {
		if !utf8.Valid(raw) {
			return strings.ToValidUTF8(string(raw), "�"), nil
		}
		return string(raw), nil
	}

	switch enc {
	case encUTF16LE, "utf16le", "utf-16-le":
		return decodeUTF16(raw, unicode.LittleEndian)
	case encUTF16BE, "utf16be", "utf-16-be":
		return decodeUTF16(raw, unicode.BigEndian)
	}

	var encoding encoding.Encoding
	switch enc {
	case encCP1250:
		encoding = charmap.Windows1250
	case encCP1251:
		encoding = charmap.Windows1251
	case encCP1252:
		encoding = charmap.Windows1252
	case encKOI8R:
		encoding = charmap.KOI8R
	default:
		// For other encodings, return an error
		return "", fmt.Errorf("unsupported encoding: %s (you may need to add encoding support)", enc)
	}

	decoder := encoding.NewDecoder()
	result, err := decoder.Bytes(raw)
	if err != nil {
		return decodeWithReplacement(raw, decoder)
	}

	return string(result), nil
}

// decodeWithReplacement decodes data replacing unconvertible characters
func decodeWithReplacement(data []byte, transformer transform.Transformer) (string, error) {
	// Transform with replacement
	result, _, err := transform.Bytes(transformer, data)
	if err != nil {
		return "", fmt.Errorf("encoding conversion failed: %w", err)
	}
	return string(result), nil
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
			if len(matches) > 1 && !strings.EqualFold(matches[1], newEncoding) {
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

	if len(prefix) > 5 && bytes.HasPrefix(prefix, []byte("<?xml")) {
		end := bytes.Index(prefix, []byte("?>"))
		if end > 0 {
			decl := string(prefix[:end+2])
			_ = xml.NewDecoder(bytes.NewReader([]byte(decl)))
			if enc := findEncodingDeclaration([]byte(decl)); enc != "" {
				return normalizeEncoding(enc)
			}
		}
	}

	return ""
}

// encMin returns the minimum of two integers.
func encMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
