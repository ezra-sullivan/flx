package imgproc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"time"
)

// ResizeTo returns a transformer that resizes images with a nearest-neighbor
// strategy.
func ResizeTo(width int, height int) ImageTransformer {
	return func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}

		_ = sourceURL

		srcImage, err := decodeImage(imageBytes)
		if err != nil {
			return nil, fmt.Errorf("decode for resize: %w", err)
		}

		resized := resizeNearest(srcImage, width, height)
		return encodeJPEG(resized)
	}
}

// AddWatermarkText overlays a visible footer marker plus a tiny built-in
// bitmap text watermark.
func AddWatermarkText(text string) ImageTransformer {
	return func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}

		srcImage, err := decodeImage(imageBytes)
		if err != nil {
			return nil, fmt.Errorf("decode for watermark: %w", err)
		}

		watermarked := addWatermarkMarker(srcImage, text, sourceURL)
		return encodeJPEG(watermarked)
	}
}

func decodeImage(imageBytes []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, err
	}

	return img, nil
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func resizeNearest(src image.Image, width int, height int) *image.RGBA {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + y*srcHeight/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}
