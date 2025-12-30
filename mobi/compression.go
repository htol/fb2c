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
			// Encode as: 0x8000 + ((distance & 0x3FFF) << 3) + (length - 3)
			code := uint16(0x8000)
			code |= uint16((match.distance & 0x3FFF) << 3)
			code |= uint16(match.length - 3)

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

		// Check for binary sequences
		if seq := findBinarySequence(data, pos); seq.length > 0 {
			// Encode as: 0x8001 + ((length - 1) << 8) + byte
			code := uint16(0x8001)
			code |= uint16(seq.length-1) << 8
			code |= uint16(seq.byteValue)

			output.WriteByte(byte(code >> 8))
			output.WriteByte(byte(code & 0xFF))

			pos += seq.length
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

// binarySeq represents a binary sequence match
type binarySeq struct {
	length    int
	byteValue byte
}

// findBinarySequence looks for repeated binary sequences
func findBinarySequence(data []byte, pos int) binarySeq {
	// Binary sequences: 0x00-0x07 or 0x80-0xFF repeated
	// Encoded as: length 2-8, then the byte value

	const (
		minLen = 2
		maxLen = 8
	)

	if pos >= len(data) {
		return binarySeq{}
	}

	firstByte := data[pos]

	// Check if it's a binary sequence byte (0x00-0x07 or 0x80-0xFF)
	if (firstByte >= 0x00 && firstByte <= 0x07) || (firstByte >= 0x80 && firstByte <= 0xFF) {
		// Count repeats
		length := 1
		for pos+length < len(data) && data[pos+length] == firstByte && length < maxLen {
			length++
		}

		if length >= minLen {
			return binarySeq{
				length:    length,
				byteValue: firstByte,
			}
		}
	}

	return binarySeq{}
}

// DecompressPalmDOC decompresses PalmDOC-compressed data
// Note: This is a simplified implementation for testing
func DecompressPalmDOC(data []byte) []byte {
	var output bytes.Buffer

	pos := 0
	for pos < len(data) {
		byte1 := data[pos]
		pos++

		if byte1 == 0x80 || (byte1&0x80) != 0 {
			// Compressed sequence
			if pos >= len(data) {
				break
			}

			byte2 := data[pos]
			pos++

			// Combine to 16-bit value
			code := uint16(byte1)<<8 | uint16(byte2)

			if code == 0x8000 {
				// LZ77 match - need next byte for full offset
				if pos >= len(data) {
					break
				}
				// This is a placeholder - full decompression would be more complex
				output.WriteByte('?')
			} else if code == 0x8001 {
				// Binary sequence - need next byte
				if pos >= len(data) {
					break
				}
				length := int((code >> 8) & 0x3F)
				val := data[pos]
				pos++
				for i := 0; i < length; i++ {
					output.WriteByte(val)
				}
			} else if byte1&0x80 != 0 && byte2 != 0x00 {
				// Space + char compression: char ^ 0x80
				output.WriteByte(' ')
				output.WriteByte(byte2 ^ 0x80)
			} else {
				// Literal
				output.WriteByte(byte1)
			}
		} else {
			// Literal byte
			output.WriteByte(byte1)
		}
	}

	return output.Bytes()
}

// CompressRecord compresses a record and returns it, possibly using multiple compression methods
func CompressRecord(data []byte, method int) []byte {
	// method: 0 = none, 1 = PalmDOC, 2 = Huff/CD
	switch method {
	case 1:
		return CompressPalmDOC(data)
	default:
		return data
	}
}

// Calculate compression ratio
func CompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	return float64(len(compressed)) / float64(len(original))
}
