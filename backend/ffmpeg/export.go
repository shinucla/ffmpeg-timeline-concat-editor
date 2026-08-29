package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

// ExportSegment is one timeline row (after repeat expansion) read directly from source files.
type ExportSegment struct {
	InputPath string
	Start     float64
	End       float64
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
				Start:    seg.Start,
				End:      seg.End,
				Filename: filename,
				Repeat:   seg.Repeat,
			})
			for i := 0; i < segmentRepeatCount(seg); i++ {
				exportSegs = append(exportSegs, ExportSegment{
					InputPath: inputPath,
					Start:     seg.Start,
					End:       seg.End,
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
				InputPath: inputPath,
				Start:     seg.Start,
				End:       seg.End,
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

func uniqueInputPaths(segments []ExportSegment) []string {
	seen := make(map[string]struct{}, len(segments))
	paths := make([]string, 0, len(segments))
	for _, seg := range segments {
		if _, ok := seen[seg.InputPath]; ok {
			continue
		}
		seen[seg.InputPath] = struct{}{}
		paths = append(paths, seg.InputPath)
	}
	return paths
}

func exportSegmentsSameDimensions(segments []ExportSegment) (bool, error) {
	paths := uniqueInputPaths(segments)
	if len(paths) == 0 {
		return false, nil
	}
	return segmentsSameDimensions(paths)
}

func exportCanvasSize(segments []ExportSegment) (int, int, error) {
	paths := uniqueInputPaths(segments)
	return concatCanvasSize(paths)
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
		if normalize.Apply {
			args = append(args,
				"-vf", buildSingleScaleVideoFilter(normalize.CanvasW, normalize.CanvasH),
				"-af", "aresample=48000,asetpts=PTS-STARTPTS",
			)
		} else {
			args = append(args,
				"-vf", "setpts=PTS-STARTPTS",
				"-af", "asetpts=PTS-STARTPTS",
			)
		}
	} else {
		var filter string
		if normalize.Apply {
			filter = buildScaleFilter(n, normalize.CanvasW, normalize.CanvasH)
		} else {
			filter = buildPTSResetFilter(n)
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

func exportMetadataBatches(
	segments []ExportSegment,
	workDir string,
	normalize exportNormalize,
	onProgress ConcatProgressFn,
) ([]string, error) {
	batchCount := (len(segments) + concatBatchSize - 1) / concatBatchSize
	var batchPaths []string
	var temps []string

	for i := 0; i < len(segments); i += concatBatchSize {
		end := i + concatBatchSize
		if end > len(segments) {
			end = len(segments)
		}
		batch := segments[i:end]
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

	if len(segments) <= concatBatchSize {
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
