package ffmpeg

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

// watermarkFontCandidates lists bundled/known sans-serif fonts, tried in order.
// When none is found we fall back to fontconfig's default `font=` lookup.
var watermarkFontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/truetype/freefont/FreeSansBold.ttf",
	"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	"/Library/Fonts/Arial Bold.ttf",
	`C:\Windows\Fonts\arialbd.ttf`,
}

func watermarkFontSpec() string {
	for _, path := range watermarkFontCandidates {
		if _, err := os.Stat(path); err == nil {
			return "fontfile='" + escapeFilterValue(path) + "'"
		}
	}
	return "font='DejaVu Sans'"
}

// escapeFilterValue escapes a value embedded in an ffmpeg filtergraph option.
// The value is emitted wrapped in single quotes, so only backslash and single
// quote need explicit handling (colons, commas and brackets are already literal
// inside single quotes).
func escapeFilterValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `'\''`)
	return value
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// buildWatermarkFilter returns a drawtext filter that burns wm into a segment
// rendered at outW x outH. Timing is relative to the segment's PTS (which the
// export filters reset to start at zero). It returns "" when nothing applies.
func buildWatermarkFilter(wm *models.Watermark, outW, outH int) string {
	if wm == nil || !wm.Enabled {
		return ""
	}
	text := strings.TrimSpace(wm.Text)
	if text == "" || outW <= 0 || outH <= 0 || wm.Duration <= 0 {
		return ""
	}

	boxW := clamp01(wm.Width) * float64(outW)
	boxH := clamp01(wm.Height) * float64(outH)
	if boxW <= 0 || boxH <= 0 {
		return ""
	}

	// Fit the font to the box: bounded by height, and by width so the text
	// (approximated at ~0.62em per rune) does not overflow horizontally.
	runes := utf8.RuneCountInString(text)
	if runes < 1 {
		runes = 1
	}
	fontSize := boxH * 0.78
	if byWidth := boxW / (0.62 * float64(runes)); byWidth < fontSize {
		fontSize = byWidth
	}
	size := int(fontSize + 0.5)
	if size < 8 {
		size = 8
	}

	x := int(clamp01(wm.X)*float64(outW) + 0.5)
	yFrac := clamp01(wm.Y)
	// Vertically center the glyph line inside the watermark box.
	y := int(yFrac*float64(outH) + (boxH-float64(size))/2 + 0.5)
	if y < 0 {
		y = 0
	}

	start := wm.Start
	if start < 0 {
		start = 0
	}
	end := start + wm.Duration

	return fmt.Sprintf(
		"drawtext=%s:expansion=none:text='%s':x=%d:y=%d:fontsize=%d:fontcolor=white@0.85:shadowcolor=black@0.65:shadowx=2:shadowy=2:enable='between(t,%.3f,%.3f)'",
		watermarkFontSpec(),
		escapeFilterValue(text),
		x,
		y,
		size,
		start,
		end,
	)
}

// appendWatermarkFilter adds the drawtext filter to a per-segment filter chain
// when the segment carries an enabled watermark.
func appendWatermarkFilter(filter string, wm *models.Watermark, outW, outH int) string {
	wmFilter := buildWatermarkFilter(wm, outW, outH)
	if wmFilter == "" {
		return filter
	}
	if filter == "" {
		return wmFilter
	}
	return filter + "," + wmFilter
}
