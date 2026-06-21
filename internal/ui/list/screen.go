// Package list implements the screen for listing vault entries and showing details of a selected entry.
package list

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/list/search"
	"kzree.com/keepy/internal/util"
)

type pane int

const copyFlashDuration = 100 * time.Millisecond

const (
	listPane pane = iota
	createPane
)

const (
	paneGap      = 1
	searchHeight = 2
)

type ListModel struct {
	showSearch      bool
	showSidePane    bool
	activePane      pane
	table           table.Model
	copyFlashID     int
	entries         []service.VaultEntry
	filteredEntries []service.VaultEntry

	width  int
	height int

	search    search.SearchModel
	entryForm entryform.EntryFormModel
}

func NewListModel() ListModel {
	return ListModel{
		showSearch:   false,
		showSidePane: false,
		activePane:   listPane,
		table:        createEntryTable(),
		search:       search.NewSearchModel(),
		entryForm:    entryform.NewEntryFormModel(),
	}
}

func (m ListModel) Init() tea.Cmd {
	return m.entryForm.Init()
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case copyFlashDoneMsg:
		return m.handleCopyFlashDoneMsg(msg)
	case CopyPasswordSuccessMsg:
		return m.handleCopyPasswordSuccessMsg()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "c":
			return m.handleCopyEntry()
		case "n":
			return m.handleOpenNewEntryForm()
		case "x":
			return m.handleClearFilter()
		case "f":
			return m.handleShowSearch()
		case "esc":
			return m.handleCloseSearch()
		}
	}

	if m.showSearch {
		oldValue := m.search.Value()
		s, cmd := m.search.Update(msg)
		m.search = s
		if m.search.Value() != oldValue {
			m.FilterEntries(m.search.Value())
		}

		cmds = append(cmds, cmd)
	} else if m.showSidePane && m.activePane == createPane {
		entryForm, cmd := m.entryForm.Update(msg)
		m.entryForm = entryForm
		cmds = append(cmds, cmd)
	} else {
		t, cmd := m.table.Update(msg)
		m.table = t
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func entriesToRows(entries []service.VaultEntry) []table.Row {
	rows := make([]table.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, table.Row{
			entry.Title,
			entry.URL,
			entry.Username,
			entry.Group,
		})
	}
	return rows
}

func (m *ListModel) SetEntries(entries []service.VaultEntry) {
	m.entries = entries
	m.filteredEntries = entries
	rows := entriesToRows(entries)
	m.table.SetRows(rows)
}

func (m *ListModel) FilterEntries(val string) {
	if val == "" {
		m.filteredEntries = m.entries
		m.table.SetRows(entriesToRows(m.entries))
		return
	}

	filtered := slices.Collect(func(yield func(service.VaultEntry) bool) {
		for _, v := range m.entries {
			title := strings.ToLower(v.Title)
			substr := strings.ToLower(val)
			if strings.Contains(title, substr) || strings.Contains(v.Username, val) {
				if !yield(v) {
					return
				}
			}
		}
	})

	currentCursor := m.table.Cursor()
	var selected gokeepasslib.UUID

	if currentCursor >= 0 && currentCursor < len(m.filteredEntries) {
		selected = m.filteredEntries[currentCursor].ID
	}

	m.filteredEntries = filtered
	m.table.SetRows(entriesToRows(filtered))
	m.table.SetCursor(0)

	for i, entry := range m.filteredEntries {
		if entry.ID == selected {
			m.table.SetCursor(i)
			break
		}
	}
}

func clearCopyFlashCmd(id int) tea.Cmd {
	return tea.Tick(copyFlashDuration, func(time.Time) tea.Msg {
		return copyFlashDoneMsg{id: id}
	})
}

func (m ListModel) renderPane(isActive bool, width, height int, content string) string {
	style := style.GetPaneStyle(isActive)
	frameW, frameH := style.GetFrameSize()
	return style.
		Width(max(0, width-frameW)).
		Height(max(0, height-frameH)).
		Render(content)
}

func (m *ListModel) getPaneWidths(contentWidth int) (int, int) {
	availableWidth := max(0, contentWidth-paneGap)
	third := availableWidth / 3

	leftWidth := util.Ternary(m.showSidePane, third*2, availableWidth)
	rightWidth := third

	return leftWidth, rightWidth
}

func (m *ListModel) resizeEntryForm(rightWidth int) {
	paneStyle := style.GetPaneStyle(m.activePane == createPane)
	frameW, frameH := paneStyle.GetFrameSize()
	m.entryForm.SetSize(
		max(0, rightWidth-frameW),
		max(0, m.height-frameH),
	)
}

func (m *ListModel) SetListTableSize(contentWidth, contentHeight int) {
	m.width = contentWidth
	m.height = contentHeight

	m.resizeTable()
}

func (m *ListModel) resizeTable() {
	leftWidth, rightWidth := m.getPaneWidths(m.width)
	baseHeight := max(1, m.height)
	m.table.SetWidth(leftWidth)
	m.table.SetHeight(util.Ternary(m.showSearch, baseHeight-searchHeight, baseHeight))

	if m.showSidePane {
		m.resizeEntryForm(rightWidth)
	}
}

func (m ListModel) View(contentWidth, contentHeight int) string {
	_, rightWidth := m.getPaneWidths(contentWidth)

	left := m.table.View()
	if m.showSearch {
		left = lipgloss.JoinVertical(lipgloss.Left, m.search.View(), m.table.View())
	}
	right := util.Ternary(m.showSidePane, m.renderPane(m.activePane == createPane, rightWidth, contentHeight, m.entryForm.View()), "")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", paneGap),
		right,
	)
}
