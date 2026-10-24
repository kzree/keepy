package util

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/util/testutil"
)

func TestPathFromHome(t *testing.T) {
	home := testutil.TempHome(t)

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
		{
			name: "absolute path outside home",
			path: filepath.Join(filepath.Dir(home), "elsewhere", "db.kdbx"),
			want: filepath.Join(filepath.Dir(home), "elsewhere", "db.kdbx"),
		},
		{
			name: "native home shorthand",
			path: "~" + string(filepath.Separator) + filepath.Join("vaults", "db.kdbx"),
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

func TestPathFromHomeRejectsUnsupportedUserExpansion(t *testing.T) {
	_, err := PathFromHome("~someone/vault.kdbx")
	require.Error(t, err)
}

func TestPathFromHomeAbsolutePathDoesNotRequireHome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	got, err := PathFromHome(path)
	require.NoError(t, err)
	assert.Equal(t, path, got)
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
