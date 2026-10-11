package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type vaultFileBaseline struct {
	fingerprint [sha256.Size]byte
	mode        os.FileMode
}

type vaultFileSnapshot struct {
	data     []byte
	baseline vaultFileBaseline
}

type vaultReadFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

// Close must release the handle even when it reports an error, like os.File.
type vaultTempFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

// vaultPersistence is a per-vault filesystem boundary. Tests can replace
// operations before use; hooks must not be modified concurrently with vault
// operations. Preparation does not mutate or lock a database: the caller
// supplies an encoder for an already isolated, encoding-ready database.
type vaultPersistence struct {
	open       func(string) (vaultReadFile, error)
	createTemp func(string, string) (vaultTempFile, error)
	remove     func(string) error
	// replace reports namespace commitment independently of post-commit errors.
	// Implementations must never delete the destination first or fall back to
	// copying. Hooks returning committed=false must leave the destination intact.
	replace       func(string, string) (committed bool, err error)
	syncDirectory func(string) error
}

func newVaultPersistence() *vaultPersistence {
	return &vaultPersistence{
		open: func(path string) (vaultReadFile, error) { return os.Open(path) },
		createTemp: func(dir, pattern string) (vaultTempFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		remove:        os.Remove,
		replace:       replaceVaultFile,
		syncDirectory: syncVaultDirectory,
	}
}

// read captures permissions from the opened file and fingerprints the same
// bytes the caller will decode, even if the path is replaced during the read.
func (p *vaultPersistence) read(path string) (snapshot *vaultFileSnapshot, err error) {
	file, err := p.open(path)
	if err != nil {
		return nil, persistenceError("open database", err)
	}
	defer func() {
		err = errors.Join(err, persistenceError("close database", file.Close()))
		if err != nil {
			snapshot = nil
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, persistenceError("stat database", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("database path must refer to a regular file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, persistenceError("read database", err)
	}
	return &vaultFileSnapshot{
		data: data,
		baseline: vaultFileBaseline{
			fingerprint: sha256.Sum256(data),
			mode:        info.Mode().Perm(),
		},
	}, nil
}

// preparedVaultFile owns a closed temporary file until replace or discard
// succeeds. It is used serially by its owner, not shared between goroutines.
type preparedVaultFile struct {
	path        string
	baseline    vaultFileBaseline
	persistence *vaultPersistence
}

// prepare writes beside destination without modifying destination. On failure,
// it closes and removes the temporary file, preserving cleanup errors alongside
// the primary cause. On success, the caller must replace or discard the result.
func (p *vaultPersistence) prepare(destination string, mode os.FileMode, encode func(io.Writer) error) (prepared *preparedVaultFile, err error) {
	file, err := p.createTemp(filepath.Dir(destination), ".keepy-*.tmp")
	if err != nil {
		return nil, persistenceError("create temporary database", err)
	}
	path := file.Name()
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, persistenceError("close temporary database", file.Close()))
		}
		if prepared == nil {
			err = errors.Join(err, persistenceError("remove temporary database", p.remove(path)))
		}
	}()
	hash := sha256.New()
	writer := &vaultEncodingWriter{writer: io.MultiWriter(file, hash)}
	encodeErr := encode(writer)
	// Some encoder paths ignore individual writes. Never accept a successful
	// Encode result after an earlier failed or short write.
	if writer.err != nil && !errors.Is(encodeErr, writer.err) {
		encodeErr = errors.Join(encodeErr, writer.err)
	}
	if encodeErr != nil {
		return nil, persistenceError("encode temporary database", encodeErr)
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		return nil, persistenceError("set database permissions", err)
	}
	if err := file.Sync(); err != nil {
		return nil, persistenceError("sync temporary database", err)
	}
	closed = true
	if err := file.Close(); err != nil {
		return nil, persistenceError("close temporary database", err)
	}
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], hash.Sum(nil))
	return &preparedVaultFile{
		path: path,
		baseline: vaultFileBaseline{
			fingerprint: fingerprint,
			mode:        mode.Perm(),
		},
		persistence: p,
	}, nil
}

// vaultEncodingWriter makes write errors sticky, including short writes that
// violate io.Writer's contract, so an encoder cannot accidentally hide them.
type vaultEncodingWriter struct {
	writer io.Writer
	err    error
}

func (w *vaultEncodingWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

// replace transfers ownership as soon as namespace replacement commits, even
// when a subsequent platform flush/close fails. A committed result must never be
// retried as a mutation. It does not sync the parent directory or publish live
// state; the transaction owner must publish the matching state before returning
// a committed error. Its errors are plain errors, not SaveError outcomes.
func (f *preparedVaultFile) replace(destination string) (committed bool, err error) {
	if f.path == "" {
		return false, errors.New("temporary database already consumed")
	}
	// Atomic replacement requires a same-directory prepared file. Resolve the
	// absolute spelling before comparing to allow relative destinations in tests.
	fromDir, err := filepath.Abs(filepath.Dir(f.path))
	if err != nil {
		return false, err
	}
	toDir, err := filepath.Abs(filepath.Dir(destination))
	if err != nil {
		return false, err
	}
	if fromDir != toDir {
		return false, errors.New("database replacement must stay in the prepared directory")
	}
	committed, err = f.persistence.replace(f.path, destination)
	if committed {
		f.path = ""
	}
	if err != nil {
		return committed, persistenceError("replace database", err)
	}
	if !committed {
		return false, errors.New("database replacement returned without a commit")
	}
	return true, nil
}

// commit replaces and requests the platform's supported durability checks. The
// bool remains true on post-commit failure. A transaction owner must publish the
// matching unlocked database/baseline whenever it is true, before releasing its
// lock. No live vault state is modified by this filesystem helper.
func (f *preparedVaultFile) commit(destination string) (committed bool, err error) {
	committed, err = f.replace(destination)
	if !committed {
		return false, errors.Join(err, f.discard())
	}
	if err != nil {
		return committed, err
	}
	if err := f.persistence.syncDirectory(filepath.Dir(destination)); err != nil {
		return true, persistenceError("sync database directory", err)
	}
	return true, nil
}

func (f *preparedVaultFile) discard() error {
	if f.path == "" {
		return nil
	}
	if err := f.persistence.remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return persistenceError("remove temporary database", err)
	}
	f.path = ""
	return nil
}

func persistenceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
