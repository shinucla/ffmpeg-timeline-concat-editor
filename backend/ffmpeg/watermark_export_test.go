package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

func sampleInput(t *testing.T) string {
	t.Helper()
	input := filepath.Join("..", "..", "sample-videos", "red-10s.mp4")
	if _, err := os.Stat(input); err != nil {
		t.Skip("sample video not available")
	}
	return input
}

func TestBuildWatermarkFilter(t *testing.T) {
	if got := buildWatermarkFilter(nil, 640, 360); got != "" {
		t.Fatalf("nil watermark should be empty, got %q", got)
	}
	if got := buildWatermarkFilter(&models.Watermark{Enabled: true, Text: "  "}, 640, 360); got != "" {
		t.Fatalf("empty text should be empty, got %q", got)
	}
	got := buildWatermarkFilter(&models.Watermark{
		Enabled: true, Text: "Hello", X: 0.1, Y: 0.2, Width: 0.4, Height: 0.2,
		Start: 1, Duration: 2,
	}, 640, 360)
	t.Logf("filter: %s", got)
	if got == "" {
		t.Fatal("expected a filter")
	}
	if !strings.Contains(got, "enable='between(t,1.000,3.000)'") {
		t.Fatalf("missing enable window: %s", got)
	}
	if !strings.Contains(got, "text='Hello'") {
		t.Fatalf("missing text: %s", got)
	}
}

func TestWatermarkTimelineRoundTrip(t *testing.T) {
	entries := []TimelineFileEntry{
		{
			Start: 1, End: 5, Filename: "clip one.mp4", Repeat: 2, RotationSteps: 1,
			Watermark: &models.Watermark{
				Enabled: true, Text: "hello: world", X: 0.1, Y: 0.2, Width: 0.3,
				Height: 0.15, Start: 0.5, Duration: 2,
			},
		},
		{Start: 6, End: 9, Filename: "clip2.mp4", Repeat: 1},
	}
	path := filepath.Join(t.TempDir(), "timeline.txt")
	if err := WriteTimelineFile(path, "cache-abc", entries); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	t.Logf("timeline:\n%s", data)

	parsed, err := ParseTimelineText(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CacheFolder != "cache-abc" {
		t.Fatalf("cache folder = %q", parsed.CacheFolder)
	}
	if len(parsed.Entries) != 2 {
		t.Fatalf("entries = %d", len(parsed.Entries))
	}
	wm := parsed.Entries[0].Watermark
	if wm == nil || !wm.Enabled {
		t.Fatalf("watermark missing on first entry: %+v", parsed.Entries[0])
	}
	if wm.Text != "hello: world" {
		t.Fatalf("text = %q", wm.Text)
	}
	if wm.Start != 0.5 || wm.Duration != 2 || wm.X != 0.1 || wm.Height != 0.15 {
		t.Fatalf("watermark values = %+v", wm)
	}
	if parsed.Entries[1].Watermark != nil {
		t.Fatalf("second entry should have no watermark")
	}
}

func TestWatermarkExportSingle(t *testing.T) {
	input := sampleInput(t)
	out := filepath.Join(t.TempDir(), "single.mp4")
	wm := &models.Watermark{
		Enabled: true, Text: `Hello: it's 100% "ok", [x];`, Start: 0.5, Duration: 2,
		X: 0.2, Y: 0.2, Width: 0.5, Height: 0.2,
	}
	seg := ExportSegment{InputPath: input, Start: 1, End: 5, Watermark: wm}
	if err := runExportBatch([]ExportSegment{seg}, out, exportNormalize{}, 4, nil); err != nil {
		t.Fatalf("single export failed: %v", err)
	}
	meta, err := Probe(out)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Duration < 1 {
		t.Fatalf("unexpected duration %v", meta.Duration)
	}
}

func TestWatermarkExportMulti(t *testing.T) {
	input := sampleInput(t)
	out := filepath.Join(t.TempDir(), "multi.mp4")
	wm := &models.Watermark{
		Enabled: true, Text: "Segment One", Start: 0, Duration: 3,
		X: 0.6, Y: 0.7, Width: 0.35, Height: 0.15,
	}
	segs := []ExportSegment{
		{InputPath: input, Start: 0, End: 3, Watermark: wm},
		{InputPath: input, Start: 3, End: 6},
	}
	if err := runExportBatch(segs, out, exportNormalize{}, 6, nil); err != nil {
		t.Fatalf("multi export failed: %v", err)
	}
	meta, err := Probe(out)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Duration < 3 {
		t.Fatalf("unexpected duration %v", meta.Duration)
	}
}
