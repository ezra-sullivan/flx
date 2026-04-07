package main

import (
	"image"
	"image/color"
	stdDraw "image/draw"
	"strings"
)

const (
	watermarkGlyphWidth   = 5
	watermarkGlyphHeight  = 7
	watermarkGlyphSpacing = 1
)

var watermarkGlyphs = map[rune][watermarkGlyphHeight]string{
	' ': {"00000", "00000", "00000", "00000", "00000", "00000", "00000"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'.': {"00000", "00000", "00000", "00000", "00000", "01100", "01100"},
	'/': {"00001", "00010", "00100", "01000", "10000", "00000", "00000"},
	':': {"00000", "01100", "01100", "00000", "01100", "01100", "00000"},
	'?': {"01110", "10001", "00001", "00010", "00100", "00000", "00100"},
	'_': {"00000", "00000", "00000", "00000", "00000", "00000", "11111"},
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
	'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
}

// addWatermarkMarker draws a translucent footer bar and a text label so the
// processed image is visibly distinct from the original.
func addWatermarkMarker(src image.Image, text string, sourceURL string) *image.RGBA {
	_ = sourceURL

	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stdDraw.Draw(dst, dst.Bounds(), src, bounds.Min, stdDraw.Src)

	barHeight := max(bounds.Dy()/6, 24)
	barRect := image.Rect(0, bounds.Dy()-barHeight, bounds.Dx(), bounds.Dy())
	barColor := color.NRGBA{R: 24, G: 24, B: 24, A: 150}
	stdDraw.Draw(dst, barRect, image.NewUniform(barColor), image.Point{}, stdDraw.Over)

	textX := 12
	label, scale := watermarkTextLayout(watermarkLabel(text), barHeight, bounds.Dx()-textX-12)
	if scale > 0 {
		textY := barRect.Min.Y + max((barHeight-watermarkGlyphHeight*scale)/2, 0)
		drawWatermarkText(dst, label, textX, textY, scale)
	}

	return dst
}

func watermarkLabel(text string) string {
	label := strings.ToUpper(strings.TrimSpace(text))
	if label == "" {
		return "FLX"
	}

	return label
}

func watermarkTextLayout(label string, barHeight int, availableWidth int) (string, int) {
	if availableWidth <= 0 {
		return "", 0
	}

	label = ellipsizeWatermarkLabel(label, availableWidth)
	if label == "" {
		return "", 0
	}

	scale := max((barHeight-12)/watermarkGlyphHeight, 1)
	for scale > 1 && watermarkTextWidth(label, scale) > availableWidth {
		scale--
	}
	if watermarkTextWidth(label, scale) > availableWidth {
		return "", 0
	}

	return label, scale
}

func ellipsizeWatermarkLabel(label string, availableWidth int) string {
	if watermarkTextWidth(label, 1) <= availableWidth {
		return label
	}

	const ellipsis = "..."
	if watermarkTextWidth(ellipsis, 1) > availableWidth {
		return ""
	}

	runes := []rune(label)
	for keep := len(runes) - 1; keep > 0; keep-- {
		candidate := string(runes[:keep]) + ellipsis
		if watermarkTextWidth(candidate, 1) <= availableWidth {
			return candidate
		}
	}

	return ellipsis
}

func watermarkTextWidth(label string, scale int) int {
	if label == "" || scale <= 0 {
		return 0
	}

	width := 0
	for range label {
		if width > 0 {
			width += watermarkGlyphSpacing * scale
		}
		width += watermarkGlyphWidth * scale
	}

	return width
}

func drawWatermarkText(dst *image.RGBA, label string, x int, y int, scale int) {
	if label == "" || scale <= 0 {
		return
	}

	shadowOffset := max(scale/2, 1)
	shadow := color.NRGBA{R: 0, G: 0, B: 0, A: 180}
	fill := color.NRGBA{R: 248, G: 248, B: 248, A: 235}
	cursorX := x

	for _, r := range label {
		glyph := watermarkGlyphForRune(r)
		drawWatermarkGlyph(dst, glyph, cursorX+shadowOffset, y+shadowOffset, scale, shadow)
		drawWatermarkGlyph(dst, glyph, cursorX, y, scale, fill)
		cursorX += (watermarkGlyphWidth + watermarkGlyphSpacing) * scale
	}
}

func watermarkGlyphForRune(r rune) [watermarkGlyphHeight]string {
	glyph, ok := watermarkGlyphs[r]
	if ok {
		return glyph
	}

	return watermarkGlyphs['?']
}

func drawWatermarkGlyph(
	dst *image.RGBA,
	glyph [watermarkGlyphHeight]string,
	x int,
	y int,
	scale int,
	fill color.NRGBA,
) {
	for row, line := range glyph {
		for col, bit := range line {
			if bit != '1' {
				continue
			}

			rect := image.Rect(
				x+col*scale,
				y+row*scale,
				x+(col+1)*scale,
				y+(row+1)*scale,
			)
			stdDraw.Draw(dst, rect, image.NewUniform(fill), image.Point{}, stdDraw.Over)
		}
	}
}
