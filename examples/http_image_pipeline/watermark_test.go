package main

import (
	"image"
	"image/color"
	"testing"
)

func TestAddWatermarkMarkerDrawsBrightTextPixels(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 320, 240))
	fill := color.NRGBA{R: 8, G: 8, B: 8, A: 255}
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			src.Set(x, y, fill)
		}
	}

	dst := addWatermarkMarker(src, "flx-demo", "https://example.com/images/42")

	brightPixels := 0
	for y := dst.Bounds().Dy() - 48; y < dst.Bounds().Dy(); y++ {
		for x := 96; x < dst.Bounds().Dx()-12; x++ {
			r, g, b, _ := dst.At(x, y).RGBA()
			if r > 0xd800 && g > 0xd800 && b > 0xd800 {
				brightPixels++
			}
		}
	}

	if brightPixels == 0 {
		t.Fatal("expected watermark text to add bright pixels in the footer")
	}
}
