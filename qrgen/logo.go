package qrgen

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	_ "golang.org/x/image/webp"
)

// ParseLogo decodes logo bytes in PNG, JPEG, GIF, WebP or SVG format.
// For SVG input it returns both a rasterized image (used for PNG rendering
// and geometry) and the raw SVG bytes (embedded verbatim in SVG output).
func ParseLogo(data []byte) (image.Image, []byte, error) {
	if looksLikeSVG(data) {
		img, err := rasterizeSVG(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing svg logo: %w", err)
		}
		return img, data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("decoding logo: %w (use png, jpeg, gif, webp or svg)", err)
	}
	return img, nil, nil
}

func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	s := strings.TrimSpace(strings.TrimPrefix(string(head), "\xef\xbb\xbf"))
	return strings.HasPrefix(s, "<svg") ||
		(strings.HasPrefix(s, "<?xml") && strings.Contains(s, "<svg")) ||
		(strings.HasPrefix(s, "<!--") && strings.Contains(s, "<svg"))
}

// rasterizeSVG renders an SVG to a 768px-wide RGBA image (height per aspect).
func rasterizeSVG(data []byte) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	vw, vh := icon.ViewBox.W, icon.ViewBox.H
	if vw <= 0 || vh <= 0 {
		return nil, fmt.Errorf("svg has no usable viewBox/size")
	}
	w := 768
	h := int(float64(w) * vh / vw)
	if h < 1 {
		h = 1
	}
	if h > 2048 {
		h = 2048
		w = int(float64(h) * vw / vh)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	icon.SetTarget(0, 0, float64(w), float64(h))
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(w, h, scanner), 1.0)
	return img, nil
}
