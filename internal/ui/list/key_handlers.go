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
	m.showSidePane = true
	m.activePane = createPane
	m.resizeTable()
	return m, m.entryForm.Init()
}

func (m ListModel) handleOpenEditEntryForm() (ListModel, tea.Cmd) {
	entry := m.getEntryOnCursor()
	if entry == nil {
		return m, nil
	}

	return m, func() tea.Msg {
		return EditEntryRequestMsg{Entry: *entry}
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
	m.showDeletePrompt = true

	if m.activePane == listPane {
		entry := m.getEntryOnCursor()
		cmd := func() tea.Msg {
			return DeleteEntryRequestMsg{
				ID: entry.ID,
			}
		}

		return m, func() tea.Msg {
			return common.OpenConfirmMsg{
				Msg: m.t.Text(key.DeleteConfirmPrompt),
				Cmd: cmd,
			}
		}
	}

	m.showDeletePrompt = true

	return m, nil
}
