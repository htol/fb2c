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
			src.SetColorIndex(x, y, uint8((x/4+y/6)%2))
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
	if again := encodeCoverJPEG(pngBuf.Bytes()); !bytes.Equal(out, again) {
		t.Error("encodeCoverJPEG is not deterministic")
	}

	// Already-valid input: undecodable bytes pass through unchanged.
	junk := []byte("not an image")
	if got := encodeCoverJPEG(junk); !bytes.Equal(got, junk) {
		t.Error("undecodable input should pass through unchanged")
	}
}
