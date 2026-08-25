package qrgen

import (
	"fmt"
	"image/color"
	"math"
)

// Gradient directions accepted in Options.Gradient.
const (
	GradientVertical   = "vertical"
	GradientHorizontal = "horizontal"
	GradientDiagonal   = "diagonal"
	GradientRadial     = "radial"
)

// painter yields a color for a pixel position, letting shapes be filled with
// a flat color or a smooth gradient.
type painter interface {
	at(x, y float64) color.Color
}

type uniformPaint struct{ c color.Color }

func (u uniformPaint) at(_, _ float64) color.Color { return u.c }

// linearPaint interpolates c1→c2 along the vector (x0,y0)→(x1,y1).
type linearPaint struct {
	c1, c2         color.RGBA
	x0, y0, dx, dy float64 // dx,dy: direction vector; len2 its squared length
	len2           float64
}

func (l linearPaint) at(x, y float64) color.Color {
	t := ((x-l.x0)*l.dx + (y-l.y0)*l.dy) / l.len2
	return lerpRGBA(l.c1, l.c2, clampF(t, 0, 1))
}

// radialPaint interpolates c1 (center) → c2 (radius r).
type radialPaint struct {
	c1, c2    color.RGBA
	cx, cy, r float64
}

func (rp radialPaint) at(x, y float64) color.Color {
	d := math.Hypot(x-rp.cx, y-rp.cy) / rp.r
	return lerpRGBA(rp.c1, rp.c2, clampF(d, 0, 1))
}

func lerpRGBA(a, b color.RGBA, t float64) color.Color {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: uint8(float64(a.A) + (float64(b.A)-float64(a.A))*t),
	}
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// newPainter builds the foreground painter for a canvas of side px.
func newPainter(o *Options, side float64) (painter, error) {
	if o.Gradient == "" {
		return uniformPaint{o.FG}, nil
	}
	c1, c2 := toRGBA(o.FG), toRGBA(o.FG2)
	switch o.Gradient {
	case GradientVertical:
		return linearPaint{c1: c1, c2: c2, dx: 0, dy: side, len2: side * side}, nil
	case GradientHorizontal:
		return linearPaint{c1: c1, c2: c2, dx: side, dy: 0, len2: side * side}, nil
	case GradientDiagonal:
		return linearPaint{c1: c1, c2: c2, dx: side, dy: side, len2: 2 * side * side}, nil
	case GradientRadial:
		return radialPaint{c1: c1, c2: c2, cx: side / 2, cy: side / 2, r: side * 0.71}, nil
	}
	return nil, fmt.Errorf("unknown gradient %q (use vertical, horizontal, diagonal or radial)", o.Gradient)
}
