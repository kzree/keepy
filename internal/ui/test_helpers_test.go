package ui

import (
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

type outcomeVault struct {
	snapshotReads   atomic.Int64
	authCalls       int
	authCredentials service.Credentials
	authPassword    string
	authUsesKey     bool
	passwordReads   int
	authErr         error
	entries         []service.VaultEntry
	passwords       map[gokeepasslib.UUID]string
	saveErr         error
	reloadErr       error
	reloaded        []service.VaultEntry
	mutations       int
	reloads         int
	closes          int
}

func (v *outcomeVault) Authenticate(dbPath, keyPath, password string, useKey bool) error {
	v.authCalls++
	v.authCredentials = service.Credentials{DBPath: dbPath, KeyFilePath: keyPath}
	v.authPassword, v.authUsesKey = password, useKey
	return v.authErr
}
func (v *outcomeVault) Close() { v.closes++ }
func (v *outcomeVault) ReAuthenticate() error {
	v.reloads++
	if v.reloadErr == nil {
		v.entries = v.reloaded
	}
	return v.reloadErr
}
func (v *outcomeVault) GetEntriesFlat() []service.VaultEntry {
	v.snapshotReads.Add(1)
	return append([]service.VaultEntry(nil), v.entries...)
}
func (v *outcomeVault) GetEntryPassword(id gokeepasslib.UUID) (string, error) {
	v.passwordReads++
	return v.passwords[id], nil
}
func (v *outcomeVault) AddNewEntry(entry service.NewVaultEntry) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		v.entries = append(v.entries, service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: entry.Title})
	}
	return v.saveErr
}
func (v *outcomeVault) UpdateEntry(id gokeepasslib.UUID, entry service.NewVaultEntry) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		for i := range v.entries {
			if v.entries[i].ID == id {
				v.entries[i].Title = entry.Title
			}
		}
	}
	return v.saveErr
}
func (v *outcomeVault) DeleteEntry(id gokeepasslib.UUID) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		for i := range v.entries {
			if v.entries[i].ID == id {
				v.entries = append(v.entries[:i], v.entries[i+1:]...)
				break
			}
		}
	}
	return v.saveErr
}

func rootWithVault(t *testing.T, vault *outcomeVault) RootModel {
	t.Helper()
	r := RootModel{activeView: screenList, db: vault, list: list.NewListModel(locale.En, control.DefaultKeymap()), width: 120, height: 30, now: time.Now}
	r.setLocale(locale.En)
	r.keys = control.DefaultKeymap()
	r.list.SetEntries(vault.GetEntriesFlat())
	r.list.SetListTableSize(110, 25)
	return r
}

func updateRoot(r *RootModel, msg tea.Msg) tea.Cmd {
	model, cmd := r.Update(msg)
	*r = model.(RootModel)
	return cmd
}

func drainRoot(t *testing.T, r *RootModel, cmd tea.Cmd) {
	t.Helper()
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd { return updateRoot(r, msg) })
}

func sendRoot(t *testing.T, r *RootModel, msg tea.Msg) {
	t.Helper()
	drainRoot(t, r, updateRoot(r, msg))
}

// captureEntrySubmission runs form lifecycle commands but defers delivering the
// resulting add request, letting tests interleave other messages before saving.
func captureEntrySubmission(t *testing.T, r *RootModel, cmd tea.Cmd) entryform.SubmitEntryMsg {
	t.Helper()
	var requests []entryform.SubmitEntryMsg
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
		if request, ok := msg.(entryform.SubmitEntryMsg); ok {
			requests = append(requests, request)
			return nil
		}
		return updateRoot(r, msg)
	})
	require.Len(t, requests, 1)
	require.NotZero(t, requests[0].RequestID)
	return requests[0]
}

func submitLogin(t *testing.T, r *RootModel, msg login.LoginSubmitMsg) {
	t.Helper()
	r.activeView = screenLogin
	r.login = login.NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: msg.DBPath, KeyFilePath: msg.KeyFilePath}}, locale.En, control.DefaultKeymap())
	drainRoot(t, r, r.login.Init())
	for _, char := range msg.Password {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
}

func submitThroughUI(t *testing.T, r *RootModel, operation string) {
	t.Helper()
	if operation == "delete" {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'd'}))
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'y'}))
		return
	}
	open := 'n'
	if operation == "update" {
		open = 'e'
	}
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: open}))
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}))
	for _, char := range "Committed" {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	for range 4 {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
}
