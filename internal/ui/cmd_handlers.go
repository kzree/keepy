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
	if r.configLoadError != nil {
		return r, nil
	}
	creds := service.Credentials{
		DBPath:      msg.DBPath,
		KeyFilePath: msg.KeyFilePath,
	}

	if err := r.db.Authenticate(msg.DBPath, msg.KeyFilePath, msg.Password, msg.KeyFilePath != ""); err != nil {
		return r, func() tea.Msg {
			return login.AuthenticationFailedMsg{Error: err}
		}
	}
	r.list.SetEntries(r.db.GetEntriesFlat())

	config := service.Config{
		Credentials: creds,
	}
	if r.cfg != nil {
		config = *r.cfg
		config.Credentials = creds
	}
	r.cfg = &config
	// Remember the active credentials even if persisting settings fails. A
	// configuration error must not undo successful vault authentication.
	r.configSaveError = service.SaveConfig(r.cfg)
	r.updateApplicationNotice()

	r.activeView = screenList
	r.resizeViews()
	return r, r.list.Init()
}

func (r RootModel) handleCopyPasswordRequestMsg(msg list.CopyPasswordRequestMsg) (RootModel, tea.Cmd) {
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
	if !r.list.AcceptsFormSubmission(msg.RequestID) {
		return r, nil
	}
	return r.finishFormSave(r.db.AddNewEntry(msg.Entry))
}

func (r RootModel) handleEditEntryRequestMsg(msg list.EditEntryRequestMsg) (RootModel, tea.Cmd) {
	if !r.list.AcceptsEditOpen(msg.RequestID) {
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
	if !r.list.AcceptsFormSubmission(msg.RequestID) {
		return r, nil
	}
	return r.finishFormSave(r.db.UpdateEntry(msg.ID, msg.Entry))
}

func saveCommitted(err error) bool {
	var outcome *service.SaveError
	return err == nil || (errors.As(err, &outcome) && outcome.Committed())
}

// finishFormSave closes a form for every committed outcome. Reporting a committed
// durability error as a form failure would offer a retry of an applied mutation.
func (r RootModel) finishFormSave(err error) (RootModel, tea.Cmd) {
	if !saveCommitted(err) {
		var cmd tea.Cmd
		r.list, cmd = r.list.Update(entryform.SubmitFailedMsg{Error: err})
		r.resizeViews()
		return r, cmd
	}

	r.list.SetEntries(r.db.GetEntriesFlat())
	var cmd tea.Cmd
	r.list, cmd = r.list.Update(entryform.SubmitSuccessMsg{Warning: err})
	r.resizeViews()
	return r, cmd
}

func (r RootModel) handleDeleteEntryRequestMsg(msg list.DeleteEntryRequestMsg) (RootModel, tea.Cmd) {
	if !r.list.AcceptsDelete(msg.RequestID) {
		return r, nil
	}
	err := r.db.DeleteEntry(msg.ID)
	if saveCommitted(err) {
		r.list.SetEntries(r.db.GetEntriesFlat())
	}
	r.list, _ = r.list.Update(list.SaveResultMsg{Error: err})
	r.resizeViews()
	return r, nil
}

// Reload is explicit and never repeats the conflicted mutation. Draft state is
// owned by the form and retained for review against the newly authenticated data.
func (r RootModel) handleReloadVault(msg list.ReloadVaultRequestMsg) (RootModel, tea.Cmd) {
	if !r.list.AcceptsReload(msg.RequestID) {
		return r, nil
	}
	err := r.db.ReAuthenticate()
	if err == nil {
		r.list.SetEntries(r.db.GetEntriesFlat())
	} else {
		err = fmt.Errorf("reload failed: %w", err)
	}
	var cmd tea.Cmd
	r.list, cmd = r.list.Update(list.ReloadVaultResultMsg{Error: err})
	r.resizeViews()
	return r, cmd
}
