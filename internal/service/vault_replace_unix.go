//go:build linux || darwin

package service

import (
	"errors"
	"os"
)

// replaceVaultFile uses one same-filesystem rename, with no deletion or copy
// fallback. Atomic namespace replacement assumes a local filesystem honoring
// rename semantics; network/provider filesystems require separate validation.
func replaceVaultFile(from, to string) (bool, error) {
	err := os.Rename(from, to)
	return err == nil, err
}

// syncVaultDirectory makes the rename durable on filesystems supporting
// directory fsync. Unsupported sync operations are reported, never ignored.
func syncVaultDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return persistenceError("open directory", err)
	}
	err = directory.Sync()
	return errors.Join(persistenceError("flush directory", err), persistenceError("close directory", directory.Close()))
}
