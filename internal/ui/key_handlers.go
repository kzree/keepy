package ui

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
)

func (r RootModel) handleKeyPressMsg(msg tea.KeyPressMsg) (RootModel, tea.Cmd, bool) {
	switch msg.String() {
	case control.QuitApplication, control.QuitApplicationAlt:
		return r, tea.Quit, true
	}

	return r, nil, false
}
