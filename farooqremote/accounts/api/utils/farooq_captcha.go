package utils

// Farooq Remote (FA-020, owner 1 Nov 2026: «Captcha code very hard to read ... i enlarge the windows 8 time to read
// code»): our own picture for the 4-digit code. Upstream's string driver gives every character a random size and a
// random colour (some come out pale) and its wave band runs through the characters; here every digit has the same
// big size, a dark colour and a little bold, on a near-white background, with only a faint line and a few faint dots
// behind it. The picture is drawn twice as large (400 x 128) and shown at 200 x 64, so it stays sharp when zoomed.
// The digits themselves still come from upstream's generator (capdString: source 2-9, length 4).

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"sync"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/mojocn/base64Captcha"
)

const (
	frCapW    = 400  // pixels; shown at half size
	frCapH    = 128  //
	frCapSize = 92.0 // digit size in pixels (72 dpi)
	frCapLift = 33   // baseline below the middle = about 0.36 of the size, so the digits sit in the middle
)

var (
	frCapFontOnce sync.Once
	frCapFont     *truetype.Font
	frCapBack     = color.RGBA{0xfa, 0xfb, 0xfd, 0xff}
	frCapFaint    = color.RGBA{0xc5, 0xcd, 0xd8, 0xff}
	frCapColors   = []color.RGBA{
		{0x1f, 0x3a, 0x93, 0xff}, // navy
		{0x1e, 0x6b, 0x34, 0xff}, // dark green
		{0xa3, 0x20, 0x1b, 0xff}, // dark red
		{0x22, 0x22, 0x22, 0xff}, // near black
		{0x5b, 0x2c, 0x83, 0xff}, // dark purple
		{0x0b, 0x5e, 0x6e, 0xff}, // dark teal
		{0x7a, 0x4b, 0x12, 0xff}, // brown
	}
)

func frCaptchaFont() *truetype.Font {
	frCapFontOnce.Do(func() {
		frCapFont = base64Captcha.DefaultEmbeddedFonts.LoadFontByName("fonts/wqy-microhei.ttc")
	})
	return frCapFont
}

// FrDrawDigits draws the code as a PNG and returns it as a data URI (the same form upstream's item.EncodeB64string gives).
func FrDrawDigits(content string) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, frCapW, frCapH))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: frCapBack}, image.Point{}, draw.Src)

	// behind the digits: one faint wave line and a few faint dots (people never have to read through them)
	amp := 10 + rand.Float64()*14
	period := 140 + rand.Float64()*120
	phase := rand.Float64() * 2 * math.Pi
	mid := float64(frCapH)/2 + (rand.Float64()-0.5)*30
	for x := 0; x < frCapW; x++ {
		y := int(mid + amp*math.Sin(2*math.Pi*float64(x)/period+phase))
		for t := -2; t <= 2; t++ {
			img.Set(x, y+t, frCapFaint)
		}
	}
	for i := 0; i < 40; i++ {
		cx, cy := rand.Intn(frCapW), rand.Intn(frCapH)
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				if dx*dx+dy*dy <= 4 {
					img.Set(cx+dx, cy+dy, frCapFaint)
				}
			}
		}
	}

	f := frCaptchaFont()
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(frCapSize)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	scale := c.PointToFixed(frCapSize)

	runes := []rune(content)
	if len(runes) == 0 {
		runes = []rune("0")
	}
	slot := frCapW / len(runes)
	for i, r := range runes {
		w := f.HMetric(scale, f.Index(r)).AdvanceWidth.Round() + 3
		x := slot*i + (slot-w)/2 + rand.Intn(17) - 8
		y := frCapH/2 + frCapLift + rand.Intn(17) - 8
		c.SetSrc(image.NewUniform(frCapColors[rand.Intn(len(frCapColors))]))
		for ox := 0; ox < 4; ox++ { // a little bold: the same digit four times, one pixel apart
			if _, err := c.DrawString(string(r), freetype.Pt(x+ox, y)); err != nil {
				return "", err
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
