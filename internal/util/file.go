// Package util provides utility functions
package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// PathFromHome preserves absolute paths and resolves relative paths under the
// user's home directory. It supports ~ and ~/path, but not ~user expansion.
func PathFromHome(pathFromHome string) (string, error) {
	if filepath.IsAbs(pathFromHome) {
		return filepath.Clean(pathFromHome), nil
	}
	if filepath.VolumeName(pathFromHome) != "" {
		return "", errors.New("drive-relative paths are not supported")
	}
	if strings.HasPrefix(pathFromHome, "~") {
		if len(pathFromHome) > 1 && !os.IsPathSeparator(pathFromHome[1]) {
			return "", errors.New("only ~ and ~/path home expansion is supported")
		}
		pathFromHome = strings.TrimPrefix(pathFromHome, "~")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	pathFromHome = strings.TrimLeftFunc(pathFromHome, func(r rune) bool {
		return r < 128 && os.IsPathSeparator(uint8(r))
	})

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
