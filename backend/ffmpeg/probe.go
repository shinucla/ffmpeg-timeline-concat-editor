package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type probeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
}

type probeStream struct {
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
}

type probeResult struct {
	Format  probeFormat   `json:"format"`
	Streams []probeStream `json:"streams"`
}

type Metadata struct {
	Duration float64
	Size     int64
	Width    int
	Height   int
	FPS      float64
}

func Probe(path string) (Metadata, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("ffprobe: %w", err)
	}

	var result probeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return Metadata{}, err
	}

	meta := Metadata{}
	if result.Format.Duration != "" {
		meta.Duration, _ = strconv.ParseFloat(result.Format.Duration, 64)
	}
	if result.Format.Size != "" {
		meta.Size, _ = strconv.ParseInt(result.Format.Size, 10, 64)
	}
	for _, s := range result.Streams {
		if s.CodecType == "video" {
			meta.Width = s.Width
			meta.Height = s.Height
			meta.FPS = parseFPS(s.RFrameRate)
			break
		}
	}
	return meta, nil
}

func parseFPS(rate string) float64 {
	if rate == "" || rate == "0/0" {
		return 30
	}
	parts := strings.Split(rate, "/")
	if len(parts) != 2 {
		f, _ := strconv.ParseFloat(rate, 64)
		if f > 0 {
			return f
		}
		return 30
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 30
	}
	return num / den
}
