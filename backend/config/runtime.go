package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type settingsFile struct {
	VideoRoot string `json:"videoRoot"`
}

type Runtime struct {
	mu           sync.RWMutex
	VideoRoot    string
	OutputDir    string
	CacheDir     string
	Port         string
	settingsPath string
}

func LoadRuntime() *Runtime {
	root := projectRoot()
	settingsPath := filepath.Join(root, ".local", "settings.json")

	videoRoot := envOr("VIDEO_ROOT", "")
	if videoRoot == "" {
		if saved, err := readSavedVideoRoot(settingsPath); err == nil && saved != "" {
			videoRoot = saved
		}
	}
	if videoRoot == "" {
		videoRoot = filepath.Join(root, "sample-videos")
	}

	outputDir := envOr("OUTPUT_DIR", filepath.Join(root, "output"))
	cacheDir := envOr("CACHE_DIR", filepath.Join(root, ".cache"))
	port := envOr("PORT", "8000")

	_ = os.MkdirAll(videoRoot, 0o755)
	_ = os.MkdirAll(outputDir, 0o755)
	_ = os.MkdirAll(cacheDir, 0o755)

	return &Runtime{
		VideoRoot:    videoRoot,
		OutputDir:    outputDir,
		CacheDir:     cacheDir,
		Port:         port,
		settingsPath: settingsPath,
	}
}

func readSavedVideoRoot(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var s settingsFile
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}
	return s.VideoRoot, nil
}

func (r *Runtime) Get() (videoRoot, outputDir, cacheDir string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.VideoRoot, r.OutputDir, r.CacheDir
}

func (r *Runtime) SetVideoRoot(path string) error {
	abs, err := ValidateVideoRoot(path)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.VideoRoot = abs
	r.mu.Unlock()

	return r.saveVideoRoot(abs)
}

func ValidateVideoRoot(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	return abs, nil
}

func (r *Runtime) saveVideoRoot(path string) error {
	dir := filepath.Dir(r.settingsPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settingsFile{VideoRoot: path}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.settingsPath, data, 0o644)
}
