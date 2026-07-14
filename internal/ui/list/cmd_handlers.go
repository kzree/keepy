package list

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/ui/common"
)

func (m ListModel) handleCopyFlashDoneMsg(msg copyFlashDoneMsg) (ListModel, tea.Cmd) {
	if msg.id == m.copyFlashID {
		setSelectedStyle(&m.table, normalSelectedStyle())
	}
	return m, nil
}

func (m ListModel) handleCopyPasswordSuccessMsg() (ListModel, tea.Cmd) {
	m.copyFlashID++
	setSelectedStyle(&m.table, copiedSelectedStyle())
	return m, clearCopyFlashCmd(m.copyFlashID)
}

func (m ListModel) handleCloseNewEntryForm() (ListModel, tea.Cmd) {
	m.activePane = listPane
	m.showSidePane = false
	return m, nil
}

func (m ListModel) handleCloseDeletePrompt(msg common.CloseConfirmMsg) (ListModel, tea.Cmd) {
	m.showDeletePrompt = false
	if msg.Success {
		cmd := msg.Cmd
		return m, cmd
	}

	return m, nil
}
