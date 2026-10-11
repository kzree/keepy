package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

var errVaultNotRegular = errors.New("database path must refer to a regular file")

// checkBaseline detects changes to content, permission bits, file identity, or
// path type without refreshing the expected baseline. It checks the namespace
// before opening and again after reading to detect replacement during the read.
// File identity comes from the opened handle, not a later read of its path.
//
// These are optimistic checks, not cross-process compare-and-swap. Another
// client can still change the file between the final check and replacement.
// Eliminating that gap requires a locking protocol honored by other clients.
func (p *vaultPersistence) checkBaseline(path string, expected vaultFileBaseline) error {
	if expected.info == nil {
		return &SaveError{Kind: SaveFailed, Op: "check database baseline", Err: errors.New("database baseline is unavailable")}
	}
	entry, err := p.lstat(path)
	if err != nil {
		return baselineReadError(err)
	}
	if !entry.Mode().IsRegular() {
		return baselineConflict(errVaultNotRegular)
	}
	current, err := p.read(path)
	if err != nil {
		return baselineReadError(err)
	}
	if current.baseline.fingerprint != expected.fingerprint ||
		current.baseline.mode != expected.mode || !os.SameFile(current.baseline.info, expected.info) {
		return baselineConflict(errors.New("database changed since authentication or the last committed save"))
	}
	entry, err = p.lstat(path)
	if err != nil {
		return baselineReadError(err)
	}
	if !entry.Mode().IsRegular() || entry.Mode().Perm() != expected.mode || !os.SameFile(entry, current.baseline.info) ||
		entry.Size() != current.baseline.info.Size() || !entry.ModTime().Equal(current.baseline.info.ModTime()) {
		return baselineConflict(errors.New("database path changed while checking its baseline"))
	}
	return nil
}

func baselineConflict(cause error) error {
	return &SaveError{Kind: SaveConflict, Op: "check database baseline", Err: cause}
}

func baselineReadError(cause error) error {
	if errors.Is(cause, os.ErrNotExist) || errors.Is(cause, errVaultNotRegular) {
		return baselineConflict(cause)
	}
	// Unreadability does not establish a content conflict. Fail closed without
	// overwriting or silently treating an inaccessible file as a fresh baseline.
	return &SaveError{Kind: SaveFailed, Op: "read database baseline", Err: cause}
}

// prepareChecked checks the authenticated/saved snapshot before encoding and
// retains that exact baseline for the second check immediately before replace.
// The caller must serialize the attempt with its vault session and provide an
// encoder operating on an isolated database. Conflicts do not mutate live state.
func (p *vaultPersistence) prepareChecked(destination string, expected vaultFileBaseline, encode func(io.Writer) error) (*preparedVaultFile, error) {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return nil, persistenceError("resolve database destination", err)
	}
	if err := p.checkBaseline(destination, expected); err != nil {
		return nil, err
	}
	prepared, err := p.prepare(destination, expected.mode, encode)
	if err != nil {
		return nil, err
	}
	prepared.expected = &expected
	return prepared, nil
}

// commitAndUpdateBaseline updates the session baseline for every committed
// result, including durability-uncertain errors. It never rereads the destination
// after commit: the new fingerprint and identity were captured from the prepared
// file, so an intervening external write cannot be adopted as our saved state.
// The caller holds its vault's exclusive lock and must also publish the matching
// unlocked working database whenever committed is true, before releasing it.
func (f *preparedVaultFile) commitAndUpdateBaseline(destination string, baseline *vaultFileBaseline) (committed bool, err error) {
	if baseline == nil || f.expected == nil {
		return false, errors.Join(errors.New("a checked database baseline is required"), f.discard())
	}
	committed, err = f.commit(destination)
	if committed {
		*baseline = f.baseline
	}
	return committed, err
}
