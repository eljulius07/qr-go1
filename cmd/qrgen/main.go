// Command qrgen is a CLI for generating styled QR codes.
//
//	qrgen -value "https://pass2fun.com" -logo logo.png -out qr.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"strings"

	"qr-go1/qrgen"
)

func main() {
	value := flag.String("value", "", "text or URL to encode (required)")
	logoPath := flag.String("logo", "", "path to logo image (png/jpeg/gif/webp/svg)")
	out := flag.String("out", "qr.png", "output file (.png or .svg)")
	size := flag.Int("size", 1024, "output size in px")
	style := flag.String("style", "dots", "module style: dots, rounded, squares")
	fg := flag.String("fg", "#000000", "foreground color (hex)")
	fg2 := flag.String("fg2", "", "second foreground color for gradients (hex)")
	gradient := flag.String("gradient", "", "gradient direction: vertical, horizontal, diagonal, radial")
	bg := flag.String("bg", "#ffffff", "background color (hex, or 'transparent')")
	logoScale := flag.Float64("logo-scale", 0.22, "logo width as fraction of image (0.10-0.30)")
	margin := flag.Int("margin", 3, "quiet zone in modules")
	flag.Parse()

	if *value == "" {
		flag.Usage()
		os.Exit(2)
	}

	opts := qrgen.Options{
		Value:     *value,
		Size:      *size,
		Style:     qrgen.Style(*style),
		Gradient:  *gradient,
		LogoScale: *logoScale,
		Margin:    *margin,
	}
	var err error
	if opts.FG, err = qrgen.ParseColor(*fg); err != nil {
		log.Fatalf("invalid -fg: %v", err)
	}
	if *fg2 != "" {
		if opts.FG2, err = qrgen.ParseColor(*fg2); err != nil {
			log.Fatalf("invalid -fg2: %v", err)
		}
	}
	if opts.BG, err = qrgen.ParseColor(*bg); err != nil {
		log.Fatalf("invalid -bg: %v", err)
	}
	if *logoPath != "" {
		data, err := os.ReadFile(*logoPath)
		if err != nil {
			log.Fatal(err)
		}
		img, svgRaw, err := qrgen.ParseLogo(data)
		if err != nil {
			log.Fatal(err)
		}
		opts.Logo, opts.LogoSVG = img, svgRaw
	}

	if strings.HasSuffix(strings.ToLower(*out), ".svg") {
		svg, res, err := qrgen.GenerateSVG(opts)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*out, []byte(svg), 0o644); err != nil {
			log.Fatal(err)
		}
		report(*out, res)
		return
	}

	res, err := qrgen.Generate(opts)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, res.Image); err != nil {
		log.Fatal(err)
	}
	report(*out, res)
}

func report(path string, res *qrgen.Result) {
	fmt.Printf("wrote %s (validated=%v, logo_scale=%.3f)\n", path, res.Validated, res.LogoScale)
}
