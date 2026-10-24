package list

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/common"
)

func (m ListModel) handleOpenEditEntryMsg(msg OpenEditEntryMsg) (ListModel, tea.Cmd) {
	if !m.AcceptsEditOpen(msg.RequestID) {
		return m, nil
	}
	m.openingEdit = false
	if msg.Error != nil {
		m.saveNotice = msg.Error
		m.resizeTable()
		return m, nil
	}
	m.showSidePane = true
	m.activePane = createPane
	m.saveNotice = nil
	cmd := m.entryForm.Edit(msg.Entry.ID, service.NewVaultEntry{
		Title:    msg.Entry.Title,
		Username: msg.Entry.Username,
		URL:      msg.Entry.URL,
		Password: msg.Password,
	})
	m.resizeTable()
	return m, cmd
}

func (m ListModel) handleCopyFlashDoneMsg(msg copyFlashDoneMsg) (ListModel, tea.Cmd) {
	if msg.id == m.copyFlashID {
		m.copyFlashActive = false
		setSelectedStyle(&m.table, normalSelectedStyle())
	}
	return m, nil
}

func (m ListModel) handleCopyPasswordSuccessMsg() (ListModel, tea.Cmd) {
	m.copyFlashID++
	m.copyFlashActive = true
	setSelectedStyle(&m.table, copiedSelectedStyle())
	return m, clearCopyFlashCmd(m.copyFlashID)
}

func (m ListModel) handleCloseNewEntryForm() (ListModel, tea.Cmd) {
	m.openingEdit = false
	m.activePane = listPane
	m.showSidePane = false
	m.resizeTable()
	cmd := m.entryForm.StartNew()
	return m, cmd
}

func (m ListModel) handleCloseDeletePrompt(msg common.CloseConfirmMsg) (ListModel, tea.Cmd) {
	if !m.showDeletePrompt || m.deleting || msg.RequestID != m.deleteRequestID {
		return m, nil
	}
	m.showDeletePrompt = false
	m.resizeTable()
	if msg.Success {
		m.deleting = true
		cmd := msg.Cmd
		return m, cmd
	}

	return m, nil
}
