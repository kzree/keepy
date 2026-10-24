package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

// testClock is a manually advanced clock for inactivity timing.
type testClock struct{ current time.Time }

func (c *testClock) now() time.Time          { return c.current }
func (c *testClock) advance(d time.Duration) { c.current = c.current.Add(d) }

// loggedInRootWithLock opens a vault through login with a 5-minute lock delay.
func loggedInRootWithLock(t *testing.T, vault *outcomeVault) (RootModel, *testClock) {
	t.Helper()
	testutil.TempHome(t)
	clock := &testClock{current: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	r := rootWithVault(t, vault)
	r.now, r.lockDelay = clock.now, 5*time.Minute
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "vault.kdbx", Password: "pw"})
	require.Equal(t, screenList, r.activeView)
	return r, clock
}

func sendLockCheck(t *testing.T, r *RootModel) tea.Cmd {
	t.Helper()
	return updateRoot(r, lockCheckMsg{ID: r.lockCheckID})
}

func TestIdleVaultLocksAndUnlocksWithPassword(t *testing.T) {
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Secret entry"}}}
	r, clock := loggedInRootWithLock(t, vault)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?'}))
	require.True(t, r.helpVisible)
	clock.advance(5 * time.Minute)

	sendLockCheck(t, &r)

	assert.Equal(t, screenLogin, r.activeView)
	assert.Equal(t, 1, vault.closes)
	assert.False(t, r.helpVisible)
	content := r.View().Content
	assert.Contains(t, content, "Vault locked after inactivity")
	assert.NotContains(t, content, "Secret entry")
	assert.Contains(t, content, "vault.kdbx", "the saved database stays selected")

	for _, char := range "pw" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, 2, vault.authCalls)
	assert.Equal(t, "pw", vault.authPassword)
	assert.Equal(t, "vault.kdbx", vault.authCredentials.DBPath)
	assert.NotContains(t, r.View().Content, "Vault locked after inactivity")
}

func TestInputPostponesLock(t *testing.T) {
	vault := &outcomeVault{}
	r, clock := loggedInRootWithLock(t, vault)
	clock.advance(3 * time.Minute)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	clock.advance(2 * time.Minute)

	assert.NotNil(t, sendLockCheck(t, &r), "a check before the idle period ends is rescheduled")
	assert.Equal(t, screenList, r.activeView)

	clock.advance(3 * time.Minute)
	sendLockCheck(t, &r)
	assert.Equal(t, screenLogin, r.activeView)
}

func TestLockDiscardsDraftAndClearsCopiedPassword(t *testing.T) {
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Entry"}}, passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r, clock := loggedInRootWithLock(t, vault)
	clip := &fakeClipboard{}
	r.clipboardReady, r.writeClipboard, r.clipboardClearDelay = true, clip.write, 2*time.Minute
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'c'}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	require.True(t, r.list.IsTextInputActive())
	clock.advance(5 * time.Minute)

	sendLockCheck(t, &r)

	assert.Equal(t, screenLogin, r.activeView)
	assert.Equal(t, []string{"secret", ""}, clip.writes)
	assert.Contains(t, r.View().Content, "Unsaved entry changes were discarded")
}

func TestLockWaitsForAcceptedVaultOperation(t *testing.T) {
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Entry"}}}
	r, clock := loggedInRootWithLock(t, vault)
	work := prepareDelayedOperation(t, &r, "delete")
	clock.advance(10 * time.Minute)

	assert.NotNil(t, sendLockCheck(t, &r))
	assert.Equal(t, screenList, r.activeView)
	assert.Zero(t, vault.closes)

	sendRoot(t, &r, work())
	assert.Equal(t, 1, vault.mutations)
	sendLockCheck(t, &r)
	assert.Equal(t, screenLogin, r.activeView)
}

func TestStaleOrDisabledLockChecksDoNothing(t *testing.T) {
	vault := &outcomeVault{}
	r, clock := loggedInRootWithLock(t, vault)
	stale := lockCheckMsg{ID: r.lockCheckID}
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "vault.kdbx", Password: "pw"})
	clock.advance(time.Hour)
	sendRoot(t, &r, stale)
	assert.Equal(t, screenList, r.activeView, "a check from an earlier session is ignored")

	r.lockDelay = 0
	sendLockCheck(t, &r)
	assert.Equal(t, screenList, r.activeView)
	assert.Zero(t, vault.closes)
}

func TestLockTimeoutComesFromConfig(t *testing.T) {
	for value, want := range map[string]time.Duration{"": service.DefaultLockTimeout, "15m": 15 * time.Minute, "off": 0, "later": service.DefaultLockTimeout} {
		home := testutil.TempHome(t)
		path := filepath.Join(home, ".config", "keepy", "config.toml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("[preferences]\nlock_timeout = '"+value+"'\n"), 0o600))
		r := NewRootModel()
		assert.Equal(t, want, r.lockDelay, value)
		if value == "later" {
			assert.Contains(t, r.renderBottomBar(118, 28), "Lock timeout setting ignored")
		} else {
			assert.NotContains(t, r.renderBottomBar(118, 28), "Lock timeout", value)
		}
	}
}
