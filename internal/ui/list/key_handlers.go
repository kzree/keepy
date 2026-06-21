package list

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/ui/list/search"
)

func (m ListModel) handleCopyEntry() (ListModel, tea.Cmd) {
	if m.showSearch {
		return m, nil
	}
	if m.activePane == listPane {
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(m.entries) {
			return m, nil
		}
		entry := m.entries[idx]

		return m, func() tea.Msg {
			return CopyPasswordRequestMsg{
				entry.ID,
			}
		}
	}

	return m, nil
}

func (m ListModel) handleOpenNewEntryForm() (ListModel, tea.Cmd) {
	if m.showSearch {
		return m, nil
	}

	m.showSidePane = true
	m.activePane = createPane
	m.resizeTable()
	return m, nil
}

func (m ListModel) handleClearFilter() (ListModel, tea.Cmd) {
	if m.showSearch {
		return m, nil
	}

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
