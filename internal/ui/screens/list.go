package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
}

func NewListModel() ListModel {
	return ListModel{
		activePane: listPane,
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
		}
	}
	return m, nil
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

func (m ListModel) View(contentWidth, contentHeight int) string {
	leftWidth, rightWidth := m.getPaneWidths(contentWidth)

	left := m.renderPane(m.activePane == listPane, leftWidth, contentHeight, "List")
	right := m.renderPane(m.activePane == detailPane, rightWidth, contentHeight, "Detail")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", paneGap),
		right,
	)
}
