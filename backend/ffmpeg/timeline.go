package ffmpeg

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const TimelineCacheHeaderPrefix = "# cache:"

type TimelineFileEntry struct {
	Start    float64
	End      float64
	Filename string
	Repeat   int
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
			Start:    start,
			End:      end,
			Filename: strings.TrimSpace(oldMatch[3]),
			Repeat:   1,
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

	repeatToken := parts[len(parts)-1]
	repeat, err := parseTimelineRepeat(repeatToken)
	if err != nil {
		return TimelineFileEntry{}, fmt.Errorf("line %d: %w", lineNumber, err)
	}

	filename := strings.Join(parts[2:len(parts)-1], " ")
	if filename == "" {
		return TimelineFileEntry{}, fmt.Errorf("line %d: missing filename", lineNumber)
	}

	return TimelineFileEntry{
		Start:    start,
		End:      end,
		Filename: filename,
		Repeat:   repeat,
	}, nil
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
		lines = append(lines, fmt.Sprintf(
			"%s %s %s %d",
			formatTimelineTime(e.Start),
			formatTimelineTime(e.End),
			e.Filename,
			repeat,
		))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
