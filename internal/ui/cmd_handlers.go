package ui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
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
	op := r.beginVaultOperation(operationLogin, msg.RequestID)
	vault := r.db
	return r, func() tea.Msg {
		result := vaultOperationResult{operation: op}
		result.err = vault.Authenticate(msg.DBPath, msg.KeyFilePath, msg.Password, msg.KeyFilePath != "")
		if result.err == nil {
			result.entries = vault.GetEntriesFlat()
			result.config = &config
			result.configError = service.SaveConfig(&config)
		}
		return result
	}
}

func (r RootModel) handleCopyPasswordRequestMsg(msg list.CopyPasswordRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() {
		return r, nil
	}
	if !r.clipboardReady {
		if r.clipboardError == nil {
			r.copyError = errors.New("clipboard is not ready")
		}
		r.updateApplicationNotice()
		r.resizeViews()
		return r, nil
	}
	pwd, err := r.db.GetEntryPassword(msg.ID)
	if err == nil && pwd == "" {
		err = errors.New("password not found for entry")
	}
	if err == nil && !r.writeClipboard([]byte(pwd)) {
		err = errors.New("failed to write password to clipboard")
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
	vault := r.db
	return r.startFormSave(msg.RequestID, func() error { return vault.AddNewEntry(msg.Entry) })
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
	vault := r.db
	return r.startFormSave(msg.RequestID, func() error { return vault.UpdateEntry(msg.ID, msg.Entry) })
}

func (r RootModel) startFormSave(requestID uint64, mutate func() error) (RootModel, tea.Cmd) {
	op := r.beginVaultOperation(operationFormSave, requestID)
	vault := r.db
	return r, func() tea.Msg {
		result := vaultOperationResult{operation: op, err: mutate()}
		if saveCommitted(result.err) {
			result.entries = vault.GetEntriesFlat()
		}
		return result
	}
}

func saveCommitted(err error) bool {
	var outcome *service.SaveError
	return err == nil || (errors.As(err, &outcome) && outcome.Committed())
}

// finishFormSave closes a form for every committed outcome. Reporting a committed
// durability error as a form failure would offer a retry of an applied mutation.
func (r RootModel) finishFormSave(err error, entries []service.VaultEntry) (RootModel, tea.Cmd) {
	if !saveCommitted(err) {
		var cmd tea.Cmd
		r.list, cmd = r.list.Update(entryform.SubmitFailedMsg{Error: err})
		r.resizeViews()
		return r, cmd
	}

	r.list.SetEntries(entries)
	var cmd tea.Cmd
	r.list, cmd = r.list.Update(entryform.SubmitSuccessMsg{Warning: err})
	r.resizeViews()
	return r, cmd
}

func (r RootModel) handleDeleteEntryRequestMsg(msg list.DeleteEntryRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsDelete(msg.RequestID) {
		return r, nil
	}
	op := r.beginVaultOperation(operationDelete, msg.RequestID)
	vault := r.db
	return r, tea.Batch(r.operationSpinner.Tick, func() tea.Msg {
		result := vaultOperationResult{operation: op, err: vault.DeleteEntry(msg.ID)}
		if saveCommitted(result.err) {
			result.entries = vault.GetEntriesFlat()
		}
		return result
	})
}

// Reload is explicit and never repeats the conflicted mutation. Draft state is
// owned by the form and retained for review against the newly authenticated data.
func (r RootModel) handleReloadVault(msg list.ReloadVaultRequestMsg) (RootModel, tea.Cmd) {
	if !r.canStartVaultOperation() || !r.list.AcceptsReload(msg.RequestID) {
		return r, nil
	}
	op := r.beginVaultOperation(operationReload, msg.RequestID)
	vault := r.db
	return r, tea.Batch(r.operationSpinner.Tick, func() tea.Msg {
		result := vaultOperationResult{operation: op, err: vault.ReAuthenticate()}
		if result.err == nil {
			result.entries = vault.GetEntriesFlat()
		} else {
			result.err = fmt.Errorf("reload failed: %w", result.err)
		}
		return result
	})
}
