// Package varint implements Variable Width Integer encoding/decoding.
//
// Varint encoding uses 7 bits per byte in big-endian format.
// The most significant bit (bit 8) indicates termination.
//
// Forward encoding: MSB is set on the first byte
// Backward encoding: MSB is set on the last byte
package varint

import (
	"errors"
)

var (
	ErrOverflow  = errors.New("varint: value overflow")
	ErrUnderflow = errors.New("varint: data underflow")
)

// EncodeForward encodes a value using forward varint encoding.
// In forward encoding, the MSB is set on the first byte of the result.
func EncodeForward(value uint32) []byte {
	if value == 0 {
		return []byte{0x80}
	}

	var chunks []byte
	for value > 0 {
		chunks = append(chunks, byte(value&0x7F))
		value >>= 7
	}

	// Reverse to get big-endian order
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}

	// Set MSB on first byte the result
	chunks[0] |= 0x80
	return chunks
}

// EncodeBackward encodes a value using backward varint encoding (MOBI style).
// In backward encoding, the MSB is set on the last byte of the result.
func EncodeBackward(value uint32) []byte {
	if value == 0 {
		return []byte{0x80}
	}

	var chunks []byte
	for value > 0 {
		chunks = append(chunks, byte(value&0x7F))
		value >>= 7
	}

	// Reverse to get big-endian order
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}

	// Set MSB on last byte of the result
	chunks[len(chunks)-1] |= 0x80
	return chunks
}

// DecodeForward decodes a value using forward varint encoding.
// Returns the decoded value and the number of bytes consumed.
func DecodeForward(data []byte) (uint32, int, error) {
	if len(data) == 0 {
		return 0, 0, ErrUnderflow
	}

	var bytes []byte

	// Read bytes from data, extract 7 bits each, stop when MSB found
	for _, b := range data {
		bytes = append(bytes, b&0x7F)
		if (b & 0x80) != 0 {
			break
		}
	}

	if len(bytes) == 0 {
		return 0, 0, errors.New("varint: no data")
	}

	// Convert bytes to value (MSB first)
	var value uint32
	for _, b := range bytes {
		value = (value << 7) | uint32(b)
	}

	return value, len(bytes), nil
}

// DecodeBackward decodes a value using backward varint encoding.
// Returns the decoded value and the number of bytes consumed.
func DecodeBackward(data []byte) (uint32, int, error) {
	if len(data) == 0 {
		return 0, 0, ErrUnderflow
	}

	var bytes []byte

	// Reverse the data
	rev := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		rev[i] = data[len(data)-1-i]
	}

	// Read until MSB
	for _, b := range rev {
		bytes = append(bytes, b&0x7F)
		if (b & 0x80) != 0 {
			break
		}
	}

	if len(bytes) == 0 {
		return 0, 0, errors.New("varint: no data")
	}

	// Reverse bytes back (to match Python implementation)
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}

	// Convert bytes to value
	var value uint32
	for _, b := range bytes {
		value = (value << 7) | uint32(b)
	}

	return value, len(bytes), nil
}

// Size returns the number of bytes needed to encode a value.
func Size(value uint32) int {
	if value == 0 {
		return 1
	}
	size := 1
	for value > 0x7F {
		value >>= 7
		size++
	}
	return size
}
