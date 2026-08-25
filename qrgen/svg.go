package qrgen

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"strings"
)

// GenerateSVG renders the same styled QR as a vector SVG. The logo, if any,
// is embedded as a base64 PNG. The SVG uses a 0..Size viewBox so it scales
// to any resolution.
func GenerateSVG(o Options) (string, *Result, error) {
	if err := o.applyDefaults(); err != nil {
		return "", nil, err
	}

	// Reuse the raster pipeline (with validation and logo-shrink retries) to
	// settle on a logo scale that is proven to decode.
	o.Validate = true
	res, err := generate(o)
	if err != nil {
		return "", nil, err
	}
	matrix := res.Matrix

	n := len(matrix)
	total := n + 2*o.Margin
	size := float64(o.Size)
	mod := size / float64(total)
	fg := hexColor(o.FG)
	bg := hexColor(o.BG)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %g %g">`, o.Size, o.Size, size, size)

	// With a gradient, modules and finders are filled from a shared def so the
	// gradient flows across the whole code, matching the raster output.
	fillRef := fg
	if o.Gradient != "" {
		fillRef = "url(#fgg)"
		b.WriteString("<defs>")
		stops := fmt.Sprintf(`<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>`, fg, hexColor(o.FG2))
		switch o.Gradient {
		case GradientHorizontal:
			fmt.Fprintf(&b, `<linearGradient id="fgg" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="%g" y2="0">%s</linearGradient>`, size, stops)
		case GradientDiagonal:
			fmt.Fprintf(&b, `<linearGradient id="fgg" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="%g" y2="%g">%s</linearGradient>`, size, size, stops)
		case GradientRadial:
			fmt.Fprintf(&b, `<radialGradient id="fgg" gradientUnits="userSpaceOnUse" cx="%g" cy="%g" r="%g">%s</radialGradient>`, size/2, size/2, size*0.71, stops)
		default: // vertical
			fmt.Fprintf(&b, `<linearGradient id="fgg" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="0" y2="%g">%s</linearGradient>`, size, stops)
		}
		b.WriteString("</defs>")
	}

	if _, _, _, a := o.BG.RGBA(); a > 0 {
		fmt.Fprintf(&b, `<rect width="%g" height="%g" fill="%s"/>`, size, size, bg)
	}

	// Logo clear zone geometry (same math as the raster path).
	var zone *roundRect
	var lx, ly, lw, lh float64
	if o.Logo != nil {
		lb := o.Logo.Bounds()
		aspect := float64(lb.Dy()) / float64(lb.Dx())
		lw = res.LogoScale * size
		lh = lw * aspect
		maxH := 0.30 * size
		if lh > maxH {
			lh = maxH
			lw = lh / aspect
		}
		cx, cy := size/2, size/2
		lx, ly = cx-lw/2, cy-lh/2
		pad := 0.9 * mod
		zr := math.Min(lw+2*pad, lh+2*pad) / 2 * 0.9
		zone = &roundRect{cx - lw/2 - pad, cy - lh/2 - pad, cx + lw/2 + pad, cy + lh/2 + pad, zr}
	}

	inFinder := finderTest(n)
	fmt.Fprintf(&b, `<g fill="%s">`, fillRef)
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
				fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s"/>`, f(cx), f(cy), f(mod*0.5))
			case StyleRounded:
				fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s"/>`, f(cx-mod/2), f(cy-mod/2), f(mod), f(mod), f(mod*0.3))
			case StyleSquares:
				fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s"/>`, f(cx-mod/2), f(cy-mod/2), f(mod), f(mod))
			}
		}
	}
	b.WriteString("</g>")

	for _, fo := range finderOrigins(n) {
		fx := (float64(o.Margin) + float64(fo[0])) * mod
		fy := (float64(o.Margin) + float64(fo[1])) * mod
		or, ir, cr := 2.3*mod, 1.7*mod, 1.1*mod
		if o.Style == StyleSquares {
			or, ir, cr = 0, 0, 0
		}
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`, f(fx), f(fy), f(7*mod), f(7*mod), f(or), fillRef)
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`, f(fx+mod), f(fy+mod), f(5*mod), f(5*mod), f(ir), bg)
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`, f(fx+2*mod), f(fy+2*mod), f(3*mod), f(3*mod), f(cr), fillRef)
	}

	if o.Logo != nil {
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`,
			f(zone.x0), f(zone.y0), f(zone.x1-zone.x0), f(zone.y1-zone.y0), f(zone.r), bg)
		mime, data := "image/png", []byte(nil)
		if o.LogoSVG != nil {
			mime, data = "image/svg+xml", o.LogoSVG
		} else {
			var buf bytes.Buffer
			if err := png.Encode(&buf, o.Logo); err != nil {
				return "", nil, fmt.Errorf("encoding logo: %w", err)
			}
			data = buf.Bytes()
		}
		fmt.Fprintf(&b, `<image x="%s" y="%s" width="%s" height="%s" href="data:%s;base64,%s" preserveAspectRatio="xMidYMid meet"/>`,
			f(lx), f(ly), f(lw), f(lh), mime, base64.StdEncoding.EncodeToString(data))
	}
	b.WriteString("</svg>")
	return b.String(), res, nil
}

func f(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

func hexColor(c color.Color) string {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return "none"
	}
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
