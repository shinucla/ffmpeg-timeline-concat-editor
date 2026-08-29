package ffmpeg

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func EncodeVideoID(relPath string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(relPath))
}

func DecodeVideoID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", fmt.Errorf("invalid video id")
	}
	return string(b), nil
}

func ResolveVideoPath(root, relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path outside root")
	}
	abs := filepath.Join(root, clean)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) && abs != rootAbs {
		return "", fmt.Errorf("path outside root")
	}
	return abs, nil
}
