package qrgen

import (
	"image/color"
	"strings"
	"testing"
)

func TestGradientsDecode(t *testing.T) {
	for _, dir := range []string{GradientVertical, GradientHorizontal, GradientDiagonal, GradientRadial} {
		t.Run(dir, func(t *testing.T) {
			res, err := Generate(Options{
				Value:    "https://pass2fun.com/gradiente",
				Logo:     makeTestLogo("pass2fun"),
				FG:       color.RGBA{R: 15, G: 30, B: 110, A: 255}, // dark blue
				FG2:      color.RGBA{R: 95, G: 10, B: 100, A: 255}, // dark purple
				Gradient: dir,
				Size:     768,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !res.Validated {
				t.Fatal("expected validated result")
			}
		})
	}
}

func TestGradientRequiresSecondColor(t *testing.T) {
	_, err := Generate(Options{Value: "x", Gradient: GradientVertical})
	if err == nil || !strings.Contains(err.Error(), "fg2") {
		t.Fatalf("expected fg2 error, got %v", err)
	}
}

func TestFG2ImpliesGradient(t *testing.T) {
	res, err := Generate(Options{
		Value: "https://example.com",
		FG:    color.RGBA{A: 255},
		FG2:   color.RGBA{R: 60, B: 90, A: 255},
		Size:  512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}
}

func TestGradientSVGHasDefs(t *testing.T) {
	svg, _, err := GenerateSVG(Options{
		Value:    "https://example.com/svg-grad",
		FG:       color.RGBA{R: 15, G: 30, B: 110, A: 255},
		FG2:      color.RGBA{R: 95, G: 10, B: 100, A: 255},
		Gradient: GradientRadial,
		Size:     512,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<radialGradient", `url(#fgg)`, "#0f1e6e", "#5f0a64"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}
}

const testSVGLogo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 110">
<rect x="4" y="4" width="292" height="102" rx="51" fill="#fff" stroke="#000" stroke-width="8"/>
<circle cx="90" cy="55" r="22" fill="#000"/>
<rect x="130" y="38" width="120" height="34" rx="17" fill="#000"/>
</svg>`

func TestSVGLogoInput(t *testing.T) {
	img, raw, err := ParseLogo([]byte(testSVGLogo))
	if err != nil {
		t.Fatal(err)
	}
	if raw == nil {
		t.Fatal("expected raw svg bytes back")
	}
	if b := img.Bounds(); b.Dx() != 768 || b.Dy() < 200 {
		t.Fatalf("unexpected raster size %v", b)
	}

	res, err := Generate(Options{Value: "https://example.com/svg-logo", Logo: img, Size: 768})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validated {
		t.Fatal("expected validated result")
	}

	svg, _, err := GenerateSVG(Options{Value: "https://example.com/svg-logo", Logo: img, LogoSVG: raw, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "data:image/svg+xml;base64,") {
		t.Fatal("svg output should embed the svg logo")
	}
}

func TestParseLogoRaster(t *testing.T) {
	if _, _, err := ParseLogo([]byte("not an image")); err == nil {
		t.Fatal("expected error for junk input")
	}
}
