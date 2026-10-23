package ui

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
)

func (r RootModel) handleKeyPressMsg(msg tea.KeyPressMsg) (RootModel, tea.Cmd, bool) {
	switch msg.String() {
	case control.QuitApplication:
		return r, tea.Quit, true
	case control.ReloadVault:
		if r.activeView == screenLogin && r.configLoadError != nil {
			r, cmd := r.retryConfiguration()
			return r, cmd, true
		}
	case control.QuitApplicationAlt:
		if r.activeView == screenLogin || (r.activeView == screenList && r.list.IsTextInputActive()) {
			return r, nil, false
		}
		return r, tea.Quit, true
	}

	return r, nil, false
}
