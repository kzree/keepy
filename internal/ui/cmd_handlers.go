package ui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
)

func (r RootModel) handleLoginSubmitMsg(msg login.LoginSubmitMsg) (RootModel, tea.Cmd) {
	// An unreadable/malformed config must be corrected explicitly, never
	// overwritten by saving credentials from a fallback login form.
	if r.configLoadError != nil || r.activeView != screenLogin || !r.canStartVaultOperation() || !r.login.AcceptsSubmission(msg.RequestID) {
		return r, nil
	}
	creds := service.Credentials{
		DBPath:      msg.DBPath,
		KeyFilePath: msg.KeyFilePath,
	}

	config := service.Config{
		Credentials: creds,
	}
	if r.cfg != nil {
		config = *r.cfg
		config.Credentials = creds
	}
	return r.startVaultOperation(operationLogin, msg.RequestID, func(vault vaultSession) vaultOperationResult {
		result := vaultOperationResult{}
		result.err = vault.Authenticate(msg.DBPath, msg.KeyFilePath, msg.Password, msg.KeyFilePath != "")
		if result.err == nil {
			result.entries = vault.GetEntriesFlat()
			result.config = &config
			result.configError = service.SaveConfig(&config)
		}
		return result
	})
}

func (r RootModel) handleCopyPasswordRequestMsg(msg list.CopyPasswordRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() {
		return r, nil
	}
	if !r.clipboardReady {
		if r.clipboardError == nil {
			r.copyError = errors.New(r.t.Text(key.ErrorClipboardNotReady))
		}
		r.updateApplicationNotice()
		r.resizeViews()
		return r, nil
	}
	pwd, err := r.db.GetEntryPassword(msg.ID)
	if err == nil && pwd == "" {
		err = errors.New(r.t.Text(key.ErrorPasswordNotFound))
	}
	if err == nil && !r.writeClipboard([]byte(pwd)) {
		err = errors.New(r.t.Text(key.ErrorClipboardWrite))
	}
	r.copyError = err
	r.updateApplicationNotice()
	r.resizeViews()
	if err != nil {
		return r, nil
	}
	return r, func() tea.Msg {
		return list.CopyPasswordSuccessMsg{}
	}
}

func (r RootModel) handleWindowSizeMsg(msg tea.WindowSizeMsg) (RootModel, tea.Cmd) {
	r.width = msg.Width
	r.height = msg.Height

	r.resizeViews()

	return r, nil
}

func (r RootModel) handleSubmitEntry(msg entryform.SubmitEntryMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsFormSubmission(msg.RequestID) {
		return r, nil
	}
	return r.startMutation(operationFormSave, msg.RequestID, func(vault vaultSession) error { return vault.AddNewEntry(msg.Entry) })
}

func (r RootModel) handleEditEntryRequestMsg(msg list.EditEntryRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsEditOpen(msg.RequestID) {
		return r, nil
	}
	password, err := r.db.GetEntryPassword(msg.Entry.ID)
	if err != nil {
		return r, func() tea.Msg {
			return list.OpenEditEntryMsg{RequestID: msg.RequestID, Error: err}
		}
	}

	return r, func() tea.Msg {
		return list.OpenEditEntryMsg{RequestID: msg.RequestID, Entry: msg.Entry, Password: password}
	}
}

func (r RootModel) handleSubmitEditEntry(msg entryform.SubmitEditEntryMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsFormSubmission(msg.RequestID) {
		return r, nil
	}
	return r.startMutation(operationFormSave, msg.RequestID, func(vault vaultSession) error { return vault.UpdateEntry(msg.ID, msg.Entry) })
}

func (r RootModel) handleDeleteEntryRequestMsg(msg list.DeleteEntryRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsDelete(msg.RequestID) {
		return r, nil
	}
	return r.startMutation(operationDelete, msg.RequestID, func(vault vaultSession) error { return vault.DeleteEntry(msg.ID) })
}

// Reload is explicit and never repeats the conflicted mutation. Draft state is
// owned by the form and retained for review against the newly authenticated data.
func (r RootModel) handleReloadVault(msg list.ReloadVaultRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsReload(msg.RequestID) {
		return r, nil
	}
	reloadFailed := r.t.Text(key.ErrorReloadFailed)
	return r.startVaultOperation(operationReload, msg.RequestID, func(vault vaultSession) vaultOperationResult {
		result := vaultOperationResult{err: vault.ReAuthenticate()}
		if result.err == nil {
			result.entries = vault.GetEntriesFlat()
		} else {
			result.err = fmt.Errorf("%s: %w", reloadFailed, result.err)
		}
		return result
	})
}
