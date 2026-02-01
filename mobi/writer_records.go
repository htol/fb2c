package mobi

// splitAndCompressRecords splits text data into ~4096-byte chunks, ensuring UTF-8 characters
// are not split across record boundaries, and optionally compresses them
func (w *Writer) splitAndCompressRecords(textData []byte) [][]byte {
	var textRecords [][]byte
	const recordSize = 4096

	i := 0
	for i < len(textData) {
		end := i + recordSize
		if end > len(textData) {
			end = len(textData)
		}

		// Adjust end to avoid splitting UTF-8 multibyte characters
		end = findUTF8SafeBoundary(textData, i, end)

		chunk := textData[i:end]
		record := chunk
		if w.options.CompressionType == PalmDOCCompression {
			record = compressRecord(chunk)
		}

		// No trailing bytes - ExtraRecordFlags=0 means no extra data at end of records
		textRecords = append(textRecords, record)

		i = end
	}
	return textRecords
}

// findUTF8SafeBoundary adjusts the end position to avoid splitting UTF-8 multibyte characters.
// It scans backwards from end to find a valid UTF-8 character boundary.
func findUTF8SafeBoundary(data []byte, start, end int) int {
	if end >= len(data) {
		return end
	}

	// Check the byte at position end-1 (last byte we'd include)
	// UTF-8 encoding:
	// - 0x00-0x7F: single byte (ASCII) - safe to split after
	// - 0xC0-0xDF: 2-byte sequence lead - must have 1 more byte
	// - 0xE0-0xEF: 3-byte sequence lead - must have 2 more bytes
	// - 0xF0-0xF7: 4-byte sequence lead - must have 3 more bytes
	// - 0x80-0xBF: continuation byte - must find the lead byte

	lastByte := data[end-1]

	// ASCII - safe to split here
	if lastByte <= 0x7F {
		return end
	}

	// If last byte is a lead byte, we can't fit the full character - back up
	if lastByte >= 0xC0 {
		return end - 1
	}

	// Last byte is a continuation byte (0x80-0xBF)
	// Scan back to find the lead byte and check if the full char fits
	for i := 1; i <= 3 && end-1-i >= start; i++ {
		b := data[end-1-i]

		if b >= 0xC0 && b <= 0xDF {
			// 2-byte lead at position end-1-i
			// Character spans [end-1-i, end-i] (2 bytes)
			// We need end to be at end-i+1 or later to include it
			if i >= 1 { // We have at least 2 bytes: lead + 1 continuation
				return end // The char fits
			}
			return end - 1 - i // Exclude the partial char
		} else if b >= 0xE0 && b <= 0xEF {
			// 3-byte lead
			if i >= 2 { // We have 3 bytes
				return end
			}
			return end - 1 - i
		} else if b >= 0xF0 && b <= 0xF7 {
			// 4-byte lead
			if i >= 3 { // We have 4 bytes
				return end
			}
			return end - 1 - i
		}
		// b is 0x80-0xBF, keep scanning back
	}

	// Couldn't find lead in 4 bytes back - malformed UTF-8, just back up 1
	return end - 1
}
