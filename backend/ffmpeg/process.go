package ffmpeg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

type ProgressFn func(progress float64, message string)

type ProcessResult struct {
	Output      string
	Manifest    string
	CacheFolder string
}

type segmentManifestEntry struct {
	Start                  float64
	End                    float64
	Filename               string
	Repeat                 int
	RotationSteps          int
	AlternateRepeatReverse bool
}

func writeSegmentManifest(path string, entries []segmentManifestEntry) error {
	fileEntries := make([]TimelineFileEntry, len(entries))
	for i, e := range entries {
		fileEntries[i] = TimelineFileEntry{
			Start:                  e.Start,
			End:                    e.End,
			Filename:               e.Filename,
			Repeat:                 e.Repeat,
			RotationSteps:          e.RotationSteps,
			AlternateRepeatReverse: e.AlternateRepeatReverse,
		}
	}
	return WriteTimelineFile(path, "", fileEntries)
}

func runFFmpeg(args ...string) error {
	return runFFmpegWithTimeProgress(args, 0, nil)
}

func ffmpegKilledHint(err error) string {
	if err == nil {
		return ""
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
			return "; FFmpeg was killed (usually out of memory — try fewer repeats, turn off alternate reverse, or increase WSL RAM)"
		}
	}
	if strings.Contains(err.Error(), "signal: killed") {
		return "; FFmpeg was killed (usually out of memory — try fewer repeats, turn off alternate reverse, or increase WSL RAM)"
	}
	return ""
}

func wrapFFmpegError(err error, stderr string) error {
	if err == nil {
		return nil
	}
	hint := ffmpegKilledHint(err)
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%w: %s%s", err, msg, hint)
	}
	if hint != "" {
		return fmt.Errorf("%w%s", err, hint)
	}
	return err
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
			return wrapFFmpegError(err, stderr.String())
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
	return wrapFFmpegError(err, stderr.String())
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
	exportSegs, manifestEntries, err := buildExportSegments(videoRoot, req)
	if err != nil {
		return ProcessResult{}, err
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

	workDir, err := os.MkdirTemp(outputDir, "export-*")
	if err != nil {
		return ProcessResult{}, err
	}
	defer os.RemoveAll(workDir)

	onProgress(0.02, "Saving timeline")
	if err := writeSegmentManifest(manifestPath, manifestEntries); err != nil {
		return ProcessResult{}, fmt.Errorf("write manifest: %w", err)
	}

	totalDuration := exportTotalDuration(exportSegs)
	mainProgressEnd := 0.95
	if req.ExportCutParts {
		mainProgressEnd = 0.85
	}
	onProgress(0.05, fmt.Sprintf("Exporting %d segments from timeline metadata", len(exportSegs)))

	lastExportPct := -1
	reportExportProgress := func(ratio float64, message string) {
		pct := int(ratio * 100)
		if pct > 100 {
			pct = 100
		}
		msg := message
		if msg == "" {
			msg = fmt.Sprintf("Exporting %d%%", pct)
		}
		if pct == lastExportPct && message == "" {
			return
		}
		if message == "" {
			lastExportPct = pct
		}
		span := mainProgressEnd - 0.05
		onProgress(0.05+ratio*span, msg)
	}

	if err := ExportTimeline(exportSegs, finalOutput, workDir, totalDuration, reportExportProgress); err != nil {
		return ProcessResult{}, fmt.Errorf("export: %w", err)
	}

	if req.ExportCutParts {
		cutParts, err := buildCutPartSegments(videoRoot, req)
		if err != nil {
			return ProcessResult{}, fmt.Errorf("cut parts: %w", err)
		}
		reportCutPartsProgress := func(ratio float64, message string) {
			onProgress(0.85+ratio*0.15, message)
		}
		if err := ExportCutParts(cutParts, finalOutput, reportCutPartsProgress); err != nil {
			return ProcessResult{}, fmt.Errorf("export cut parts: %w", err)
		}
	}

	onProgress(1.0, "Done")
	return ProcessResult{
		Output:   finalOutput,
		Manifest: manifestPath,
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
