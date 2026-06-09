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
