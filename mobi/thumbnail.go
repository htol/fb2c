package mobi

import (
	"bytes"
	"image"
	"image/jpeg"

	_ "image/gif" // register GIF/PNG decoding for covers; thumbnail re-encodes as JPEG
	_ "image/png"
)

// thumbnailMaxHeight is the Kindle library-list thumbnail height (~154x240).
const thumbnailMaxHeight = 240

// buildThumbnail downscales a decoded cover to at most thumbnailMaxHeight
// pixels high using a box filter and re-encodes it as JPEG. Stdlib only and
// deterministic (fixed quality, no metadata). Undecodable or already-small
// input falls back to the original bytes: an oversized thumbnail is valid,
// merely wasteful.
func buildThumbnail(coverData []byte) []byte {
	src, _, err := image.Decode(bytes.NewReader(coverData))
	if err != nil {
		return coverData
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= thumbnailMaxHeight {
		return coverData
	}
	dh := thumbnailMaxHeight
	dw := sw * dh / sh
	if dw < 1 {
		dw = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy0 := b.Min.Y + y*sh/dh
		sy1 := b.Min.Y + (y+1)*sh/dh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := 0; x < dw; x++ {
			sx0 := b.Min.X + x*sw/dw
			sx1 := b.Min.X + (x+1)*sw/dw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					// 16-bit alpha-premultiplied components; covers are opaque
					// photos, so dropping alpha loses nothing for JPEG output.
					pr, pg, pb, _ := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					bl += uint64(pb)
					n++
				}
			}
			off := dst.PixOffset(x, y)
			dst.Pix[off+0] = byte(r / n >> 8)
			dst.Pix[off+1] = byte(g / n >> 8)
			dst.Pix[off+2] = byte(bl / n >> 8)
			dst.Pix[off+3] = 0xFF
		}
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 75}); err != nil {
		return coverData
	}
	return out.Bytes()
}
