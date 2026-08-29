package config

import (
	"errors"
	"os/exec"
	"strings"
)

// BrowseFolder opens a native directory picker when zenity or kdialog is available.
func BrowseFolder() (string, error) {
	if path, err := runPicker("zenity", "--file-selection", "--directory", "--title=Select video folder"); err == nil {
		return ValidateVideoRoot(path)
	}
	if path, err := runPicker("kdialog", "--getexistingdirectory"); err == nil {
		return ValidateVideoRoot(path)
	}
	return "", errors.New("no native folder picker found (install zenity or kdialog)")
}

func runPicker(cmd string, args ...string) (string, error) {
	if _, err := exec.LookPath(cmd); err != nil {
		return "", err
	}
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errors.New("no folder selected")
	}
	return path, nil
}
