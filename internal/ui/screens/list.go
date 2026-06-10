package screens

import (
	"strings"

	"golang.design/x/clipboard"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
)

type pane int

const (
	listPane pane = iota
	detailPane
)

const paneGap = 1

type ListModel struct {
	activePane pane
	table      table.Model
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
			if m.activePane == listPane {
				m.activePane = detailPane
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
		{Title: "Group", Width: 10},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(1),
		table.WithWidth(42),
	)

	return t
}

func (m ListModel) renderPane(isActive bool, width, height int, content string) string {
	style := style.GetPaneStyle(isActive)
	return style.
		Width(max(0, width)).
		Height(max(0, height)).
		Render(content)
}

func (m *ListModel) getWidth(availableWidth int, isActive bool) int {
	third := availableWidth / 3

	if isActive {
		return third * 2
	}

	return third
}

func (m *ListModel) getPaneWidths(contentWidth int) (int, int) {
	availableWidth := max(0, contentWidth-paneGap)
	leftWidth := m.getWidth(availableWidth, m.activePane == listPane)
	rightWidth := m.getWidth(availableWidth, m.activePane == detailPane)

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
	right := m.renderPane(m.activePane == detailPane, rightWidth, contentHeight, "Detail")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", paneGap),
		right,
	)
}
