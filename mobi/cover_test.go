package mobi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// TestEncodeCoverJPEG verifies the cover pipeline: any decodable input becomes
// a baseline JPEG carrying the JFIF APP0 marker (required by the Kindle
// firmware's decoder; Go's encoder omits it), and the output is deterministic.
func TestEncodeCoverJPEG(t *testing.T) {
	// Palette PNG (the fb2 corpus cover shape) as input.
	src := image.NewPaletted(image.Rect(0, 0, 40, 60), color.Palette{color.Black, color.White})
	for y := 0; y < 60; y++ {
		for x := 0; x < 40; x++ {
			src.SetColorIndex(x, y, uint8((x/4+y/6)%2)) //nolint:gosec // x<40, y<60: the value stays far below 256
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}

	out := encodeCoverJPEG(pngBuf.Bytes())
	if len(out) < 4 || string(out[:2]) != "\xff\xd8" {
		t.Fatalf("output is not a JPEG: starts %v", out[:4])
	}
	if !bytes.HasPrefix(out[2:], []byte{0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}) {
		t.Fatalf("JFIF APP0 marker missing after SOI: %v", out[2:12])
	}
	// Exactly one SOI: the firmware rejects a stream whose first marker after
	// the APP0 segment is another SOI (double SOI — Go's encoder emits its own
	// SOI, so APP0 must be spliced in, never prepended with a hand-written
	// one). Regression: on-device the cover page went blank for exactly this.
	app0End := 2 + 2 + int(out[4])<<8 | int(out[5]) // SOI + marker+len of APP0
	if next := out[app0End : app0End+2]; next[0] != 0xFF || next[1] == 0xD8 || next[1] == 0xD9 {
		t.Fatalf("marker after APP0 = % X; want a real segment marker, not SOI/EOI (double-SOI bug)", next)
	}
	if nSOI := bytes.Count(out, []byte{0xFF, 0xD8}); nSOI != 1 {
		t.Errorf("SOI count = %d, want exactly 1", nSOI)
	}
	// The stream must end with EOI and stay decodable end-to-end.
	if !bytes.HasSuffix(out, []byte{0xFF, 0xD9}) {
		t.Errorf("output does not end with EOI")
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("re-encoded cover does not decode: %v", err)
	}
	if again := encodeCoverJPEG(pngBuf.Bytes()); !bytes.Equal(out, again) {
		t.Error("encodeCoverJPEG is not deterministic")
	}

	// Already-valid input: undecodable bytes pass through unchanged.
	junk := []byte("not an image")
	if got := encodeCoverJPEG(junk); !bytes.Equal(got, junk) {
		t.Error("undecodable input should pass through unchanged")
	}
}

// TestEncodeCoverJPEGSingleSOI guards the exact on-device failure mode of the
// first implementation: SOI + APP0 + SOI (double SOI). The Kindle firmware
// renders nothing for such a stream even though desktop decoders accept it.
func TestEncodeCoverJPEGSingleSOI(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 8, 8))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}
	out := encodeCoverJPEG(pngBuf.Bytes())
	if bytes.Contains(out[2:], []byte{0xFF, 0xD8}) {
		t.Fatalf("double SOI in cover stream: % X", out[:24])
	}
}
