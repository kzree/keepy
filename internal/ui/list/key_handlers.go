package list

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list/search"
)

func (m *ListModel) getEntryOnCursor() *service.VaultEntry {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.filteredEntries) {
		return nil
	}
	return &m.filteredEntries[idx]
}

func (m ListModel) handleCopyEntry() (ListModel, tea.Cmd) {
	if m.activePane == listPane {
		entry := m.getEntryOnCursor()
		if entry == nil {
			return m, nil
		}

		return m, func() tea.Msg {
			return CopyPasswordRequestMsg{
				entry.ID,
			}
		}
	}

	return m, nil
}

func (m ListModel) handleOpenNewEntryForm() (ListModel, tea.Cmd) {
	m.openingEdit = false
	m.showSidePane = true
	m.activePane = createPane
	m.saveNotice = nil
	m.resizeTable()
	cmd := m.entryForm.StartNew()
	return m, cmd
}

func (m ListModel) handleOpenEditEntryForm() (ListModel, tea.Cmd) {
	entry := m.getEntryOnCursor()
	if entry == nil {
		return m, nil
	}
	m.openingEdit = true
	m.editRequestID++
	requestID := m.editRequestID

	return m, func() tea.Msg {
		return EditEntryRequestMsg{RequestID: requestID, Entry: *entry}
	}
}

func (m ListModel) handleClearFilter() (ListModel, tea.Cmd) {
	s, cmd := m.search.Update(search.ClearSearchMsg{})
	m.search = s
	m.FilterEntries("")
	return m, cmd
}

func (m ListModel) handleShowSearch() (ListModel, tea.Cmd) {
	if !m.showSearch {
		m.showSearch = true
		m.resizeTable()
	}
	return m, nil
}

func (m ListModel) handleCloseSearch() (ListModel, tea.Cmd) {
	if m.showSearch {
		m.showSearch = false
		m.resizeTable()
	}
	return m, nil
}

func (m ListModel) handleOpenDeletePrompt() (ListModel, tea.Cmd) {
	if m.activePane == listPane {
		entry := m.getEntryOnCursor()
		if entry == nil {
			return m, nil
		}
		m.showDeletePrompt = true
		m.openingEdit = false
		m.deleteRequestID++
		requestID := m.deleteRequestID
		cmd := func() tea.Msg {
			return DeleteEntryRequestMsg{
				RequestID: requestID,
				ID:        entry.ID,
			}
		}

		return m, func() tea.Msg {
			return common.OpenConfirmMsg{
				RequestID: requestID,
				Msg:       m.t.Text(key.DeleteConfirmPrompt),
				Cmd:       cmd,
			}
		}
	}

	m.showDeletePrompt = true

	return m, nil
}
