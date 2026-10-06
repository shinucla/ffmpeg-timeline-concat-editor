package ffmpeg

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var clockRE = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)

type YtdlpSegment struct {
	Start string
	End   string
}

type YtdlpDownloadResult struct {
	Outputs []string
}

func parseClockToSeconds(value string) (int, error) {
	if !clockRE.MatchString(value) {
		return 0, fmt.Errorf("time must be HH:MM:SS")
	}
	var h, m, s int
	if _, err := fmt.Sscanf(value, "%d:%d:%d", &h, &m, &s); err != nil {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	if m > 59 || s > 59 {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	return h*3600 + m*60 + s, nil
}

func validateYtdlpSegment(seg YtdlpSegment) error {
	startSec, err := parseClockToSeconds(strings.TrimSpace(seg.Start))
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	endSec, err := parseClockToSeconds(strings.TrimSpace(seg.End))
	if err != nil {
		return fmt.Errorf("end: %w", err)
	}
	if endSec <= startSec {
		return fmt.Errorf("end must be after start")
	}
	return nil
}

func parseYtdlpPrintedPaths(stdout string) []string {
	var paths []string
	seen := map[string]bool{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(line))
		switch ext {
		case ".mp4", ".mkv", ".webm", ".m4a", ".mp3":
			if !seen[line] {
				seen[line] = true
				paths = append(paths, line)
			}
		}
	}
	return paths
}

func summarizeYtdlpError(stdout, stderr string, err error) error {
	combined := strings.TrimSpace(stderr + "\n" + stdout)
	var useful []string
	for _, line := range strings.Split(combined, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "ERROR:") ||
			strings.Contains(upper, "403") ||
			strings.Contains(upper, "FFMPEG") ||
			strings.HasPrefix(upper, "ERROR ") {
			useful = append(useful, line)
		}
	}
	if len(useful) > 0 {
		if len(useful) > 6 {
			useful = useful[len(useful)-6:]
		}
		return fmt.Errorf("yt-dlp failed: %s", strings.Join(useful, " | "))
	}
	if combined != "" {
		if len(combined) > 800 {
			combined = combined[len(combined)-800:]
		}
		return fmt.Errorf("yt-dlp failed: %w: %s", err, combined)
	}
	return fmt.Errorf("yt-dlp failed: %w", err)
}

func ytdlpOutputTemplate(index, total int) string {
	if total <= 1 {
		return "[%(uploader)s] %(title).70s.%(ext)s"
	}
	return fmt.Sprintf("[%%(uploader)s] %%(title).70s-%02d.%%(ext)s", index)
}

func runYtdlpSection(
	videoRoot, url string,
	seg YtdlpSegment,
	outTemplate string,
	forceKeyframes bool,
) (string, error) {
	args := []string{
		"--no-update",
		"--no-playlist",
		// Prefer MP4 DASH (not progressive itag 18). Progressive + ffmpeg downloader
		// often gets YouTube 403 → "ffmpeg exited with code 8".
		"-f", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b",
		"--merge-output-format", "mp4",
		"-P", videoRoot,
		"-o", outTemplate,
		"--download-sections", fmt.Sprintf("*%s-%s", strings.TrimSpace(seg.Start), strings.TrimSpace(seg.End)),
		"--print", "after_move:filepath",
		url,
	}
	if forceKeyframes {
		args = append([]string{
			"--no-update",
			"--no-playlist",
			"--force-keyframes-at-cuts",
		}, args[2:]...)
	}

	cmd := exec.Command("yt-dlp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", summarizeYtdlpError(stdout.String(), stderr.String(), err)
	}
	paths := parseYtdlpPrintedPaths(stdout.String())
	if len(paths) == 0 {
		return "", summarizeYtdlpError(stdout.String(), stderr.String(), fmt.Errorf("no media files produced"))
	}
	return paths[0], nil
}

// DownloadYtdlpSections downloads URL ranges with yt-dlp only (--download-sections).
// No app-side trim/concat: each section is a separate yt-dlp output file.
func DownloadYtdlpSections(
	videoRoot, url string,
	segments []YtdlpSegment,
	onProgress ProgressFn,
) (YtdlpDownloadResult, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return YtdlpDownloadResult{}, fmt.Errorf("url is required")
	}
	if len(segments) == 0 {
		return YtdlpDownloadResult{}, fmt.Errorf("at least one segment is required")
	}
	for i, seg := range segments {
		if err := validateYtdlpSegment(seg); err != nil {
			return YtdlpDownloadResult{}, fmt.Errorf("segment %d: %w", i+1, err)
		}
	}

	var outputs []string
	total := len(segments)
	for i, seg := range segments {
		if onProgress != nil {
			onProgress(
				float64(i)/float64(total),
				fmt.Sprintf("yt-dlp downloading section %d/%d (%s-%s)", i+1, total, seg.Start, seg.End),
			)
		}

		outTemplate := ytdlpOutputTemplate(i+1, total)
		path, err := runYtdlpSection(videoRoot, url, seg, outTemplate, true)
		if err != nil {
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "ffmpeg exited") || strings.Contains(errText, "403") {
				// Fallback: still yt-dlp --download-sections, without re-encode cuts.
				path, err = runYtdlpSection(videoRoot, url, seg, outTemplate, false)
			}
		}
		if err != nil {
			return YtdlpDownloadResult{}, fmt.Errorf("section %d: %w", i+1, err)
		}
		outputs = append(outputs, path)
	}

	if onProgress != nil {
		onProgress(1, fmt.Sprintf("Downloaded %d file(s)", len(outputs)))
	}
	return YtdlpDownloadResult{Outputs: outputs}, nil
}
