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
	BitRate  string `json:"bit_rate"`
}

type probeStream struct {
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
	BitRate    string `json:"bit_rate"`
}

type probeResult struct {
	Format  probeFormat   `json:"format"`
	Streams []probeStream `json:"streams"`
}

type Metadata struct {
	Duration      float64
	Size          int64
	Width         int
	Height        int
	FPS           float64
	BitRate       int64 // container/format average bitrate (bits/s)
	VideoBitRate  int64 // video stream bitrate when reported (bits/s)
	AudioBitRate  int64 // audio stream bitrate when reported (bits/s)
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
	meta.BitRate = parseBitRate(result.Format.BitRate)
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			if meta.Width == 0 {
				meta.Width = s.Width
				meta.Height = s.Height
				meta.FPS = parseFPS(s.RFrameRate)
			}
			if meta.VideoBitRate == 0 {
				meta.VideoBitRate = parseBitRate(s.BitRate)
			}
		case "audio":
			if meta.AudioBitRate == 0 {
				meta.AudioBitRate = parseBitRate(s.BitRate)
			}
		}
	}
	return meta, nil
}

// VideoBitRateBPS returns the best available video bitrate estimate in bits/s.
func (m Metadata) VideoBitRateBPS() int64 {
	if m.VideoBitRate > 0 {
		return m.VideoBitRate
	}
	if m.BitRate > 0 {
		v := m.BitRate - m.AudioBitRate
		if m.AudioBitRate == 0 && m.BitRate > 192_000 {
			v = m.BitRate - 192_000
		}
		if v > 0 {
			return v
		}
		return m.BitRate
	}
	if m.Duration > 0 && m.Size > 0 {
		total := int64(float64(m.Size) * 8 / m.Duration)
		if total > 192_000 {
			return total - 192_000
		}
		return total
	}
	return 0
}

func parseBitRate(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "N/A" {
		return 0
	}
	bps, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || bps <= 0 {
		return 0
	}
	return bps
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
