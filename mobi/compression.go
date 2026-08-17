// Package mobi provides PalmDOC compression.
package mobi

import (
	"bytes"
)

// PalmDOC compression uses LZ77-style compression with special encodings

// CompressPalmDOC compresses data using PalmDOC compression
func CompressPalmDOC(data []byte) []byte {
	var output bytes.Buffer

	// Process data in 4096-byte records (except last which may be smaller)
	for i := 0; i < len(data); i += 4096 {
		end := i + 4096
		if end > len(data) {
			end = len(data)
		}
		record := data[i:end]

		compressed := compressRecord(record)
		output.Write(compressed)

		// Add trailing overlap byte for non-final records
		if end < len(data) {
			// The last byte of each record is duplicated as first byte of next
			output.WriteByte(record[len(record)-1])
		}
	}

	return output.Bytes()
}

// compressRecord compresses a single record (max 4096 bytes uncompressed)
func compressRecord(data []byte) []byte {
	var output bytes.Buffer

	pos := 0
	for pos < len(data) {
		// Try LZ77 compression first (look for 3-10 byte repeats within 2047 bytes)
		if match := findLZMatch(data, pos); match.length >= 3 {
			// Encoding: bit 15=1, bits 14-4=distance (11 bits), bits 2-0=length-3 (3 bits)
			code := uint16(0x8000) | uint16((match.distance&0x07FF)<<3) | uint16(match.length-3) //nolint:gosec // Values guaranteed to fit by LZ77 constraints

			// Write big-endian
			output.WriteByte(byte(code >> 8))
			output.WriteByte(byte(code & 0xFF))

			pos += match.length
			continue
		}

		// Check for space (0x20) followed by char 0x40-0x7F
		if pos+1 < len(data) && data[pos] == 0x20 && data[pos+1] >= 0x40 && data[pos+1] <= 0x7F {
			// Encode as: char ^ 0x80
			output.WriteByte(data[pos+1] ^ 0x80)
			pos += 2
			continue
		}

		// Literal byte
		output.WriteByte(data[pos])
		pos++
	}

	return output.Bytes()
}

// lzMatch represents an LZ77 match
type lzMatch struct {
	distance int
	length   int
}

// findLZMatch looks for repeated sequences within the lookback window
func findLZMatch(data []byte, pos int) lzMatch {
	// Max lookback distance: 2047 bytes
	// Max match length: 10 bytes
	// Min match length: 3 bytes

	const (
		maxDistance = 2047
		maxMatchLen = 10
		minMatchLen = 3
	)

	// Can't match before start of data
	if pos == 0 {
		return lzMatch{}
	}

	// Calculate lookback start
	lookbackStart := pos - maxDistance
	if lookbackStart < 0 {
		lookbackStart = 0
	}

	// Find the best match (prefer longest, then closest)
	var bestMatch lzMatch

	for length := maxMatchLen; length >= minMatchLen; length-- {
		// Need enough data ahead
		if pos+length > len(data) {
			continue
		}

		// Search for this sequence in lookback window
		target := data[pos : pos+length]

		for i := lookbackStart; i < pos; i++ {
			// Check if we can match at position i
			if i+length > pos {
				continue // Can't overlap current position
			}

			// Check for match
			if bytes.Equal(data[i:i+length], target) {
				distance := pos - i
				if distance > maxDistance {
					continue
				}

				// Found a match - this is the best for this length
				if length > bestMatch.length {
					bestMatch = lzMatch{
						distance: distance,
						length:   length,
					}
					break // Don't need to check more positions for this length
				}
			}
		}

		if bestMatch.length > 0 {
			return bestMatch
		}
	}

	return lzMatch{}
}

// CompressionRatio returns the compressed/original size ratio.
func CompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	return float64(len(compressed)) / float64(len(original))
}
