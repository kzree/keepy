package service

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows FILE_RENAME_INFO has a four-byte union before RootDirectory. Calculate
// the variable filename offset from the native architecture, not a fixed value.
type vaultRenameInfo struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// replaceVaultFile requests one same-volume namespace replacement by handle.
// It deliberately avoids ReplaceFile's documented partial-effect failures and
// MoveFileEx's copy/delete fallback. The supported atomicity model is local NTFS
// rename semantics, not arbitrary network/provider/filter-driver behavior.
// Go permission bits do not represent Windows DACLs or alternate data streams;
// this operation does not claim to preserve those additional metadata classes.
func replaceVaultFile(from, to string) (committed bool, err error) {
	from, err = windowsVaultPath(from)
	if err != nil {
		return false, err
	}
	to, err = windowsVaultPath(to)
	if err != nil {
		return false, err
	}
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return false, err
	}
	target, err := windows.UTF16FromString(to)
	if err != nil {
		return false, err
	}
	handle, err := windows.CreateFile(source, windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return false, err
	}
	defer func() {
		err = errors.Join(err, persistenceError("close replacement handle", windows.CloseHandle(handle)))
	}()
	var layout vaultRenameInfo
	offset := int(unsafe.Offsetof(layout.FileName))
	size := max(int(unsafe.Sizeof(layout)), offset+2*len(target))
	buffer := make([]byte, size)
	info := (*vaultRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.ReplaceIfExists = 1
	info.FileNameLength = uint32(2 * (len(target) - 1))
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[offset])), len(target)), target)
	err = windows.SetFileInformationByHandle(handle, windows.FileRenameInfo, &buffer[0], uint32(len(buffer)))
	runtime.KeepAlive(buffer)
	if err != nil {
		return false, err
	}
	// The original path has been replaced. A flush or close failure from this
	// point onward cannot be classified as an uncommitted save.
	committed = true
	return true, persistenceError("flush replacement file", windows.FlushFileBuffers(handle))
}

// Native calls do not receive os.Open's automatic long-path normalization.
func windowsVaultPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(abs, `\\?\`) {
		return abs, nil
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(abs, `\\`), nil
	}
	return `\\?\` + abs, nil
}

// Windows has no implemented Unix-equivalent parent-directory fsync. The
// prepared file is synced before rename and its handle is flushed afterward.
// Success confirms those checks, not a universal power-loss durability guarantee
// for the directory entry. Do not attempt a privileged volume-wide flush.
func syncVaultDirectory(string) error { return nil }
