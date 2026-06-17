// Package list implements the screen for listing vault entries and showing details of a selected entry.
package list

import (
	"strings"
	"time"

	"golang.design/x/clipboard"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/util"
)

type pane int

const copyFlashDuration = 200 * time.Millisecond

const (
	listPane pane = iota
	createPane
)

const paneGap = 1

type ListModel struct {
	showSidePane bool
	activePane   pane
	table        table.Model
	copyFlashID  int
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
	case copyFlashDoneMsg:
		if msg.id == m.copyFlashID {
			setSelectedStyle(&m.table, normalSelectedStyle())
		}
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
				row := m.table.SelectedRow()
				if len(row) <= 3 {
					break
				}
				pwd := row[4] // TODO: this is very hacky, need to find a better way to identify entry password

				clipboard.Write(clipboard.FmtText, []byte(pwd))
				m.copyFlashID++
				setSelectedStyle(&m.table, copiedSelectedStyle())
				return m, clearCopyFlashCmd(m.copyFlashID)
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
			entry.Group,
			entry.Password,
		})
	}
	m.table.SetRows(rows)
}

func clearCopyFlashCmd(id int) tea.Cmd {
	return tea.Tick(copyFlashDuration, func(time.Time) tea.Msg {
		return copyFlashDoneMsg{id: id}
	})
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
