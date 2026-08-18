package mobi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// TestBuildThumbnail verifies the thumbnail pipeline: a tall cover is
// downscaled to at most thumbnailMaxHeight, re-encoded smaller, and the
// output is deterministic.
func TestBuildThumbnail(t *testing.T) {
	// 480x800 gradient PNG as a synthetic cover
	src := image.NewRGBA(image.Rect(0, 0, 480, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 480; x++ {
			src.Set(x, y, color.RGBA{byte(x % 256), byte(y % 256), 0x80, 0xFF})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}
	cover := pngBuf.Bytes()

	thumb := buildThumbnail(cover)
	img, format, err := image.Decode(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail does not decode: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("thumbnail format = %s, want jpeg", format)
	}
	if h := img.Bounds().Dy(); h > thumbnailMaxHeight {
		t.Errorf("thumbnail height = %d, want <= %d", h, thumbnailMaxHeight)
	}
	if w := img.Bounds().Dx(); w*800 != 480*img.Bounds().Dy() {
		t.Errorf("aspect ratio not preserved: %dx%d from 480x800", w, img.Bounds().Dy())
	}

	// Determinism: same input must produce identical bytes.
	if again := buildThumbnail(cover); !bytes.Equal(thumb, again) {
		t.Error("buildThumbnail is not deterministic")
	}
}

// TestBuildThumbnailJFIF verifies the firmware contract on the thumbnail
// record: the Kindle's thumbnail decoder, like its cover decoder, refuses a
// JPEG without a JFIF APP0 marker — the device then leaves a 0-byte
// thumbnail_<ASIN>_EBOK_portrait.jpg.tmp.partial in system/thumbnails/
// (verified on-device 2026-08-18). The marker must sit right after the
// single SOI; a second SOI is equally fatal.
func TestBuildThumbnailJFIF(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 480, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 480; x++ {
			src.Set(x, y, color.RGBA{byte(x % 256), byte(y % 256), 0x80, 0xFF})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}

	thumb := buildThumbnail(pngBuf.Bytes())
	if len(thumb) < 2 || thumb[0] != 0xFF || thumb[1] != 0xD8 {
		t.Fatalf("thumbnail is not a JPEG: starts %v", thumb[:4])
	}
	if !bytes.HasPrefix(thumb[2:], jfifAPP0) {
		t.Fatalf("thumbnail lacks JFIF APP0 after SOI: % X", thumb[:24])
	}
	if bytes.Contains(thumb[2:], []byte{0xFF, 0xD8}) {
		t.Fatalf("double SOI in thumbnail stream: % X", thumb[:24])
	}
}

// TestBuildThumbnailFallback verifies graceful degradation: undecodable input
// and already-small images are returned unchanged.
func TestBuildThumbnailFallback(t *testing.T) {
	junk := []byte("not an image at all")
	if got := buildThumbnail(junk); !bytes.Equal(got, junk) {
		t.Error("undecodable input should fall back to the original bytes")
	}

	small := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	if err := png.Encode(&buf, small); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}
	if got := buildThumbnail(buf.Bytes()); !bytes.Equal(got, buf.Bytes()) {
		t.Error("small image should be returned unchanged")
	}
}
