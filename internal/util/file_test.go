package util

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathFromHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "home shorthand",
			path: "~",
			want: home,
		},
		{
			name: "path below home shorthand",
			path: "~/.config/keepy/config.json",
			want: filepath.Join(home, ".config", "keepy", "config.json"),
		},
		{
			name: "relative path",
			path: filepath.Join("vaults", "db.kdbx"),
			want: filepath.Join(home, "vaults", "db.kdbx"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PathFromHome(tt.path)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenOrCreateFileCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "file.txt")

	file, err := OpenOrCreateFile(path)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}

func TestOpenOrCreateFileKeepsExistingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("existing contents"), 0o644))

	file, err := OpenOrCreateFile(path)
	require.NoError(t, err)
	defer file.Close()

	contents, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "existing contents", string(contents))
}

func TestOpenOrCreateFileAndTruncateClearsExistingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("existing contents"), 0o644))

	file, err := OpenOrCreateFileAndTruncate(path)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Empty(t, contents)
}
