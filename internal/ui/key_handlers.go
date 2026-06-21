package ui

import tea "charm.land/bubbletea/v2"

func (r RootModel) handleKeyPressMsg(msg tea.KeyPressMsg) (RootModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c", "q":
		return r, tea.Quit, true
	}

	return r, nil, false
}
