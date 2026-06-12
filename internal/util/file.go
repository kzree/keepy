// Package util provides utility functions
package util

import (
	"os"
	"path/filepath"
	"strings"
)

func PathFromHome(pathFromHome string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	pathFromHome = strings.TrimPrefix(pathFromHome, "~")
	pathFromHome = strings.TrimPrefix(pathFromHome, string(filepath.Separator))

	return filepath.Join(home, pathFromHome), nil
}

func OpenOrCreateFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
}

func OpenOrCreateFileAndTruncate(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}
