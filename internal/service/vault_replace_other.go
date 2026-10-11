//go:build !linux && !darwin && !windows

package service

import "errors"

func replaceVaultFile(string, string) (bool, error) {
	return false, errors.New("atomic database replacement is unsupported on this platform")
}

func syncVaultDirectory(string) error {
	return errors.New("database directory sync is unsupported on this platform")
}
