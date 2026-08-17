package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestHeaderWriteSize verifies the written record-0 header size: 16 bytes of
// PalmDOC header plus a 232-byte MOBI header (HeaderLength) = 248 bytes total.
func TestHeaderWriteSize(t *testing.T) {
	h := NewHeader(1000, 1)

	var buf bytes.Buffer
	if err := h.Write(&buf); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) != 16+232 {
		t.Fatalf("Write() emitted %d bytes, want 248 (16 PalmDOC + 232 MOBI)", len(data))
	}

	mobiData := data[16:]
	if string(mobiData[:4]) != "MOBI" {
		t.Errorf("MOBI marker = %q, want \"MOBI\"", mobiData[:4])
	}
	if headerLength := binary.BigEndian.Uint32(mobiData[4:8]); headerLength != 232 {
		t.Errorf("HeaderLength = %d, want 232", headerLength)
	}
}
