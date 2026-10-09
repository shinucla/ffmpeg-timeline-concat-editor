package ffmpeg

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

const TimelineCacheHeaderPrefix = "# cache:"
const TimelineWatermarkHeaderPrefix = "# wm"

type TimelineFileEntry struct {
	Start                  float64
	End                    float64
	Filename               string
	Repeat                 int
	RotationSteps          int
	AlternateRepeatReverse bool
	Watermark              *models.Watermark
}

type ParsedTimeline struct {
	CacheFolder string
	Entries     []TimelineFileEntry
}

var cacheFolderNameRE = regexp.MustCompile(`^cache-[a-zA-Z0-9_-]+$`)

func IsValidCacheFolderName(name string) bool {
	return name != "" && cacheFolderNameRE.MatchString(name)
}

func ParseTimelineText(text string) (ParsedTimeline, error) {
	var result ParsedTimeline
	lines := strings.Split(text, "\n")

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if cacheFolder, ok := parseTimelineCacheHeader(line); ok {
				result.CacheFolder = cacheFolder
				continue
			}
			if len(result.Entries) > 0 {
				if wm, ok := parseTimelineWatermarkHeader(line); ok {
					result.Entries[len(result.Entries)-1].Watermark = wm
				}
			}
			continue
		}

		entry, err := parseTimelineSegmentLine(line, i+1)
		if err != nil {
			return ParsedTimeline{}, err
		}
		result.Entries = append(result.Entries, entry)
	}

	if len(result.Entries) == 0 {
		return ParsedTimeline{}, fmt.Errorf("timeline file has no segments")
	}

	return result, nil
}

func parseTimelineCacheHeader(line string) (string, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if !strings.HasPrefix(strings.ToLower(rest), "cache:") {
		return "", false
	}
	name := strings.TrimSpace(rest[len("cache:"):])
	if !IsValidCacheFolderName(name) {
		return "", false
	}
	return name, true
}

// parseTimelineWatermarkHeader parses a "# wm x y w h start duration text..."
// comment line. Position/size are fractions, times are seconds relative to the
// owning segment.
func parseTimelineWatermarkHeader(line string) (*models.Watermark, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	fields := strings.Fields(rest)
	if len(fields) < 7 || !strings.EqualFold(fields[0], "wm") {
		return nil, false
	}
	values := make([]float64, 6)
	for i := 0; i < 6; i++ {
		v, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, false
		}
		values[i] = v
	}
	return &models.Watermark{
		Enabled:  true,
		Text:     strings.Join(fields[7:], " "),
		X:        values[0],
		Y:        values[1],
		Width:    values[2],
		Height:   values[3],
		Start:    values[4],
		Duration: values[5],
	}, true
}

func parseTimelineSegmentLine(line string, lineNumber int) (TimelineFileEntry, error) {
	oldRE := regexp.MustCompile(`^(\d{2}:\d{2}:\d{2})\s*-\s*(\d{2}:\d{2}:\d{2})\s+(.+)$`)
	if oldMatch := oldRE.FindStringSubmatch(line); oldMatch != nil {
		start, err := parseTimelineClock(oldMatch[1])
		if err != nil {
			return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		end, err := parseTimelineClock(oldMatch[2])
		if err != nil {
			return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if end <= start {
			return TimelineFileEntry{}, fmt.Errorf("line %d: end must be after start", lineNumber)
		}
		return TimelineFileEntry{
			Start:                  start,
			End:                    end,
			Filename:               strings.TrimSpace(oldMatch[3]),
			Repeat:                 1,
			RotationSteps:          0,
			AlternateRepeatReverse: false,
		}, nil
	}

	parts := strings.Fields(line)
	if len(parts) < 4 {
		return TimelineFileEntry{}, fmt.Errorf("invalid timeline line %d", lineNumber)
	}

	start, err := parseTimelineClock(parts[0])
	if err != nil {
		return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
	}
	end, err := parseTimelineClock(parts[1])
	if err != nil {
		return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
	}
	if end <= start {
		return TimelineFileEntry{}, fmt.Errorf("line %d: end must be after start", lineNumber)
	}

	repeat, rotationSteps, alternateRepeatReverse, filename, err := parseTimelineTrailingFields(parts)
	if err != nil {
		return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
	}

	return TimelineFileEntry{
		Start:                  start,
		End:                    end,
		Filename:               filename,
		Repeat:                 repeat,
		RotationSteps:          rotationSteps,
		AlternateRepeatReverse: alternateRepeatReverse,
	}, nil
}

func parseTimelineTrailingFields(parts []string) (repeat, rotationSteps int, alternateRepeatReverse bool, filename string, err error) {
	if len(parts) < 4 {
		return 0, 0, false, "", fmt.Errorf("invalid timeline line")
	}

	if len(parts) >= 6 {
		alternateToken := parts[len(parts)-1]
		rotationToken := parts[len(parts)-2]
		repeatToken := parts[len(parts)-3]
		parsedAlternate, altErr := parseTimelineAlternateRepeatReverse(alternateToken)
		parsedRotation, rotErr := parseTimelineRotationSteps(rotationToken)
		parsedRepeat, repErr := parseTimelineRepeat(repeatToken)
		if altErr == nil && rotErr == nil && repErr == nil {
			filename = strings.Join(parts[2:len(parts)-3], " ")
			if filename == "" {
				return 0, 0, false, "", fmt.Errorf("missing filename")
			}
			return parsedRepeat, parsedRotation, parsedAlternate, filename, nil
		}
	}

	if len(parts) >= 5 {
		rotationToken := parts[len(parts)-1]
		repeatToken := parts[len(parts)-2]
		parsedRotation, rotErr := parseTimelineRotationSteps(rotationToken)
		parsedRepeat, repErr := parseTimelineRepeat(repeatToken)
		if rotErr == nil && repErr == nil {
			filename = strings.Join(parts[2:len(parts)-2], " ")
			if filename == "" {
				return 0, 0, false, "", fmt.Errorf("missing filename")
			}
			return parsedRepeat, parsedRotation, false, filename, nil
		}
	}

	repeatToken := parts[len(parts)-1]
	parsedRepeat, err := parseTimelineRepeat(repeatToken)
	if err != nil {
		return 0, 0, false, "", err
	}
	filename = strings.Join(parts[2:len(parts)-1], " ")
	if filename == "" {
		return 0, 0, false, "", fmt.Errorf("missing filename")
	}
	return parsedRepeat, 0, false, filename, nil
}

func parseTimelineAlternateRepeatReverse(token string) (bool, error) {
	var value int
	if _, err := fmt.Sscanf(token, "%d", &value); err != nil || (value != 0 && value != 1) {
		return false, fmt.Errorf("alternate reverse must be 0 or 1")
	}
	return value == 1, nil
}

func parseTimelineRotationSteps(token string) (int, error) {
	var rotation int
	if _, err := fmt.Sscanf(token, "%d", &rotation); err != nil || rotation < 0 || rotation > 3 {
		return 0, fmt.Errorf("rotation must be a number from 0 to 3")
	}
	return rotation, nil
}

func parseTimelineClock(value string) (float64, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	var h, m, s int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &s); err != nil {
		return 0, fmt.Errorf("invalid time: %s", value)
	}
	return float64(h*3600 + m*60 + s), nil
}

func parseTimelineRepeat(token string) (int, error) {
	var repeat int
	if _, err := fmt.Sscanf(token, "%d", &repeat); err != nil || repeat < 1 {
		return 0, fmt.Errorf("repeat must be a number >= 1")
	}
	return repeat, nil
}

func normalizeRotationSteps(steps int) int {
	steps %= 4
	if steps < 0 {
		steps += 4
	}
	return steps
}

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

func formatTimelineNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatTimelineWatermark(wm *models.Watermark) string {
	return fmt.Sprintf(
		"%s %s %s %s %s %s %s %s",
		TimelineWatermarkHeaderPrefix,
		formatTimelineNumber(wm.X),
		formatTimelineNumber(wm.Y),
		formatTimelineNumber(wm.Width),
		formatTimelineNumber(wm.Height),
		formatTimelineNumber(wm.Start),
		formatTimelineNumber(wm.Duration),
		wm.Text,
	)
}

func WriteTimelineFile(path, cacheFolder string, entries []TimelineFileEntry) error {
	var lines []string
	if cacheFolder != "" {
		lines = append(lines, fmt.Sprintf("%s %s", TimelineCacheHeaderPrefix, cacheFolder))
	}
	for _, e := range entries {
		repeat := e.Repeat
		if repeat < 1 {
			repeat = 1
		}
		rotation := normalizeRotationSteps(e.RotationSteps)
		alternate := 0
		if e.AlternateRepeatReverse {
			alternate = 1
		}
		lines = append(lines, fmt.Sprintf(
			"%s %s %s %d %d %d",
			formatTimelineTime(e.Start),
			formatTimelineTime(e.End),
			e.Filename,
			repeat,
			rotation,
			alternate,
		))
		if e.Watermark != nil && e.Watermark.Enabled {
			lines = append(lines, formatTimelineWatermark(e.Watermark))
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
