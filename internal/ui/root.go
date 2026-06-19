// Package ui contains the user interface components of the application
package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
)

type RootModel struct {
	activeView screen

	db *service.Vault

	login login.LoginModel
	list  list.ListModel

	width  int
	height int
}

func NewRootModel() RootModel {
	return RootModel{
		activeView: screenLogin,
		login:      login.NewLoginModel(),
		list:       list.NewListModel(),
		db:         service.NewVault(),
	}
}

func (r RootModel) Init() tea.Cmd {
	err := clipboard.Init()
	// TODO: better error handling
	if err != nil {
		panic("Failed to initialize clipboard: " + err.Error())
	}

	switch r.activeView {
	case screenLogin:
		return r.login.Init()
	}

	return nil
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case login.LoginSubmitMsg:
		creds := service.Credentials{
			DBPath:      msg.DBPath,
			KeyFilePath: msg.KeyFilePath,
		}
		service.SaveCredentials(&creds)

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

	case list.CopyPasswordRequestMsg:
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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return r, tea.Quit
		}

	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height

		w, h := r.getContentSize()

		r.list.SetListTableSize(w, h)
	}

	switch r.activeView {
	case screenLogin:
		login, cmd := r.login.Update(msg)
		r.login = login
		return r, cmd
	case screenList:
		list, cmd := r.list.Update(msg)
		r.list = list
		return r, cmd
	}

	return r, nil
}

func (r *RootModel) renderCurrentView(contentWidth, contentHeight int) string {
	switch r.activeView {
	case screenLogin:
		return r.login.View()
	case screenList:
		return r.list.View(contentWidth, contentHeight)
	}

	return ""
}

func (r *RootModel) getContentSize() (int, int) {
	frameW, frameH := style.Frame.GetFrameSize()

	width := max(0, r.width-frameW)
	height := max(0, r.height-frameH)
	return width, height
}

func (r RootModel) View() tea.View {
	w, h := r.getContentSize()
	body := r.renderCurrentView(w, h)

	return tea.NewView(r.renderFrame(body))
}
