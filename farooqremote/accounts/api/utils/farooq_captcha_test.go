package utils

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"
)

// Farooq Remote: the picture code is a 400 x 128 PNG data URI with dark digits on a light background.
func TestFrDrawDigits(t *testing.T) {
	for _, code := range []string{"2345", "6789", "8888"} {
		uri, err := FrDrawDigits(code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		const head = "data:image/png;base64,"
		if !strings.HasPrefix(uri, head) {
			t.Fatalf("%s: not a PNG data URI: %.40s", code, uri)
		}
		raw, err := base64.StdEncoding.DecodeString(uri[len(head):])
		if err != nil {
			t.Fatalf("%s: base64: %v", code, err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: png: %v", code, err)
		}
		b := img.Bounds()
		if b.Dx() != frCapW || b.Dy() != frCapH {
			t.Fatalf("%s: size %dx%d, want %dx%d", code, b.Dx(), b.Dy(), frCapW, frCapH)
		}
		dark, light := 0, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				lum := (299*r + 587*g + 114*bl) / 1000 >> 8
				if lum < 130 {
					dark++
				} else if lum > 230 {
					light++
				}
			}
		}
		// four big digits cover thousands of pixels; the background stays the larger part
		if dark < 3000 || light < frCapW*frCapH/2 {
			t.Fatalf("%s: dark %d, light %d - digits missing or background not light", code, dark, light)
		}
		t.Logf("%s: %d bytes, dark %d, light %d", code, len(raw), dark, light)
	}
}
