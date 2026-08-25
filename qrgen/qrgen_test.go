package qrgen

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// makeTestLogo draws a pill-shaped logo with a border and text, similar in
// spirit to the pass2fun sample.
func makeTestLogo(text string) image.Image {
	w, h := 300, 110
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Border pill, then inner white pill.
	fillRoundRect(img, roundRect{0, 0, float64(w), float64(h), float64(h) / 2}, uniformPaint{color.Black})
	fillRoundRect(img, roundRect{6, 6, float64(w) - 6, float64(h) - 6, float64(h)/2 - 6}, uniformPaint{color.White})
	// Crude centered text with the basic 7x13 face.
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
	}
	tw := d.MeasureString(text).Ceil()
	d.Dot = fixed.P((w-tw)/2, h/2+4)
	d.DrawString(text)
	return img
}

func TestGenerateBasicDecodes(t *testing.T) {
	res, err := Generate(Options{Value: "https://example.com/hello", Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}
}

func TestGenerateWithLogoDecodes(t *testing.T) {
	res, err := Generate(Options{
		Value: "https://pass2fun.com/",
		Logo:  makeTestLogo("pass2fun"),
		Size:  1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}
	if res.LogoScale < 0.10 {
		t.Fatalf("logo shrank too much: %f", res.LogoScale)
	}
}

func TestAllStylesDecode(t *testing.T) {
	for _, style := range []Style{StyleDots, StyleRounded, StyleSquares} {
		t.Run(string(style), func(t *testing.T) {
			_, err := Generate(Options{
				Value: "https://example.com/style-test?x=12345",
				Logo:  makeTestLogo("logo"),
				Style: style,
				Size:  768,
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLongValueWithLogo(t *testing.T) {
	long := "https://example.com/booking?id=ABC123456&session=" + strings.Repeat("x", 120)
	_, err := Generate(Options{Value: long, Logo: makeTestLogo("brand"), Size: 1024})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSVGOutput(t *testing.T) {
	svg, res, err := GenerateSVG(Options{
		Value: "https://example.com/svg",
		Logo:  makeTestLogo("svg"),
		Size:  1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "data:image/png;base64,") {
		t.Fatal("svg missing expected content")
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}
}

func TestSmallSizeWithLogoDecodes(t *testing.T) {
	// Simulates a scanner seeing the code small/far away.
	res, err := Generate(Options{
		Value: "https://pass2fun.com/promo",
		Logo:  makeTestLogo("pass2fun"),
		Size:  256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}
}

func TestTransparentBackgroundDecodes(t *testing.T) {
	bg, _ := ParseColor("transparent")
	_, err := Generate(Options{
		Value: "https://example.com/transparent",
		Logo:  makeTestLogo("brand"),
		BG:    bg,
		Size:  512,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCustomColorsDecode(t *testing.T) {
	_, err := Generate(Options{
		Value: "https://example.com/colors",
		Size:  512,
		FG:    color.RGBA{R: 20, G: 40, B: 120, A: 255},
		BG:    color.RGBA{R: 245, G: 245, B: 245, A: 255},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBadInputs(t *testing.T) {
	if _, err := Generate(Options{Value: ""}); err == nil {
		t.Fatal("empty value should fail")
	}
	if _, err := Generate(Options{Value: "x", Size: 50}); err == nil {
		t.Fatal("tiny size should fail")
	}
	if _, err := Generate(Options{Value: "x", LogoScale: 0.9}); err == nil {
		t.Fatal("huge logo scale should fail")
	}
}
