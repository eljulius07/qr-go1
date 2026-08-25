// Package qrgen generates styled QR codes (dot modules, rounded finder
// patterns, optional centered logo) as PNG or SVG, and can validate that the
// result still decodes to the original value.
package qrgen

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	xdraw "golang.org/x/image/draw"
)

// Style selects how dark modules are drawn.
type Style string

const (
	StyleDots    Style = "dots"    // circles, like the pass2fun sample
	StyleRounded Style = "rounded" // rounded squares
	StyleSquares Style = "squares" // classic squares
)

// Options controls generation. Zero values get sensible defaults.
type Options struct {
	Value     string      // required: text or URL to encode
	Logo      image.Image // optional centered logo
	LogoSVG   []byte      // raw SVG source of the logo, embedded as-is in SVG output
	Size      int         // output width/height in px (default 1024)
	Style     Style       // default StyleDots
	FG        color.Color // module color (default black)
	FG2       color.Color // optional second color for gradients
	Gradient  string      // "", vertical, horizontal, diagonal, radial (requires FG2)
	BG        color.Color // background (default white); may be transparent
	ECLevel   qrcode.RecoveryLevel
	ecSet     bool
	LogoScale float64 // logo width as fraction of the full image (default 0.22, max 0.30)
	Margin    int     // quiet zone in modules (default 3)
	// Validate re-decodes the rendered image and, if it fails, retries with a
	// smaller logo. Enabled by default in Generate.
	Validate bool
}

// SetECLevel forces an error-correction level (otherwise: H with logo, M without).
func (o *Options) SetECLevel(l qrcode.RecoveryLevel) {
	o.ECLevel = l
	o.ecSet = true
}

// Result is a generated QR code.
type Result struct {
	Image     *image.RGBA // final rendered image (nil for SVG-only use)
	Matrix    [][]bool    // module matrix without quiet zone
	LogoScale float64     // logo scale actually used after validation retries
	Validated bool        // true if the image was decoded successfully
}

const superSample = 4 // render at Nx and downscale for antialiasing

func (o *Options) applyDefaults() error {
	if strings.TrimSpace(o.Value) == "" {
		return errors.New("value is required")
	}
	if o.Size == 0 {
		o.Size = 1024
	}
	if o.Size < 128 || o.Size > 4096 {
		return fmt.Errorf("size must be between 128 and 4096, got %d", o.Size)
	}
	if o.Style == "" {
		o.Style = StyleDots
	}
	switch o.Style {
	case StyleDots, StyleRounded, StyleSquares:
	default:
		return fmt.Errorf("unknown style %q (use dots, rounded or squares)", o.Style)
	}
	if o.FG == nil {
		o.FG = color.Black
	}
	if o.BG == nil {
		o.BG = color.White
	}
	if o.FG2 != nil && o.Gradient == "" {
		o.Gradient = GradientVertical
	}
	if o.Gradient != "" {
		if o.FG2 == nil {
			return errors.New("gradient requires a second color (fg2)")
		}
		switch o.Gradient {
		case GradientVertical, GradientHorizontal, GradientDiagonal, GradientRadial:
		default:
			return fmt.Errorf("unknown gradient %q (use vertical, horizontal, diagonal or radial)", o.Gradient)
		}
	}
	if !o.ecSet {
		if o.Logo != nil {
			o.ECLevel = qrcode.Highest
		} else {
			o.ECLevel = qrcode.Medium
		}
	}
	if o.Logo != nil && o.ECLevel < qrcode.High {
		// A centered logo destroys modules; require strong error correction.
		o.ECLevel = qrcode.High
	}
	if o.LogoScale == 0 {
		o.LogoScale = 0.22
	}
	if o.LogoScale < 0.10 || o.LogoScale > 0.30 {
		return fmt.Errorf("logo_scale must be between 0.10 and 0.30, got %.2f", o.LogoScale)
	}
	if o.Margin == 0 {
		o.Margin = 3
	}
	if o.Margin < 1 || o.Margin > 10 {
		return fmt.Errorf("margin must be between 1 and 10 modules, got %d", o.Margin)
	}
	return nil
}

// Generate renders the QR code. With Validate enabled it decodes the result
// and retries with a progressively smaller logo if decoding fails.
func Generate(o Options) (*Result, error) {
	o.Validate = true
	return generate(o)
}

// GenerateUnvalidated renders without the decode check (faster).
func GenerateUnvalidated(o Options) (*Result, error) {
	o.Validate = false
	return generate(o)
}

func generate(o Options) (*Result, error) {
	if err := o.applyDefaults(); err != nil {
		return nil, err
	}
	matrix, err := buildMatrix(o.Value, o.ECLevel)
	if err != nil {
		return nil, err
	}

	scale := o.LogoScale
	var lastErr error
	// Up to 4 attempts, shrinking the logo each time decoding fails.
	for attempt := 0; attempt < 4; attempt++ {
		img := render(matrix, o, scale)
		if !o.Validate {
			return &Result{Image: img, Matrix: matrix, LogoScale: scale}, nil
		}
		if err := Verify(img, o.Value); err == nil {
			return &Result{Image: img, Matrix: matrix, LogoScale: scale, Validated: true}, nil
		} else {
			lastErr = err
		}
		if o.Logo == nil {
			break // nothing to shrink; a bare QR failing to decode is a real bug
		}
		scale *= 0.85
	}
	return nil, fmt.Errorf("generated QR failed validation: %w", lastErr)
}

func buildMatrix(value string, ec qrcode.RecoveryLevel) ([][]bool, error) {
	q, err := qrcode.New(value, ec)
	if err != nil {
		return nil, fmt.Errorf("encoding value: %w", err)
	}
	q.DisableBorder = true
	return q.Bitmap(), nil
}

// render draws the full image at superSample resolution and downscales.
func render(matrix [][]bool, o Options, logoScale float64) *image.RGBA {
	n := len(matrix)
	total := n + 2*o.Margin
	rs := o.Size * superSample          // supersampled canvas size
	mod := float64(rs) / float64(total) // module size in supersampled px

	canvas := image.NewRGBA(image.Rect(0, 0, rs, rs))
	fillRect(canvas, canvas.Bounds(), o.BG)

	// Foreground painter (flat color or gradient) and background paint.
	fgPaint, err := newPainter(&o, float64(rs))
	if err != nil {
		// Direction is validated in applyDefaults; this cannot happen.
		fgPaint = uniformPaint{o.FG}
	}
	bgPaint := uniformPaint{o.BG}

	// Logo clear zone (stadium-shaped rounded rect) in supersampled px.
	var zone *roundRect
	var logoRect image.Rectangle
	if o.Logo != nil {
		lb := o.Logo.Bounds()
		aspect := float64(lb.Dy()) / float64(lb.Dx())
		lw := logoScale * float64(rs)
		lh := lw * aspect
		maxH := 0.30 * float64(rs)
		if lh > maxH { // very tall logo: clamp height, shrink width
			lh = maxH
			lw = lh / aspect
		}
		cx, cy := float64(rs)/2, float64(rs)/2
		logoRect = image.Rect(int(cx-lw/2), int(cy-lh/2), int(cx+lw/2), int(cy+lh/2))
		pad := 0.9 * mod
		zr := math.Min(lw+2*pad, lh+2*pad) / 2 * 0.9
		zone = &roundRect{
			x0: cx - lw/2 - pad, y0: cy - lh/2 - pad,
			x1: cx + lw/2 + pad, y1: cy + lh/2 + pad,
			r: zr,
		}
	}

	inFinder := finderTest(n)

	// Data modules.
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !matrix[y][x] || inFinder(x, y) {
				continue
			}
			cx := (float64(o.Margin) + float64(x) + 0.5) * mod
			cy := (float64(o.Margin) + float64(y) + 0.5) * mod
			if zone != nil && zone.contains(cx, cy, 0.55*mod) {
				continue
			}
			switch o.Style {
			case StyleDots:
				fillCircle(canvas, cx, cy, mod*0.5, fgPaint)
			case StyleRounded:
				fillRoundRect(canvas, roundRect{cx - mod/2, cy - mod/2, cx + mod/2, cy + mod/2, mod * 0.3}, fgPaint)
			case StyleSquares:
				fillRectP(canvas, image.Rect(int(cx-mod/2), int(cy-mod/2), int(math.Ceil(cx+mod/2)), int(math.Ceil(cy+mod/2))), fgPaint)
			}
		}
	}

	// Finder patterns: rounded ring + rounded center, like the sample image.
	for _, f := range finderOrigins(n) {
		fx := (float64(o.Margin) + float64(f[0])) * mod
		fy := (float64(o.Margin) + float64(f[1])) * mod
		outer := roundRect{fx, fy, fx + 7*mod, fy + 7*mod, 2.3 * mod}
		inner := roundRect{fx + mod, fy + mod, fx + 6*mod, fy + 6*mod, 1.7 * mod}
		center := roundRect{fx + 2*mod, fy + 2*mod, fx + 5*mod, fy + 5*mod, 1.1 * mod}
		if o.Style == StyleSquares {
			outer.r, inner.r, center.r = 0, 0, 0
		}
		fillRoundRect(canvas, outer, fgPaint)
		fillRoundRect(canvas, inner, bgPaint)
		fillRoundRect(canvas, center, fgPaint)
	}

	// Clear zone + logo on top.
	if o.Logo != nil {
		fillRoundRect(canvas, *zone, bgPaint)
		xdraw.CatmullRom.Scale(canvas, logoRect, o.Logo, o.Logo.Bounds(), xdraw.Over, nil)
	}

	// Downscale to final size with a smooth kernel for antialiasing.
	out := image.NewRGBA(image.Rect(0, 0, o.Size, o.Size))
	xdraw.CatmullRom.Scale(out, out.Bounds(), canvas, canvas.Bounds(), xdraw.Src, nil)
	return out
}

// finderOrigins returns the top-left module of each 7x7 finder pattern.
func finderOrigins(n int) [3][2]int {
	return [3][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}}
}

func finderTest(n int) func(x, y int) bool {
	return func(x, y int) bool {
		return (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7)
	}
}
