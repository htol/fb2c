package mobi

import (
	"testing"
	"unicode/utf8"
)

// TestSplitRecordsPreservesUTF8 verifies that text records don't split UTF-8 multibyte characters
func TestSplitRecordsPreservesUTF8(t *testing.T) {
	// Create test data with Cyrillic text that would cause splits at 4096 boundary
	// Cyrillic characters are 2 bytes in UTF-8 (0xD0-0xD1 + continuation byte)

	// Build a string that would cause a split at byte 4096 in the middle of a character
	// Each Cyrillic char is 2 bytes, so 2048 chars = 4096 bytes exactly
	cyrillicChar := "д" // 0xD0 0xB4 = 2 bytes

	// Create exactly 4096 bytes of Cyrillic (2048 characters)
	var testData []byte
	for i := 0; i < 2048; i++ {
		testData = append(testData, cyrillicChar...)
	}

	// Add one more byte to force a second record, positioned so it splits a character
	// If we add 2047 more chars (4094 bytes) + 1 char, the boundary at 4096 will be mid-char
	testData = append(testData, []byte("дддд")...) // 8 more bytes

	// Total: 4104 bytes = first record gets 4096, second gets 8
	// Since we're all 2-byte chars, byte 4096 could land mid-character

	// For a definitive test, construct data where byte 4095 is a 2-byte lead (0xD0/0xD1)
	// This means the second byte of that char would be at 4096, which gets split

	// Create 4095 ASCII bytes + 1 Cyrillic char (2 bytes) + more data
	testData = make([]byte, 0, 5000)
	for i := 0; i < 4095; i++ {
		testData = append(testData, 'a')
	}
	testData = append(testData, []byte("д")...) // Bytes at position 4095, 4096
	testData = append(testData, []byte("more text here")...)

	// Now byte 4095 is 0xD0 (first byte of 'д'), byte 4096 is 0xB4 (second byte)
	// Splitting at 4096 would put 0xD0 alone in record 1 - INVALID UTF-8!

	records := splitTextRecords(testData)

	if len(records) < 2 {
		t.Fatalf("Expected at least 2 records, got %d", len(records))
	}

	// Check each record ends with valid UTF-8 (no split characters)
	for i, rec := range records {
		// No trailing bytes to remove - records are pure text
		textPortion := rec

		// The text portion should be valid UTF-8
		if !utf8.Valid(textPortion) {
			t.Errorf("Record %d text portion is not valid UTF-8 (character was split)", i)

			// Show the problematic bytes at the end
			if len(textPortion) > 5 {
				t.Errorf("Last 5 bytes of record %d: %x", i, textPortion[len(textPortion)-5:])
			}
		}

		// Additionally, check that the last byte is not a UTF-8 lead byte
		if len(textPortion) > 0 {
			lastByte := textPortion[len(textPortion)-1]
			if lastByte >= 0xC0 && lastByte < 0xF8 {
				t.Errorf("Record %d ends with UTF-8 lead byte 0x%02x (indicates split character)", i, lastByte)
			}
		}
	}
}

// TestSplitRecordsUTF8EdgeCases tests specific UTF-8 boundary scenarios
func TestSplitRecordsUTF8EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		makeData func() []byte
	}{
		{
			name: "2-byte char at boundary",
			makeData: func() []byte {
				// 4095 ASCII + 2-byte Cyrillic = split at byte 4096
				data := make([]byte, 4095)
				for i := range data {
					data[i] = 'x'
				}
				return append(data, []byte("дtest")...)
			},
		},
		{
			name: "3-byte char at boundary",
			makeData: func() []byte {
				// 4094 ASCII + 3-byte char (e.g., Chinese) = split
				data := make([]byte, 4094)
				for i := range data {
					data[i] = 'x'
				}
				return append(data, []byte("中test")...) // 中 is 3 bytes: E4 B8 AD
			},
		},
		{
			name: "4-byte char at boundary",
			makeData: func() []byte {
				// 4093 ASCII + 4-byte char (emoji) = split
				data := make([]byte, 4093)
				for i := range data {
					data[i] = 'x'
				}
				return append(data, []byte("😀test")...) // 😀 is 4 bytes: F0 9F 98 80
			},
		},
		{
			name: "all ASCII no issue",
			makeData: func() []byte {
				data := make([]byte, 5000)
				for i := range data {
					data[i] = 'a'
				}
				return data
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.makeData()
			records := splitTextRecords(data)

			for i, rec := range records {
				// No trailing bytes to remove - records are pure text
				textPortion := rec

				if !utf8.Valid(textPortion) {
					t.Errorf("Record %d contains invalid UTF-8 (split character)", i)
				}
			}
		})
	}
}
