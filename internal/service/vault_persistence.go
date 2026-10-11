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
	replace    func(string, string) error
}

func newVaultPersistence() *vaultPersistence {
	return &vaultPersistence{
		open: func(path string) (vaultReadFile, error) { return os.Open(path) },
		createTemp: func(dir, pattern string) (vaultTempFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		remove: os.Remove,
		// This is only a replacement primitive. Platform-specific atomicity
		// and directory durability will be established by the save pipeline.
		replace: os.Rename,
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
	if err := encode(io.MultiWriter(file, hash)); err != nil {
		return nil, persistenceError("encode temporary database", err)
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

// replace invokes the replacement primitive and transfers ownership on success.
// It does not sync the directory or publish live vault state. Its errors are
// plain errors, not transactional SaveError outcomes.
func (f *preparedVaultFile) replace(destination string) error {
	if f.path == "" {
		return errors.New("temporary database already consumed")
	}
	if err := f.persistence.replace(f.path, destination); err != nil {
		return persistenceError("replace database", err)
	}
	f.path = ""
	return nil
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
