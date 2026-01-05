package mobi

import (
	"bytes"
	"fmt"
	"testing"
)

// TestCountBytes counts what Write() actually writes
func TestCountBytes(t *testing.T) {
	h := NewMOBIHeader(1000, 1)

	var buf bytes.Buffer
	err := h.Write(&buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	data := buf.Bytes()
	total := len(data)
	mobiData := data[16:] // Skip PalmDOC header

	fmt.Printf("=== BYTE COUNT ANALYSIS ===\n")
	fmt.Printf("Total bytes written: %d\n", total)
	fmt.Printf("Expected: 284 (16 PalmDOC + 268 MOBI)\n")
	fmt.Printf("Extra bytes: %d\n\n", total-284)

	fmt.Printf("=== BYTES AFTER MOBI MARKER ===\n")
	fmt.Printf("Total MOBI data: %d bytes (expected 268)\n", len(mobiData))
	fmt.Printf("Extra: %d bytes\n\n", len(mobiData)-268)

	// Analyze sections
	fmt.Printf("=== SECTIONS AFTER MOBI MARKER ===\n")
	fmt.Printf("+0x14 to +0x81 (HeaderLength to end before UnknownBig1): %d bytes (28 uint32s * 4 = 112)\n", len(mobiData[4:4+112]))
	fmt.Printf("Should be: 112 bytes\n")

	// Check where UnknownBig1 actually is
	unknownBig1Offset := 4 + 112 // After HeaderLength + 28 uint32s
	fmt.Printf("\nUnknownBig1 should start at offset: +0x%X (from MOBI marker)\n", unknownBig1Offset)
	fmt.Printf("That's byte %d in mobiData (byte %d total)\n", unknownBig1Offset, 16+unknownBig1Offset)

	// Check what's there
	fmt.Printf("32 bytes at UnknownBig1 location: %x\n", mobiData[unknownBig1Offset:unknownBig1Offset+32])

	// Check what's after UnknownBig1
	afterUnknownBig1 := unknownBig1Offset + 32
	fmt.Printf("\nAfter UnknownBig1, offset +0x%X (from MOBI marker):\n", afterUnknownBig1)
	fmt.Printf("Next 8 bytes (should be Unknown7): %x\n", mobiData[afterUnknownBig1:afterUnknownBig1+8])

	// Check First/Last ContentRec
	firstLastOffset := afterUnknownBig1 + 8
	fmt.Printf("Next 4 bytes (should be First/Last ContentRec): %x\n", mobiData[firstLastOffset:firstLastOffset+4])

	// Check remaining uint32s
	remainingOffset := firstLastOffset + 4
	remainingSize := 16 * 4
	fmt.Printf("Next 64 bytes (16 remaining uint32s): %x\n", mobiData[remainingOffset:remainingOffset+remainingSize])

	// Check Unknown9
	unknown9Offset := remainingOffset + remainingSize
	fmt.Printf("Final 8 bytes (Unknown9): %x\n", mobiData[unknown9Offset:unknown9Offset+8])

	totalCalculated := unknown9Offset + 8
	fmt.Printf("\nCalculated total after MOBI marker: %d bytes\n", totalCalculated)
	fmt.Printf("Actual: %d bytes\n", len(mobiData))
}
