package ffmpeg

import "fmt"

// appendLibx264EncodeArgs adds H.264/AAC encode flags.
// When maxVideoBitrateBPS > 0, CRF quality is kept but peak bitrate is capped
// to the highest source video bitrate so outputs do not inflate past originals.
func appendLibx264EncodeArgs(args []string, maxVideoBitrateBPS int64) []string {
	args = append(args,
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "fast",
	)
	if maxVideoBitrateBPS > 0 {
		rate := formatFFmpegBitrate(maxVideoBitrateBPS)
		buf := formatFFmpegBitrate(maxVideoBitrateBPS * 2)
		args = append(args, "-maxrate", rate, "-bufsize", buf)
	}
	return append(args,
		"-c:a", "aac",
		"-b:a", "192k",
	)
}

func formatFFmpegBitrate(bps int64) string {
	if bps < 1000 {
		return "1k"
	}
	return fmt.Sprintf("%dk", (bps+500)/1000)
}

func maxVideoBitrateFromPaths(paths []string) int64 {
	var max int64
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		meta, err := Probe(path)
		if err != nil {
			continue
		}
		if bps := meta.VideoBitRateBPS(); bps > max {
			max = bps
		}
	}
	return max
}

func maxVideoBitrateFromSegments(segments []ExportSegment) int64 {
	paths := make([]string, 0, len(segments))
	for _, seg := range segments {
		paths = append(paths, seg.InputPath)
	}
	return maxVideoBitrateFromPaths(paths)
}
