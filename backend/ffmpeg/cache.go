package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func newCacheFolderName() string {
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(id) > 12 {
		id = id[:12]
	}
	return "cache-" + id
}

func cacheWorkDir(outputDir, cacheFolder string) string {
	return filepath.Join(outputDir, cacheFolder)
}

func segmentPartPath(workDir string, index int) string {
	return filepath.Join(workDir, fmt.Sprintf("part%02d.mp4", index))
}

func segmentFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func segmentFileReady(path string) bool {
	if !segmentFileExists(path) {
		return false
	}
	meta, err := Probe(path)
	if err != nil || meta.Duration <= 0 {
		return false
	}
	return true
}

type cacheCheckReporter func(checked, total int)

func cachedPartsReady(workDir string, totalSegments int, report cacheCheckReporter) int {
	ready := 0
	for i := 1; i <= totalSegments; i++ {
		if report != nil {
			report(i, totalSegments)
		}
		if !segmentFileExists(segmentPartPath(workDir, i)) {
			return ready
		}
		ready++
	}
	return ready
}

func resolveCacheFolder(outputDir string, requested string, forceRecut bool) (workDir, cacheFolder string) {
	if !forceRecut && IsValidCacheFolderName(requested) {
		return cacheWorkDir(outputDir, requested), requested
	}
	cacheFolder = newCacheFolderName()
	return cacheWorkDir(outputDir, cacheFolder), cacheFolder
}
