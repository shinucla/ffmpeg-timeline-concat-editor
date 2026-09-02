package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

// exportMetadataBatchSize caps parallel inputs per FFmpeg filter_complex export.
// Keep this low: each input is decoded concurrently and reverse/rotation spike RAM.
const exportMetadataBatchSize = 16

// ExportSegment is one timeline row (after repeat expansion) read directly from source files.
type ExportSegment struct {
	InputPath     string
	Start         float64
	End           float64
	RotationSteps int
	Reverse       bool
}

func formatFFmpegSeekTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	fracMs := int((seconds - float64(total)) * 1000)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if fracMs > 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, fracMs)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func segmentRepeatCount(seg models.Segment) int {
	if seg.Repeat < 1 {
		return 1
	}
	return seg.Repeat
}

func buildExportSegments(videoRoot string, req models.ProcessRequest) ([]ExportSegment, []segmentManifestEntry, error) {
	if len(req.Clips) == 0 {
		return nil, nil, fmt.Errorf("no clips to process")
	}

	var exportSegs []ExportSegment
	var manifestEntries []segmentManifestEntry

	for _, clip := range req.Clips {
		rel, err := DecodeVideoID(clip.VideoID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid video id %s: %w", clip.VideoID, err)
		}
		inputPath, err := ResolveVideoPath(videoRoot, rel)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve video %s: %w", clip.VideoID, err)
		}
		filename := filepath.Base(rel)

		for _, seg := range clip.Segments {
			if seg.End <= seg.Start {
				continue
			}
			manifestEntries = append(manifestEntries, segmentManifestEntry{
				Start:                  seg.Start,
				End:                    seg.End,
				Filename:               filename,
				Repeat:                 seg.Repeat,
				RotationSteps:          seg.RotationSteps,
				AlternateRepeatReverse: seg.AlternateRepeatReverse,
			})
			for i := 0; i < segmentRepeatCount(seg); i++ {
				reverse := seg.AlternateRepeatReverse && (i+1)%2 == 0
				exportSegs = append(exportSegs, ExportSegment{
					InputPath:     inputPath,
					Start:         seg.Start,
					End:           seg.End,
					RotationSteps: seg.RotationSteps,
					Reverse:       reverse,
				})
			}
		}
	}

	if len(exportSegs) == 0 {
		return nil, nil, fmt.Errorf("no segments to process")
	}
	return exportSegs, manifestEntries, nil
}

func buildCutPartSegments(videoRoot string, req models.ProcessRequest) ([]ExportSegment, error) {
	if len(req.Clips) == 0 {
		return nil, fmt.Errorf("no clips to process")
	}

	var parts []ExportSegment
	for _, clip := range req.Clips {
		rel, err := DecodeVideoID(clip.VideoID)
		if err != nil {
			return nil, fmt.Errorf("invalid video id %s: %w", clip.VideoID, err)
		}
		inputPath, err := ResolveVideoPath(videoRoot, rel)
		if err != nil {
			return nil, fmt.Errorf("resolve video %s: %w", clip.VideoID, err)
		}

		for _, seg := range clip.Segments {
			if seg.End <= seg.Start {
				continue
			}
			parts = append(parts, ExportSegment{
				InputPath:     inputPath,
				Start:         seg.Start,
				End:           seg.End,
				RotationSteps: seg.RotationSteps,
			})
		}
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no segments to process")
	}
	return parts, nil
}

func cutPartOutputPath(finalOutput string, index int) string {
	base := strings.TrimSuffix(finalOutput, filepath.Ext(finalOutput))
	return fmt.Sprintf("%s-part%03d.mp4", base, index)
}

func ExportCutParts(
	segments []ExportSegment,
	finalOutput string,
	onProgress func(ratio float64, message string),
) error {
	total := len(segments)
	for i, seg := range segments {
		out := cutPartOutputPath(finalOutput, i+1)
		partIndex := i + 1
		baseRatio := float64(i) / float64(total)
		duration := seg.End - seg.Start
		if duration <= 0 {
			duration = 1
		}

		if onProgress != nil {
			onProgress(baseRatio, fmt.Sprintf("Exporting cut part %d/%d", partIndex, total))
		}

		err := runExportBatch([]ExportSegment{seg}, out, exportNormalize{}, duration, func(partRatio float64) {
			if onProgress == nil {
				return
			}
			overall := baseRatio + partRatio/float64(total)
			if overall > 1 {
				overall = 1
			}
			onProgress(overall, fmt.Sprintf("Exporting cut part %d/%d: %d%%", partIndex, total, int(partRatio*100)))
		})
		if err != nil {
			return fmt.Errorf("export cut part %d: %w", partIndex, err)
		}
	}

	if onProgress != nil {
		onProgress(1, fmt.Sprintf("Exported %d cut parts", total))
	}
	return nil
}

func exportTotalDuration(segments []ExportSegment) float64 {
	total := 0.0
	for _, seg := range segments {
		if seg.End > seg.Start {
			total += seg.End - seg.Start
		}
	}
	return total
}

func exportBatchDuration(segments []ExportSegment) float64 {
	total := exportTotalDuration(segments)
	if total <= 0 {
		return 1
	}
	return total
}

func exportRotationSteps(steps int) int {
	steps %= 4
	if steps < 0 {
		steps += 4
	}
	return steps
}

func rotationVideoFilter(steps int) string {
	switch exportRotationSteps(steps) {
	case 1:
		return "transpose=1,"
	case 2:
		return "transpose=1,transpose=1,"
	case 3:
		return "transpose=2,"
	default:
		return ""
	}
}

func reverseVideoFilter(reverse bool) string {
	if reverse {
		return "reverse,"
	}
	return ""
}

func segmentAudioFilter(seg ExportSegment, normalize bool) string {
	var parts []string
	if normalize {
		parts = append(parts, "aresample=48000")
	}
	if seg.Reverse {
		parts = append(parts, "areverse")
	}
	parts = append(parts, "asetpts=PTS-STARTPTS")
	return strings.Join(parts, ",")
}

func segmentEffectiveSize(seg ExportSegment) (int, int, error) {
	meta, err := Probe(seg.InputPath)
	if err != nil {
		return 0, 0, err
	}
	if meta.Width <= 0 || meta.Height <= 0 {
		return 0, 0, fmt.Errorf("segment %s has no video dimensions", filepath.Base(seg.InputPath))
	}
	width, height := meta.Width, meta.Height
	switch exportRotationSteps(seg.RotationSteps) {
	case 1, 3:
		width, height = height, width
	}
	return width, height, nil
}

func exportSegmentsSameDimensions(segments []ExportSegment) (bool, error) {
	if len(segments) == 0 {
		return false, nil
	}
	firstW, firstH, err := segmentEffectiveSize(segments[0])
	if err != nil {
		return false, err
	}
	for _, seg := range segments[1:] {
		width, height, err := segmentEffectiveSize(seg)
		if err != nil {
			return false, err
		}
		if width != firstW || height != firstH {
			return false, nil
		}
	}
	return true, nil
}

func exportCanvasSize(segments []ExportSegment) (int, int, error) {
	maxW, maxH := 0, 0
	for _, seg := range segments {
		width, height, err := segmentEffectiveSize(seg)
		if err != nil {
			return 0, 0, err
		}
		if width > maxW {
			maxW = width
		}
		if height > maxH {
			maxH = height
		}
	}
	return evenDimension(maxW), evenDimension(maxH), nil
}

func appendSeekInputs(args []string, segments []ExportSegment) []string {
	for _, seg := range segments {
		args = append(args,
			"-ss", formatFFmpegSeekTime(seg.Start),
			"-to", formatFFmpegSeekTime(seg.End),
			"-i", seg.InputPath,
		)
	}
	return args
}

func buildSingleVideoFilter(seg ExportSegment, normalize exportNormalize) string {
	rotation := rotationVideoFilter(seg.RotationSteps)
	reverse := reverseVideoFilter(seg.Reverse)
	if normalize.Apply {
		return rotation + reverse + buildSingleScaleVideoFilter(normalize.CanvasW, normalize.CanvasH)
	}
	if rotation == "" && reverse == "" {
		return "setpts=PTS-STARTPTS"
	}
	return rotation + reverse + "setpts=PTS-STARTPTS"
}

func buildExportPTSResetFilter(segments []ExportSegment) string {
	var filters strings.Builder
	for i, seg := range segments {
		rotation := rotationVideoFilter(seg.RotationSteps)
		reverse := reverseVideoFilter(seg.Reverse)
		filters.WriteString(fmt.Sprintf("[%d:v]%s%ssetpts=PTS-STARTPTS[v%d];", i, rotation, reverse, i))
		filters.WriteString(fmt.Sprintf("[%d:a]%s[a%d];", i, segmentAudioFilter(seg, false), i))
	}
	var concatInputs strings.Builder
	for i := 0; i < len(segments); i++ {
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	filters.WriteString(fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concatInputs.String(), len(segments)))
	return filters.String()
}

func buildExportScaleFilter(segments []ExportSegment, canvasW, canvasH int) string {
	var filters strings.Builder
	for i, seg := range segments {
		rotation := rotationVideoFilter(seg.RotationSteps)
		reverse := reverseVideoFilter(seg.Reverse)
		filters.WriteString(fmt.Sprintf(
			"[%d:v]%s%sscale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1,setpts=PTS-STARTPTS[v%d];",
			i, rotation, reverse, canvasW, canvasH, canvasW, canvasH, i,
		))
		filters.WriteString(fmt.Sprintf("[%d:a]%s[a%d];", i, segmentAudioFilter(seg, true), i))
	}
	var concatInputs strings.Builder
	for i := 0; i < len(segments); i++ {
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	filters.WriteString(fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concatInputs.String(), len(segments)))
	return filters.String()
}

func buildSingleScaleVideoFilter(canvasW, canvasH int) string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1,setpts=PTS-STARTPTS",
		canvasW, canvasH, canvasW, canvasH,
	)
}

func runExportBatch(
	segments []ExportSegment,
	output string,
	normalize exportNormalize,
	durationSec float64,
	onRatio func(float64),
) error {
	n := len(segments)
	if n == 0 {
		return fmt.Errorf("empty export batch")
	}

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostats"}
	args = appendSeekInputs(args, segments)

	if n == 1 {
		seg := segments[0]
		args = append(args, "-vf", buildSingleVideoFilter(seg, normalize))
		args = append(args, "-af", segmentAudioFilter(seg, normalize.Apply))
	} else {
		var filter string
		if normalize.Apply {
			filter = buildExportScaleFilter(segments, normalize.CanvasW, normalize.CanvasH)
		} else {
			filter = buildExportPTSResetFilter(segments)
		}
		args = append(args,
			"-filter_complex", filter,
			"-map", "[outv]",
			"-map", "[outa]",
		)
	}

	args = append(args,
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "fast",
		"-c:a", "aac",
		"-b:a", "192k",
		"-fps_mode", "passthrough",
		output,
	)

	if onRatio == nil {
		return runFFmpeg(args...)
	}
	return runFFmpegWithTimeProgress(args, durationSec, onRatio)
}

type exportNormalize struct {
	Apply   bool
	CanvasW int
	CanvasH int
}

func resolveExportNormalize(segments []ExportSegment) (exportNormalize, error) {
	sameSize, err := exportSegmentsSameDimensions(segments)
	if err != nil {
		return exportNormalize{}, err
	}
	if sameSize {
		return exportNormalize{}, nil
	}
	canvasW, canvasH, err := exportCanvasSize(segments)
	if err != nil {
		return exportNormalize{}, err
	}
	return exportNormalize{Apply: true, CanvasW: canvasW, CanvasH: canvasH}, nil
}

func splitExportMetadataBatches(segments []ExportSegment) [][]ExportSegment {
	var batches [][]ExportSegment
	for i := 0; i < len(segments); {
		batchSize := exportMetadataBatchSize
		if segments[i].Reverse {
			batchSize = 1
		} else {
			for j := i + 1; j < i+batchSize && j < len(segments); j++ {
				if segments[j].Reverse {
					batchSize = j - i
					break
				}
			}
		}
		end := i + batchSize
		if end > len(segments) {
			end = len(segments)
		}
		batches = append(batches, segments[i:end])
		i = end
	}
	return batches
}

func exportMetadataBatches(
	segments []ExportSegment,
	workDir string,
	normalize exportNormalize,
	onProgress ConcatProgressFn,
) ([]string, error) {
	batches := splitExportMetadataBatches(segments)
	batchCount := len(batches)
	var batchPaths []string
	var temps []string

	for _, batch := range batches {
		batchIndex := len(batchPaths) + 1
		out := filepath.Join(workDir, fmt.Sprintf("batch-%03d.mp4", batchIndex))
		batchPaths = append(batchPaths, out)
		temps = append(temps, out)

		batchDuration := exportBatchDuration(batch)
		baseRatio := float64(len(batchPaths)-1) / float64(batchCount)

		if onProgress != nil {
			onProgress(
				baseRatio,
				fmt.Sprintf("Exporting batch %d/%d (%d segments from timeline)", batchIndex, batchCount, len(batch)),
			)
		}

		err := runExportBatch(batch, out, normalize, batchDuration, func(batchRatio float64) {
			if onProgress == nil {
				return
			}
			overall := baseRatio + batchRatio/float64(batchCount)
			if overall > 1 {
				overall = 1
			}
			onProgress(overall, fmt.Sprintf("Export batch %d/%d: %d%%", batchIndex, batchCount, int(batchRatio*100)))
		})
		if err != nil {
			for _, temp := range temps {
				_ = os.Remove(temp)
			}
			return nil, fmt.Errorf("export batch %d: %w", batchIndex, err)
		}
	}

	return batchPaths, nil
}

// ExportTimeline renders the final output directly from timeline metadata using input
// seek (-ss/-to before -i) plus PTS reset and optional resolution normalization.
// Unlike trim-in-filter, input seek only reads the segment byte range from disk.
func ExportTimeline(
	segments []ExportSegment,
	output string,
	workDir string,
	totalDurationSec float64,
	onProgress ConcatProgressFn,
) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to export")
	}

	normalize, err := resolveExportNormalize(segments)
	if err != nil {
		return err
	}

	runWithProgress := func(duration float64, startMsg string) error {
		if duration <= 0 {
			duration = totalDurationSec
		}
		if onProgress != nil && startMsg != "" {
			onProgress(0, startMsg)
		}
		err := runExportBatch(segments, output, normalize, duration, func(ratio float64) {
			if onProgress != nil {
				onProgress(ratio, fmt.Sprintf("Exporting %d%%", int(ratio*100)))
			}
		})
		if err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(1, "Export complete")
		}
		return nil
	}

	if len(segments) == 1 {
		return runWithProgress(exportBatchDuration(segments), "Exporting single segment from timeline")
	}

	batches := splitExportMetadataBatches(segments)
	if len(batches) == 1 {
		msg := fmt.Sprintf("Exporting %d segments from timeline (PTS reset)", len(segments))
		if normalize.Apply {
			msg = fmt.Sprintf("Exporting %d segments from timeline (PTS reset + resolution normalize)", len(segments))
		}
		return runWithProgress(exportTotalDuration(segments), msg)
	}

	if onProgress != nil {
		onProgress(0, fmt.Sprintf("Exporting %d segments in metadata batches", len(segments)))
	}

	batchPaths, err := exportMetadataBatches(segments, workDir, normalize, func(ratio float64, message string) {
		if onProgress != nil {
			onProgress(ratio*0.85, message)
		}
	})
	if err != nil {
		return err
	}

	return ConcatSegments(batchPaths, output, workDir, totalDurationSec, func(ratio float64, message string) {
		if onProgress == nil {
			return
		}
		overall := 0.85 + ratio*0.15
		if overall > 1 {
			overall = 1
		}
		msg := message
		if msg == "" {
			msg = fmt.Sprintf("Merging export batches %d%%", int(ratio*100))
		}
		onProgress(overall, msg)
	})
}
