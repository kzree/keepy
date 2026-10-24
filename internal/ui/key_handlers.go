package ui

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/ui/common"
)

func (r RootModel) handleKeyPressMsg(msg tea.KeyPressMsg) (RootModel, tea.Cmd, bool) {
	key := msg.String()
	if key == control.ShowHelpAnywhere || (key == control.ShowHelp && (r.helpVisible || !r.isTextInputActive())) {
		r.helpVisible = !r.helpVisible
		if r.helpVisible {
			r.keymapHelp = common.NewKeymapHelpModel()
		}
		r.resizeViews()
		return r, nil, true
	}
	if r.helpVisible {
		switch key {
		case "esc":
			r.helpVisible = false
			return r, nil, true
		case control.QuitApplication, control.QuitApplicationAlt:
			if r.pendingOperation.kind != operationNone {
				r.quitAfterOperation = true
				return r, nil, true
			}
			return r, tea.Quit, true
		}
		var cmd tea.Cmd
		r.keymapHelp, cmd = r.keymapHelp.Update(msg)
		return r, cmd, true
	}
	if r.pendingOperation.kind != operationNone {
		if msg.String() == control.QuitApplication || (msg.String() == control.QuitApplicationAlt && r.activeView == screenList && !r.list.IsTextInputActive()) {
			r.quitAfterOperation = true
		}
		return r, nil, true
	}
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

func (r RootModel) isTextInputActive() bool {
	return (r.activeView == screenLogin && r.configLoadError == nil && r.login.IsTextInputActive()) || (r.activeView == screenList && r.list.IsTextInputActive())
}

func (r RootModel) keymapContext() string {
	if r.activeView == screenLogin {
		if r.configLoadError != nil {
			return control.RecoveryKeys
		}
		return r.login.KeymapContext()
	}
	return r.list.KeymapContext()
}
