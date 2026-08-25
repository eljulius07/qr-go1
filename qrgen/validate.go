package qrgen

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
)

// Verify decodes img as a QR code and checks it contains want.
// Transparent backgrounds are composited over white first, since scanners
// see the code on some surface anyway.
func Verify(img image.Image, want string) error {
	flat := image.NewRGBA(img.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)

	bmp, err := gozxing.NewBinaryBitmapFromImage(flat)
	if err != nil {
		return fmt.Errorf("preparing bitmap: %w", err)
	}
	reader := zxqr.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	res, err := reader.Decode(bmp, hints)
	if err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	if res.GetText() != want {
		return fmt.Errorf("decoded text mismatch: got %q", res.GetText())
	}
	return nil
}
