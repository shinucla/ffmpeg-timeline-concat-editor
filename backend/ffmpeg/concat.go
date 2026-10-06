package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const concatBatchSize = 20

type ConcatProgressFn func(ratio float64, message string)

func segmentsSameDimensions(paths []string) (bool, error) {
	if len(paths) == 0 {
		return false, nil
	}
	first, err := Probe(paths[0])
	if err != nil {
		return false, err
	}
	for _, path := range paths[1:] {
		meta, err := Probe(path)
		if err != nil {
			return false, err
		}
		if meta.Width != first.Width || meta.Height != first.Height {
			return false, nil
		}
	}
	return true, nil
}

func batchDurationSec(paths []string) float64 {
	total := 0.0
	for _, path := range paths {
		meta, err := Probe(path)
		if err != nil {
			continue
		}
		total += meta.Duration
	}
	if total <= 0 {
		return 1
	}
	return total
}

func estimateTotalMerges(count, batchSize int) int {
	if count <= 1 {
		return 0
	}
	merges := 0
	level := count
	for level > 1 {
		next := 0
		for i := 0; i < level; i += batchSize {
			end := i + batchSize
			if end > level {
				end = level
			}
			if end-i > 1 {
				merges++
			}
			next++
		}
		level = next
	}
	return merges
}

func buildPTSResetFilter(n int) string {
	var filters strings.Builder
	for i := 0; i < n; i++ {
		filters.WriteString(fmt.Sprintf("[%d:v]setpts=PTS-STARTPTS[v%d];", i, i))
		filters.WriteString(fmt.Sprintf("[%d:a]asetpts=PTS-STARTPTS[a%d];", i, i))
	}
	var concatInputs strings.Builder
	for i := 0; i < n; i++ {
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	filters.WriteString(fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concatInputs.String(), n))
	return filters.String()
}

func buildScaleFilter(n, canvasW, canvasH int) string {
	var filters strings.Builder
	for i := 0; i < n; i++ {
		filters.WriteString(fmt.Sprintf(
			"[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1,setpts=PTS-STARTPTS[v%d];",
			i, canvasW, canvasH, canvasW, canvasH, i,
		))
		filters.WriteString(fmt.Sprintf(
			"[%d:a]aresample=48000,asetpts=PTS-STARTPTS[a%d];",
			i, i,
		))
	}
	var concatInputs strings.Builder
	for i := 0; i < n; i++ {
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	filters.WriteString(fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concatInputs.String(), n))
	return filters.String()
}

func concatBatchWithFilter(paths []string, output, filter string, durationSec float64, onBatchRatio func(float64)) error {
	n := len(paths)
	if n < 2 {
		return fmt.Errorf("need at least 2 segments to filter-concat")
	}

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-nostats"}
	for _, path := range paths {
		args = append(args, "-i", path)
	}
	args = append(args,
		"-filter_complex", filter,
		"-map", "[outv]",
		"-map", "[outa]",
	)
	args = appendLibx264EncodeArgs(args, maxVideoBitrateFromPaths(paths))
	args = append(args, output)

	if onBatchRatio == nil {
		return runFFmpeg(args...)
	}
	return runFFmpegWithTimeProgress(args, durationSec, onBatchRatio)
}

func mergeBatchWithProgress(paths []string, output string, durationSec float64, onBatchRatio func(float64)) error {
	sameSize, err := segmentsSameDimensions(paths)
	if err != nil {
		return err
	}
	if sameSize {
		return concatBatchWithFilter(paths, output, buildPTSResetFilter(len(paths)), durationSec, onBatchRatio)
	}
	canvasW, canvasH, err := concatCanvasSize(paths)
	if err != nil {
		return err
	}
	return concatBatchWithFilter(paths, output, buildScaleFilter(len(paths), canvasW, canvasH), durationSec, onBatchRatio)
}

func concatTreeBatched(
	paths []string,
	output string,
	workDir string,
	onProgress ConcatProgressFn,
) error {
	level := append([]string(nil), paths...)
	var temps []string
	defer func() {
		for _, temp := range temps {
			_ = os.Remove(temp)
		}
	}()

	totalMerges := estimateTotalMerges(len(paths), concatBatchSize)
	mergeIndex := 0
	round := 0

	for len(level) > 1 {
		batchInRound := 0
		mergesInRound := 0
		for i := 0; i < len(level); i += concatBatchSize {
			end := i + concatBatchSize
			if end > len(level) {
				end = len(level)
			}
			if end-i > 1 {
				mergesInRound++
			}
		}

		var next []string
		for i := 0; i < len(level); i += concatBatchSize {
			end := i + concatBatchSize
			if end > len(level) {
				end = len(level)
			}
			batch := level[i:end]
			if len(batch) == 1 {
				next = append(next, batch[0])
				continue
			}

			batchInRound++
			mergeIndex++
			baseRatio := float64(mergeIndex-1) / float64(totalMerges)

			isFinalRound := len(level) <= concatBatchSize
			out := filepath.Join(workDir, fmt.Sprintf("merge-r%d-b%d.mp4", round, len(next)))
			if isFinalRound && len(next) == 0 && i+concatBatchSize >= len(level) {
				out = output
			} else {
				temps = append(temps, out)
			}

			if onProgress != nil {
				onProgress(
					baseRatio,
					fmt.Sprintf(
						"Concat round %d: merging batch %d/%d (%d parts)",
						round+1,
						batchInRound,
						mergesInRound,
						len(batch),
					),
				)
			}

			batchDuration := batchDurationSec(batch)
			err := mergeBatchWithProgress(batch, out, batchDuration, func(batchRatio float64) {
				if onProgress == nil || totalMerges == 0 {
					return
				}
				overall := baseRatio + batchRatio/float64(totalMerges)
				if overall > 1 {
					overall = 1
				}
				onProgress(overall, fmt.Sprintf("Concat round %d batch %d/%d: %d%%", round+1, batchInRound, mergesInRound, int(batchRatio*100)))
			})
			if err != nil {
				return fmt.Errorf("merge batch round %d: %w", round+1, err)
			}
			next = append(next, out)
		}

		for _, path := range level {
			stillUsed := false
			for _, kept := range next {
				if path == kept {
					stillUsed = true
					break
				}
			}
			if !stillUsed && path != output {
				for _, temp := range temps {
					if path == temp {
						_ = os.Remove(path)
						break
					}
				}
			}
		}

		level = next
		round++
	}

	if len(level) == 1 && level[0] != output {
		return copyFile(level[0], output)
	}
	if onProgress != nil {
		onProgress(1, "Concatenating segments 100%")
	}
	return nil
}

func ConcatSegments(
	segmentPaths []string,
	output string,
	workDir string,
	totalDurationSec float64,
	onProgress ConcatProgressFn,
) error {
	if len(segmentPaths) == 0 {
		return fmt.Errorf("no segments to concat")
	}
	if len(segmentPaths) == 1 {
		if onProgress != nil {
			onProgress(0, "Copying single segment")
			onProgress(1, "Concatenating segments 100%")
		}
		if err := copyFile(segmentPaths[0], output); err != nil {
			return err
		}
		return nil
	}

	if len(segmentPaths) <= concatBatchSize {
		if onProgress != nil {
			onProgress(0, fmt.Sprintf("Concatenating %d parts (PTS reset)", len(segmentPaths)))
		}
		duration := batchDurationSec(segmentPaths)
		if duration <= 0 {
			duration = totalDurationSec
		}
		err := mergeBatchWithProgress(segmentPaths, output, duration, func(batchRatio float64) {
			if onProgress != nil {
				onProgress(batchRatio, fmt.Sprintf("Concatenating segments %d%%", int(batchRatio*100)))
			}
		})
		if err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(1, "Concatenating segments 100%")
		}
		return nil
	}

	return concatTreeBatched(segmentPaths, output, workDir, onProgress)
}
