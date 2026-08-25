package qrgen

import (
	"fmt"
	"image/color"
	"strings"
)

// ParseColor parses "#rgb", "#rrggbb", "#rrggbbaa" (with or without '#'),
// or the keyword "transparent".
func ParseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil, fmt.Errorf("empty color")
	}
	if s == "transparent" || s == "none" {
		return color.RGBA{}, nil
	}
	s = strings.TrimPrefix(s, "#")
	var r, g, b, a uint8 = 0, 0, 0, 255
	switch len(s) {
	case 3:
		if _, err := fmt.Sscanf(s, "%1x%1x%1x", &r, &g, &b); err != nil {
			return nil, fmt.Errorf("invalid color %q", s)
		}
		r, g, b = r*17, g*17, b*17
	case 6:
		if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
			return nil, fmt.Errorf("invalid color %q", s)
		}
	case 8:
		if _, err := fmt.Sscanf(s, "%02x%02x%02x%02x", &r, &g, &b, &a); err != nil {
			return nil, fmt.Errorf("invalid color %q", s)
		}
	default:
		return nil, fmt.Errorf("invalid color %q (use #rgb, #rrggbb or #rrggbbaa)", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: a}, nil
}
