package qrgen

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// roundRect is an axis-aligned rectangle with rounded corners of radius r,
// in float pixel coordinates.
type roundRect struct {
	x0, y0, x1, y1, r float64
}

// contains reports whether point (px,py), grown by pad, falls inside the shape.
func (rr *roundRect) contains(px, py, pad float64) bool {
	g := roundRect{rr.x0 - pad, rr.y0 - pad, rr.x1 + pad, rr.y1 + pad, rr.r + pad}
	return g.hit(px, py)
}

func (rr *roundRect) hit(px, py float64) bool {
	if px < rr.x0 || px > rr.x1 || py < rr.y0 || py > rr.y1 {
		return false
	}
	r := math.Min(rr.r, math.Min((rr.x1-rr.x0)/2, (rr.y1-rr.y0)/2))
	if r <= 0 {
		return true
	}
	// Distance check only matters inside the corner squares.
	cx := clampF(px, rr.x0+r, rr.x1-r)
	cy := clampF(py, rr.y0+r, rr.y1-r)
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= r*r
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func fillRect(img *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// fillRectP fills a rectangle with a painter, using the fast path for
// uniform colors.
func fillRectP(img *image.RGBA, r image.Rectangle, p painter) {
	if u, ok := p.(uniformPaint); ok {
		fillRect(img, r, u.c)
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, p.at(float64(x)+0.5, float64(y)+0.5))
		}
	}
}

func fillCircle(img *image.RGBA, cx, cy, r float64, p painter) {
	x0, y0 := int(cx-r), int(cy-r)
	x1, y1 := int(math.Ceil(cx+r)), int(math.Ceil(cy+r))
	r2 := r * r
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			dx, dy := px-cx, py-cy
			if dx*dx+dy*dy <= r2 {
				img.Set(x, y, p.at(px, py))
			}
		}
	}
}

func fillRoundRect(img *image.RGBA, rr roundRect, p painter) {
	r := math.Min(rr.r, math.Min((rr.x1-rr.x0)/2, (rr.y1-rr.y0)/2))
	if r <= 0 {
		fillRectP(img, image.Rect(int(rr.x0), int(rr.y0), int(math.Ceil(rr.x1)), int(math.Ceil(rr.y1))), p)
		return
	}
	// Center cross as fast rect fills, then the four corner squares per-pixel.
	fillRectP(img, image.Rect(int(rr.x0+r), int(rr.y0), int(math.Ceil(rr.x1-r)), int(math.Ceil(rr.y1))), p)
	fillRectP(img, image.Rect(int(rr.x0), int(rr.y0+r), int(math.Ceil(rr.x1)), int(math.Ceil(rr.y1-r))), p)
	corners := [4][2]float64{
		{rr.x0 + r, rr.y0 + r}, {rr.x1 - r, rr.y0 + r},
		{rr.x0 + r, rr.y1 - r}, {rr.x1 - r, rr.y1 - r},
	}
	r2 := r * r
	for _, cn := range corners {
		x0, y0 := int(cn[0]-r), int(cn[1]-r)
		x1, y1 := int(math.Ceil(cn[0]+r)), int(math.Ceil(cn[1]+r))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				px := float64(x) + 0.5
				py := float64(y) + 0.5
				// Only paint pixels that belong to the rounded shape.
				if px >= rr.x0 && px <= rr.x1 && py >= rr.y0 && py <= rr.y1 {
					dx, dy := px-cn[0], py-cn[1]
					inCornerSquare := (px < rr.x0+r || px > rr.x1-r) && (py < rr.y0+r || py > rr.y1-r)
					if !inCornerSquare || dx*dx+dy*dy <= r2 {
						img.Set(x, y, p.at(px, py))
					}
				}
			}
		}
	}
}
