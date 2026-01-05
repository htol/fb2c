// Package mobi provides MOBI header generation.
package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestMOBIHeaderSize verifies that the MOBI header writes exactly 268 bytes
// from the MOBI marker (284 bytes from record start including PalmDOC header)
func TestMOBIHeaderSize(t *testing.T) {
	h := NewMOBIHeader(1000, 1)

	var buf bytes.Buffer
	err := h.Write(&buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Total size from record start should be:
	// 16 bytes PalmDOC header + 232 bytes MOBI header = 248 bytes
	totalSize := buf.Len()
	if totalSize != 248 {
		t.Errorf("Total header size = %d bytes, want 248 bytes (16 PalmDOC + 232 MOBI)", totalSize)
	}

	// Size from MOBI marker should be 232 bytes
	mobiOffset := 16 // PalmDOC header size
	mobiSize := totalSize - mobiOffset
	if mobiSize != 232 {
		t.Errorf("MOBI header size = %d bytes, want 232 bytes", mobiSize)
	}
}

// TestMOBIHeaderFieldOffsets verifies that critical fields are at the correct
// offsets according to the MobileRead Wiki specification for MOBI (232 bytes)
func TestMOBIHeaderFieldOffsets(t *testing.T) {
	h := NewMOBIHeader(1000, 1)

	// Set test values for critical fields
	h.FirstContentRec = 123
	h.LastContentRec = 456
	h.EXTHFlags = 0x40

	var buf bytes.Buffer
	err := h.Write(&buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	data := buf.Bytes()

	// Helper to check uint32 at offset from MOBI marker
	checkUint32 := func(name string, offsetFromMarker int, want uint32) {
		offset := 16 + offsetFromMarker // 16 = PalmDOC header size
		if offset+4 > len(data) {
			t.Errorf("Offset %d for %s is out of bounds (buffer size: %d)",
				offset, name, len(data))
			return
		}
		got := binary.BigEndian.Uint32(data[offset : offset+4])
		if got != want {
			t.Errorf("%s at offset +0x%X = 0x%08X, want 0x%08X", name, offsetFromMarker, got, want)
		}
	}

	// Helper to check uint16 at offset from MOBI marker
	checkUint16 := func(name string, offsetFromMarker int, want uint16) {
		offset := 16 + offsetFromMarker
		if offset+2 > len(data) {
			t.Errorf("Offset %d for %s is out of bounds (buffer size: %d)",
				offset, name, len(data))
			return
		}
		got := binary.BigEndian.Uint16(data[offset : offset+2])
		if got != want {
			t.Errorf("%s at offset +0x%X = 0x%04X, want 0x%04X", name, offsetFromMarker, got, want)
		}
	}

	// Check MOBI marker at offset +0x10
	mobiMarker := data[16:20]
	if !bytes.Equal(mobiMarker, []byte("MOBI")) {
		t.Errorf("MOBI marker = %q, want \"MOBI\"", mobiMarker)
	}

	// Check HeaderLength at offset +0x14 (20) from record start
	// Offset from MOBI marker: 0x14 - 0x10 = 0x04
	checkUint32("HeaderLength", 0x04, 232)

	// Check EXTHFlags at offset +0x80 (128) from record start
	// Offset from MOBI marker: 0x80 - 0x10 = 0x70
	checkUint32("EXTHFlags", 0x70, 0x40)

	// Check FirstContentRec at offset +0xC0 (192) from record start
	// Offset from MOBI marker: 0xC0 - 0x10 = 0xB0
	checkUint16("FirstContentRec", 0xB0, 123)

	// Check LastContentRec at offset +0xC2 (194) from record start
	// Offset from MOBI marker: 0xC2 - 0x10 = 0xB2
	checkUint16("LastContentRec", 0xB2, 456)
}

// TestMOBIHeaderDefaults verifies that default values match specification
func TestMOBIHeaderDefaults(t *testing.T) {
	h := NewMOBIHeader(5000, 3)

	// Check critical defaults
	if h.MOBIMarker != [4]byte{'M', 'O', 'B', 'I'} {
		t.Errorf("MOBIMarker = %v, want 'MOBI'", h.MOBIMarker)
	}

	if h.HeaderLength != 232 {
		t.Errorf("HeaderLength = %d, want 232", h.HeaderLength)
	}

	if h.MOBIType != 2 {
		t.Errorf("MOBIType = %d, want 2 (book)", h.MOBIType)
	}

	if h.TextEncoding != UTF8Encoding {
		t.Errorf("TextEncoding = %d, want %d (UTF-8)", h.TextEncoding, UTF8Encoding)
	}

	if h.FileVersion != 6 {
		t.Errorf("FileVersion = %d, want 6", h.FileVersion)
	}

	if h.EXTHFlags != 0x40 {
		t.Errorf("EXTHFlags = 0x%X, want 0x40 (has EXTH)", h.EXTHFlags)
	}

	if h.FirstContentRec != 1 {
		t.Errorf("FirstContentRec = %d, want 1", h.FirstContentRec)
	}

	if h.LastContentRec != 3 {
		t.Errorf("LastContentRec = %d, want 3", h.LastContentRec)
	}

	// Check that Unknown1 is initialized (32-byte array)
	if len(h.Unknown1) != 32 {
		t.Errorf("Unknown1 length = %d, want 32", len(h.Unknown1))
	}

	// Check that Unknown4 is initialized (8-byte array)
	if len(h.Unknown4) != 8 {
		t.Errorf("Unknown4 length = %d, want 8", len(h.Unknown4))
	}

	// Check that Unknown216 is initialized (8-byte array)
	if len(h.Unknown216) != 8 {
		t.Errorf("Unknown216 length = %d, want 8", len(h.Unknown216))
	}
}

// TestMOBIHeaderHelperMethods verifies that helper methods work correctly
func TestMOBIHeaderHelperMethods(t *testing.T) {
	h := NewMOBIHeader(1000, 1)

	// Test SetEXTHFlags
	h.SetEXTHFlags(0x50)
	if h.EXTHFlags != 0x50 {
		t.Errorf("SetEXTHFlags(0x50) failed: got 0x%X", h.EXTHFlags)
	}

	// Test SetContentRecords
	h.SetContentRecords(5, 10)
	if h.FirstContentRec != 5 {
		t.Errorf("SetContentRecords(5, 10): FirstContentRec = %d, want 5", h.FirstContentRec)
	}
	if h.LastContentRec != 10 {
		t.Errorf("SetContentRecords(5, 10): LastContentRec = %d, want 10", h.LastContentRec)
	}

	// Test SetFullName
	h.SetFullName("Test Book Title")
	if h.FullNameLength != 15 {
		t.Errorf("SetFullName(\"Test Book Title\"): FullNameLength = %d, want 15", h.FullNameLength)
	}
}

// TestMOBIHeaderRecordSize verifies that record size is set correctly
func TestMOBIHeaderRecordSize(t *testing.T) {
	// Test standard MOBI 6
	h := NewMOBIHeader(1000, 1)
	if h.RecordSize != StandardRecordSize {
		t.Errorf("Standard MOBI: RecordSize = %d, want %d", h.RecordSize, StandardRecordSize)
	}

	// Note: KF8JointRecordSize (0x10000000) cannot fit in uint16.
	// The KF8 format uses a different mechanism to signal KF8 content.
	// This test only verifies the standard MOBI 6 record size.
}

// TestMOBIHeaderDRMOffsets verifies DRM fields are at correct offsets
func TestMOBIHeaderDRMOffsets(t *testing.T) {
	h := NewMOBIHeader(1000, 1)

	// Set DRM values for testing
	h.DRMOffset = 0x11111111
	h.DRMCount = 0x22222222
	h.DRMSize = 0x33333333
	h.DRMFlags = 0x44444444

	var buf bytes.Buffer
	err := h.Write(&buf)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	data := buf.Bytes()

	// Check DRM fields at correct offsets
	checkUint32 := func(name string, offsetFromMarker int, want uint32) {
		offset := 16 + offsetFromMarker
		if offset+4 > len(data) {
			t.Errorf("Offset %d for %s is out of bounds (buffer size: %d)",
				offset, name, len(data))
			return
		}
		got := binary.BigEndian.Uint32(data[offset : offset+4])
		if got != want {
			t.Errorf("%s at offset +0x%X = 0x%08X, want 0x%08X", name, offsetFromMarker, got, want)
		}
	}

	checkUint32("DRMOffset", 0x98, 0x11111111)
	checkUint32("DRMCount", 0x9C, 0x22222222)
	checkUint32("DRMSize", 0xA0, 0x33333333)
	checkUint32("DRMFlags", 0xA4, 0x44444444)
}
