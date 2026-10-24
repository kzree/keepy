package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/ui/common"
)

func (r RootModel) handleKeyPressMsg(msg tea.KeyPressMsg) (RootModel, tea.Cmd, bool) {
	if key.Matches(msg, r.keys.ShowHelpAnywhere) || (key.Matches(msg, r.keys.ShowHelp) && (r.helpVisible || !r.isTextInputActive())) {
		r.helpVisible = !r.helpVisible
		if r.helpVisible {
			r.keymapHelp = common.NewKeymapHelpModel(r.locale, r.keys, r.keymapError != nil)
		}
		r.resizeViews()
		return r, nil, true
	}
	if r.helpVisible {
		switch {
		case msg.String() == control.CloseEntryForm:
			r.helpVisible = false
			return r, nil, true
		case key.Matches(msg, r.keys.Quit, r.keys.QuitAlt):
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
		if key.Matches(msg, r.keys.Quit) || (key.Matches(msg, r.keys.QuitAlt) && r.activeView == screenList && !r.list.IsTextInputActive()) {
			r.quitAfterOperation = true
		}
		return r, nil, true
	}
	switch {
	case key.Matches(msg, r.keys.Quit):
		return r, tea.Quit, true
	case key.Matches(msg, r.keys.Reload) && r.activeView == screenLogin && r.configLoadError != nil:
		r, cmd := r.retryConfiguration()
		return r, cmd, true
	case key.Matches(msg, r.keys.QuitAlt):
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
