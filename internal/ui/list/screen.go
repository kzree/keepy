// Package list implements the screen for listing vault entries and showing details of a selected entry.
package list

import (
	"strings"

	"golang.design/x/clipboard"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/util"
)

type pane int

const (
	listPane pane = iota
	createPane
)

const paneGap = 1

type ListModel struct {
	showSidePane bool
	activePane   pane
	table        table.Model
}

func NewListModel() ListModel {
	err := clipboard.Init()
	if err != nil {
		panic("Failed to initialize clipboard: " + err.Error())
	}

	return ListModel{
		activePane: listPane,
		table:      createEntryTable(),
	}
}

func (m ListModel) Init() tea.Cmd {
	return nil
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			if !m.showSidePane {
				break
			}

			if m.activePane == listPane {
				m.activePane = createPane
			} else {
				m.activePane = listPane
			}
		case "c":
			if m.activePane == listPane {
				entry := m.table.SelectedRow()[3] // Hacky af right now
				clipboard.Write(clipboard.FmtText, []byte(entry))
			}
		}
	}

	t, cmd := m.table.Update(msg)
	m.table = t

	return m, cmd
}

func (m *ListModel) SetEntries(entries []service.VaultEntry) {
	rows := make([]table.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, table.Row{
			entry.Title,
			entry.URL,
			entry.Username,
			entry.Password,
		})
	}
	m.table.SetRows(rows)
}

func createEntryTable() table.Model {
	columns := []table.Column{
		{Title: "Title", Width: 20},
		{Title: "URL", Width: 20},
		{Title: "Email", Width: 20},
		{Title: "pwd", Width: 0},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(1),
		table.WithWidth(42),
	)

	s := table.DefaultStyles()
	s.Selected = s.Selected.
		Foreground(lipgloss.BrightBlue).
		Bold(true)
	t.SetStyles(s)

	return t
}

func (m ListModel) renderPane(isActive bool, width, height int, content string) string {
	style := style.GetPaneStyle(isActive)
	return style.
		Width(max(0, width)).
		Height(max(0, height)).
		Render(content)
}

func (m *ListModel) getPaneWidths(contentWidth int) (int, int) {
	availableWidth := max(0, contentWidth-paneGap)
	third := availableWidth / 3

	leftWidth := util.Ternary(m.showSidePane, third*2, availableWidth)
	rightWidth := third

	return leftWidth, rightWidth
}

func (m *ListModel) SetListTableSize(contentWidth, contentHeight int) {
	leftWidth, _ := m.getPaneWidths(contentWidth)
	m.table.SetWidth(leftWidth)
	m.table.SetHeight(max(1, contentHeight))
}

func (m ListModel) View(contentWidth, contentHeight int) string {
	_, rightWidth := m.getPaneWidths(contentWidth)

	left := m.table.View()
	right := util.Ternary(m.showSidePane, m.renderPane(m.activePane == createPane, rightWidth, contentHeight, "Detail"), "")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", paneGap),
		right,
	)
}
