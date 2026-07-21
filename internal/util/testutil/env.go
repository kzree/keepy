package testutil

import "testing"

const HomeEnv = "HOME"

func TempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv(HomeEnv, home)
	return home
}
