package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
)

func (r RootModel) handleLoginSubmitMsg(msg login.LoginSubmitMsg) (RootModel, tea.Cmd) {
	creds := service.Credentials{
		DBPath:      msg.DBPath,
		KeyFilePath: msg.KeyFilePath,
	}

	config := service.Config{
		Credentials: creds,
	}
	service.SaveConfig(&config)

	if err := r.db.Authenticate(msg.DBPath, msg.KeyFilePath, msg.Password, msg.KeyFilePath != ""); err != nil {
		return r, func() tea.Msg {
			return login.AuthenticationFailedMsg{Error: err}
		}
	}
	r.list.SetEntries(r.db.GetEntriesFlat())

	w, h := r.getContentSize()
	r.list.SetListTableSize(w, h)
	r.activeView = screenList
	return r, r.list.Init()
}

func (r RootModel) handleCopyPasswordRequestMsg(msg list.CopyPasswordRequestMsg) (RootModel, tea.Cmd) {
	pwd, err := r.db.GetEntryPassword(msg.ID)
	if err != nil {
		return r, func() tea.Msg {
			return list.CopyPasswordFailureMsg{
				Error: err,
			}
		}
	}

	if pwd == "" {
		return r, func() tea.Msg {
			return list.CopyPasswordFailureMsg{
				Error: errors.New("password not found for entry"),
			}
		}
	}

	changed := clipboard.Write(clipboard.FmtText, []byte(pwd))
	if changed == nil {
		return r, func() tea.Msg {
			return list.CopyPasswordFailureMsg{
				Error: errors.New("failed to write password to clipboard"),
			}
		}
	}
	return r, func() tea.Msg {
		return list.CopyPasswordSuccessMsg{}
	}
}

func (r RootModel) handleWindowSizeMsg(msg tea.WindowSizeMsg) (RootModel, tea.Cmd) {
	r.width = msg.Width
	r.height = msg.Height

	w, h := r.getContentSize()
	r.list.SetListTableSize(w, h)

	return r, nil
}

func (r RootModel) handleSubmitEntry(msg entryform.SubmitEntryMsg) (RootModel, tea.Cmd) {
	err := r.db.AddNewEntry(msg.Entry)
	if err != nil {
		return r, func() tea.Msg {
			return entryform.SubmitFailedMsg{
				Error: err,
			}
		}
	}

	return r, func() tea.Msg {
		return entryform.SubmitSuccessMsg{}
	}
}
