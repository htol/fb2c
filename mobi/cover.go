package mobi

import (
	"bytes"
	"image"
	"image/jpeg"
)

// coverJPEGQuality is the re-encode quality for non-JPEG covers.
const coverJPEGQuality = 85

// jfifAPP0 is the standard JFIF APP0 marker segment (version 1.01, no
// density). Go's image/jpeg encoder omits APP0 entirely, and the Kindle
// firmware's JPEG decoder refuses such streams: the cover page renders blank
// even though every desktop decoder accepts them. Verified on-device
// 2026-08-18: the same re-encoded JPEG fails without APP0 and renders with it.
var jfifAPP0 = []byte{
	0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00,
	0x01, 0x01, 0x01, 0x00, 0x64, 0x00, 0x64, 0x00, 0x00,
}

// encodeCoverJPEG re-encodes the cover image as a baseline JPEG with a JFIF
// APP0 marker. The Kindle firmware renders only such covers reliably: it
// rejected fb2c's palette-PNG covers (Calibre always ships JPEG), and Go's
// JPEG encoder alone omits the JFIF header the decoder expects. Falls back to
// the input bytes when they cannot be decoded (already-JPEG inputs with a
// JFIF header pass through unchanged).
func encodeCoverJPEG(data []byte) []byte {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var buf bytes.Buffer
	buf.WriteByte(0xFF)
	buf.WriteByte(0xD8) // SOI
	buf.Write(jfifAPP0)
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: coverJPEGQuality}); err != nil {
		return data
	}
	return buf.Bytes()
}
