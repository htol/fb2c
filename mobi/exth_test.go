package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestEXTHHeaderLengthExcludesPadding verifies spec §4: the EXTH HeaderLength
// covers the 12-byte header plus records but excludes the final padding, while
// the byte count returned by Write and by GetTotalLength includes it
// (FullNameOffset must point past the padded EXTH, spec §5).
func TestEXTHHeaderLengthExcludesPadding(t *testing.T) {
	w := NewEXTHWriter()
	w.AddASIN("B012345678") // 10 data bytes: pure = 12 + 8 + 10 = 30, pad 2, padded 32

	var buf bytes.Buffer
	n, err := w.Write(&buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) != n {
		t.Fatalf("Write returned %d but wrote %d bytes", n, len(data))
	}
	if got := w.GetTotalLength(); got != n {
		t.Fatalf("GetTotalLength() = %d, want %d (padded length)", got, n)
	}

	if string(data[:4]) != "EXTH" {
		t.Fatalf("identifier = %q, want EXTH", data[:4])
	}
	headerLength := binary.BigEndian.Uint32(data[4:8])
	if headerLength != 30 {
		t.Errorf("HeaderLength = %d, want 30 (pure length, padding excluded)", headerLength)
	}
	if recordCount := binary.BigEndian.Uint32(data[8:12]); recordCount != 1 {
		t.Errorf("RecordCount = %d, want 1", recordCount)
	}

	// Padded total length, while HeaderLength stays pure.
	if n != 32 {
		t.Errorf("padded byte count = %d, want 32", n)
	}
	// The two padding bytes must be zeros after the last record.
	if data[30] != 0 || data[31] != 0 {
		t.Errorf("padding bytes = %v, want zeros", data[30:32])
	}
}

// TestEXTHWriteEmpty verifies Write emits nothing without records.
func TestEXTHWriteEmpty(t *testing.T) {
	w := NewEXTHWriter()
	var buf bytes.Buffer
	n, err := w.Write(&buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Fatalf("Write with no records emitted %d bytes, want 0", buf.Len())
	}
	if got := w.GetTotalLength(); got != 0 {
		t.Fatalf("GetTotalLength() = %d, want 0", got)
	}
}
