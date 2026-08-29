package ffmpeg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

type ProgressFn func(progress float64, message string)

func formatTimelineTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type ProcessResult struct {
	Output      string
	Manifest    string
	CacheFolder string
}

type segmentManifestEntry struct {
	Start    float64
	End      float64
	Filename string
	Repeat   int
}

func writeSegmentManifest(path, cacheFolder string, entries []segmentManifestEntry) error {
	fileEntries := make([]TimelineFileEntry, len(entries))
	for i, e := range entries {
		fileEntries[i] = TimelineFileEntry{
			Start:    e.Start,
			End:      e.End,
			Filename: e.Filename,
			Repeat:   e.Repeat,
		}
	}
	return WriteTimelineFile(path, cacheFolder, fileEntries)
}

func formatFFmpegTime(seconds float64) string {
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

func runFFmpeg(args ...string) error {
	return runFFmpegWithTimeProgress(args, 0, nil)
}

func parseFFmpegOutTimeSeconds(line string) (float64, bool) {
	kinds := []struct {
		prefix string
		div    float64
	}{
		{"out_time_us=", 1_000_000},
		{"out_time_ms=", 1_000},
		{"out_time=", 1},
	}
	for _, kind := range kinds {
		if !strings.HasPrefix(line, kind.prefix) {
			continue
		}
		value := strings.TrimPrefix(line, kind.prefix)
		if kind.prefix == "out_time=" {
			parts := strings.SplitN(value, ":", 3)
			if len(parts) != 3 {
				return 0, false
			}
			h, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			s, err3 := strconv.ParseFloat(parts[2], 64)
			if err1 != nil || err2 != nil || err3 != nil {
				return 0, false
			}
			return float64(h*3600+m*60) + s, true
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		return float64(n) / kind.div, true
	}
	return 0, false
}

func runFFmpegWithTimeProgress(args []string, totalDurationSec float64, onRatio func(ratio float64)) error {
	quiet := []string{"-hide_banner", "-loglevel", "error", "-nostats"}
	if onRatio != nil {
		if totalDurationSec <= 0 {
			totalDurationSec = 1
		}
		quiet = append(quiet, "-progress", "pipe:1")
	}
	cmd := exec.Command("ffmpeg", append(quiet, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if onRatio == nil {
		if err := cmd.Run(); err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}
		return nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "progress=end" {
				onRatio(1)
				continue
			}
			outSeconds, ok := parseFFmpegOutTimeSeconds(line)
			if !ok || outSeconds <= 0 {
				continue
			}
			ratio := outSeconds / totalDurationSec
			if ratio > 1 {
				ratio = 1
			}
			onRatio(ratio)
		}
	}()

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func CutSegment(input, output string, start, end float64) error {
	if end <= start {
		return fmt.Errorf("invalid segment duration")
	}
	return runFFmpeg(
		"-y",
		"-ss", formatFFmpegTime(start),
		"-to", formatFFmpegTime(end),
		"-i", input,
		"-vf", "setpts=PTS-STARTPTS",
		"-af", "asetpts=PTS-STARTPTS",
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "fast",
		"-c:a", "aac",
		"-b:a", "192k",
		"-fps_mode", "passthrough",
		output,
	)
}

func evenDimension(n int) int {
	if n <= 0 {
		return 2
	}
	if n%2 != 0 {
		return n + 1
	}
	return n
}

func concatCanvasSize(segmentPaths []string) (int, int, error) {
	maxW, maxH := 0, 0
	for _, path := range segmentPaths {
		meta, err := Probe(path)
		if err != nil {
			return 0, 0, fmt.Errorf("probe segment %s: %w", filepath.Base(path), err)
		}
		if meta.Width <= 0 || meta.Height <= 0 {
			return 0, 0, fmt.Errorf("segment %s has no video dimensions", filepath.Base(path))
		}
		if meta.Width > maxW {
			maxW = meta.Width
		}
		if meta.Height > maxH {
			maxH = meta.Height
		}
	}
	return evenDimension(maxW), evenDimension(maxH), nil
}

func segmentRepeat(seg models.Segment) int {
	if seg.Repeat < 1 {
		return 1
	}
	return seg.Repeat
}

func expandConcatPaths(segmentPaths []string, req models.ProcessRequest) []string {
	if len(segmentPaths) == 0 {
		return nil
	}
	expanded := make([]string, 0, len(segmentPaths))
	pathIndex := 0
	for _, clip := range req.Clips {
		for _, seg := range clip.Segments {
			if pathIndex >= len(segmentPaths) {
				return expanded
			}
			path := segmentPaths[pathIndex]
			pathIndex++
			for i := 0; i < segmentRepeat(seg); i++ {
				expanded = append(expanded, path)
			}
		}
	}
	return expanded
}

func concatTotalDuration(req models.ProcessRequest) float64 {
	total := 0.0
	for _, clip := range req.Clips {
		for _, seg := range clip.Segments {
			if seg.End <= seg.Start {
				continue
			}
			total += (seg.End - seg.Start) * float64(segmentRepeat(seg))
		}
	}
	return total
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func ProcessProject(
	videoRoot, outputDir string,
	req models.ProcessRequest,
	onProgress ProgressFn,
) (ProcessResult, error) {
	if len(req.Clips) == 0 {
		return ProcessResult{}, fmt.Errorf("no clips to process")
	}

	var manifestEntries []segmentManifestEntry
	totalSegments := 0
	for _, clip := range req.Clips {
		rel, err := DecodeVideoID(clip.VideoID)
		if err != nil {
			return ProcessResult{}, fmt.Errorf("invalid video id %s: %w", clip.VideoID, err)
		}
		filename := filepath.Base(rel)
		for _, seg := range clip.Segments {
			totalSegments++
			manifestEntries = append(manifestEntries, segmentManifestEntry{
				Start:    seg.Start,
				End:      seg.End,
				Filename: filename,
				Repeat:   seg.Repeat,
			})
		}
	}
	if totalSegments == 0 {
		return ProcessResult{}, fmt.Errorf("no segments to process")
	}

	name := req.Name
	if name == "" {
		name = "output"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".mp4") {
		name += ".mp4"
	}
	finalOutput := filepath.Join(videoRoot, name)
	manifestPath := strings.TrimSuffix(finalOutput, filepath.Ext(finalOutput)) + ".txt"

	workDir, cacheFolder := resolveCacheFolder(outputDir, req.CacheFolder, req.ForceRecut)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return ProcessResult{}, err
	}

	onProgress(0.02, "Saving timeline")
	if err := writeSegmentManifest(manifestPath, cacheFolder, manifestEntries); err != nil {
		return ProcessResult{}, fmt.Errorf("write manifest: %w", err)
	}

	cachedReady := 0
	if !req.ForceRecut {
		onProgress(0.03, "Checking cached segments")
		cachedReady = cachedPartsReady(workDir, totalSegments, func(checked, total int) {
			onProgress(
				0.03+float64(checked)/float64(total)*0.02,
				fmt.Sprintf("Checking cached segment %d/%d", checked, total),
			)
		})
	}
	if cachedReady == totalSegments {
		onProgress(0.05, fmt.Sprintf("Resuming: reusing cache folder %s (%d segments)", cacheFolder, cachedReady))
	} else if cachedReady > 0 {
		onProgress(0.05, fmt.Sprintf("Resuming: found %d/%d cached segments in %s", cachedReady, totalSegments, cacheFolder))
	} else if req.ForceRecut {
		onProgress(0.05, fmt.Sprintf("Re-cutting all segments into %s", cacheFolder))
	} else if req.CacheFolder != "" {
		onProgress(0.05, fmt.Sprintf("Cache folder %s not ready; cutting segments", cacheFolder))
	} else {
		onProgress(0.05, fmt.Sprintf("Cutting segments into %s", cacheFolder))
	}

	var segmentPaths []string
	if cachedReady == totalSegments {
		segmentPaths = make([]string, 0, totalSegments)
		for i := 1; i <= totalSegments; i++ {
			segmentPaths = append(segmentPaths, segmentPartPath(workDir, i))
		}
		onProgress(0.90, fmt.Sprintf("All %d segments cached, starting concat", totalSegments))
	} else {
		done := cachedReady
		segIndex := 0
		for _, clip := range req.Clips {
			rel, err := DecodeVideoID(clip.VideoID)
			if err != nil {
				return ProcessResult{}, fmt.Errorf("invalid video id %s: %w", clip.VideoID, err)
			}
			inputPath, err := ResolveVideoPath(videoRoot, rel)
			if err != nil {
				return ProcessResult{}, fmt.Errorf("resolve video %s: %w", clip.VideoID, err)
			}

			for _, seg := range clip.Segments {
				segIndex++
				outPath := segmentPartPath(workDir, segIndex)
				segmentPaths = append(segmentPaths, outPath)

				if !req.ForceRecut && segmentFileExists(outPath) {
					onProgress(0.05+float64(done)/float64(totalSegments)*0.85, fmt.Sprintf("Using cached segment %d/%d", segIndex, totalSegments))
					done++
					continue
				}

				onProgress(0.05+float64(done)/float64(totalSegments)*0.85, fmt.Sprintf("Cutting segment %d/%d", segIndex, totalSegments))
				if err := CutSegment(inputPath, outPath, seg.Start, seg.End); err != nil {
					return ProcessResult{}, fmt.Errorf("cut segment %d: %w", segIndex, err)
				}
				done++
				onProgress(0.05+float64(done)/float64(totalSegments)*0.85, fmt.Sprintf("Cut segment %d/%d done", segIndex, totalSegments))
			}
		}
	}

	onProgress(0.91, "Concatenating segments 0%")

	concatPaths := expandConcatPaths(segmentPaths, req)
	totalDuration := concatTotalDuration(req)

	lastConcatPct := -1
	reportConcatProgress := func(ratio float64, message string) {
		pct := int(ratio * 100)
		if pct > 100 {
			pct = 100
		}
		msg := message
		if msg == "" {
			msg = fmt.Sprintf("Concatenating segments %d%%", pct)
		}
		if pct == lastConcatPct && message == "" {
			return
		}
		if message == "" {
			lastConcatPct = pct
		}
		onProgress(0.91+ratio*0.09, msg)
	}

	if err := ConcatSegments(concatPaths, finalOutput, workDir, totalDuration, reportConcatProgress); err != nil {
		return ProcessResult{}, fmt.Errorf("concat: %w", err)
	}
	_ = os.RemoveAll(workDir)
	onProgress(1.0, "Done")
	return ProcessResult{
		Output:      finalOutput,
		Manifest:    manifestPath,
		CacheFolder: cacheFolder,
	}, nil
}

func ExtractFrame(input, output string, timestamp float64) error {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-ss", fmt.Sprintf("%.3f", timestamp),
		"-i", input,
		"-frames:v", "1",
		"-q:v", "2",
		output,
	)
	return cmd.Run()
}
