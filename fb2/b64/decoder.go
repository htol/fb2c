// Package b64 provides robust base64 decoding for FB2 files.
//
// FB2 files often contain malformed base64 data. This decoder handles
// invalid characters gracefully by skipping them, similar to FBReader.
package b64

import (
	"encoding/base64"
	"errors"
)

var (
	ErrInvalidData = errors.New("b64: invalid base64 data")
)

// Decode decodes base64 data, handling invalid characters gracefully.
// It first tries the standard decoder, then falls back to a more robust
// FBReader-compatible algorithm that skips invalid characters.
func Decode(raw []byte) ([]byte, error) {
	// Try standard base64 first (faster)
	std, err := base64.StdEncoding.DecodeString(string(raw))
	if err == nil {
		return std, nil
	}

	// Fall back to robust decoder
	return robustDecode(raw)
}

// robustDecode implements FBReader-compatible base64 decoding.
// It skips invalid characters instead of failing.
func robustDecode(raw []byte) ([]byte, error) {
	var out []byte
	var quad [4]byte
	var quadPos int

	for _, b := range raw {
		val, ok := decodeByte(b)
		if !ok {
			// Skip invalid character
			continue
		}

		// Handle padding (=)
		if val == 64 {
			// Padding completes the current quad
			if quadPos > 0 {
				for i := quadPos; i < 4; i++ {
					quad[i] = 0 // Use 0 for padding positions
				}

				// Decode the quad
				triple := (uint32(quad[0]) << 18) | (uint32(quad[1]) << 12) |
					(uint32(quad[2]) << 6) | uint32(quad[3])

				// Output bytes based on how many chars we had before padding
				if quadPos >= 2 {
					out = append(out, byte(triple>>16))
				}
				if quadPos >= 3 {
					out = append(out, byte(triple>>8))
				}
			}
			return out, nil
		}

		quad[quadPos] = val
		quadPos++

		if quadPos == 4 {
			// Decode complete quad
			triple := (uint32(quad[0]) << 18) | (uint32(quad[1]) << 12) |
				(uint32(quad[2]) << 6) | uint32(quad[3])

			out = append(out, byte(triple>>16))
			out = append(out, byte(triple>>8))
			out = append(out, byte(triple))

			quadPos = 0
		}
	}

	// Handle remaining partial quad without padding
	if quadPos > 0 {
		// Pad with zeros
		for i := quadPos; i < 4; i++ {
			quad[i] = 0
		}

		triple := (uint32(quad[0]) << 18) | (uint32(quad[1]) << 12) |
			(uint32(quad[2]) << 6) | uint32(quad[3])

		if quadPos >= 2 {
			out = append(out, byte(triple>>16))
		}
		if quadPos >= 3 {
			out = append(out, byte(triple>>8))
		}
	}

	return out, nil
}

// decodeByte converts a base64 character to its 6-bit value.
// Returns false for invalid characters.
func decodeByte(b byte) (byte, bool) {
	switch {
	case 'A' <= b && b <= 'Z':
		return b - 'A', true
	case 'a' <= b && b <= 'z':
		return b - 'a' + 26, true
	case '0' <= b && b <= '9':
		return b - '0' + 52, true
	case b == '+':
		return 62, true
	case b == '/':
		return 63, true
	case b == '=':
		return 64, true // Padding marker
	default:
		return 0, false // Invalid character
	}
}

// Encode encodes data to base64 using standard encoding.
func Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
