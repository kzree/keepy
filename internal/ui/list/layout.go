package list

import (
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/style"
)

const (
	paneGap          = 1
	minListPaneWidth = 44
	minFormPaneWidth = 42
)

func (m ListModel) paneWidths(width int) (listWidth, formWidth int) {
	if !m.showSidePane {
		return max(0, width), 0
	}
	if width < minListPaneWidth+paneGap+minFormPaneWidth {
		return 0, max(0, width)
	}
	formWidth = max(minFormPaneWidth, width/3)
	return width - paneGap - formWidth, formWidth
}

func (m ListModel) formFrame(width, height int) (int, int) {
	if width < 4 || height < 4 {
		return 0, 0
	}
	return style.GetPaneStyle(m.activePane == createPane).GetFrameSize()
}

func (m *ListModel) SetListTableSize(width, height int) {
	width, height = max(0, width), max(0, height)
	if m.width == width && m.height == height {
		return
	}
	m.width, m.height = width, height
	m.resizeTable()
}

func (m ListModel) promptView() string {
	var prompt string
	if m.showSearch {
		prompt = m.search.View()
	} else if m.showDeletePrompt {
		prompt = m.confirm.View()
	}
	if prompt == "" {
		return ""
	}
	return boundedView(lipgloss.NewStyle().Width(max(1, m.width)).Render(prompt), m.width, m.height)
}

func (m *ListModel) resizeTable() {
	listWidth, formWidth := m.paneWidths(m.width)
	if listWidth == 0 {
		listWidth = m.width
	} // retain the hidden table's state
	cursor := m.table.Cursor()
	// Hide columns before rebuilding rows, keeping the existing four-cell schema.
	m.table.SetColumns(entryTableColumns(m.t, listWidth))
	m.table.SetWidth(max(1, listWidth))
	selected := normalSelectedStyle()
	if m.copyFlashActive {
		selected = copiedSelectedStyle()
	}
	setSelectedStyle(&m.table, selected)
	m.search.SetWidth(m.width)
	m.table.SetHeight(max(1, m.height-lipgloss.Height(m.promptView())))
	m.restoreTableCursor(cursor)
	if m.showSidePane {
		frameW, frameH := m.formFrame(formWidth, m.height)
		m.entryForm.SetSize(max(0, formWidth-frameW), max(0, m.height-frameH))
	}
}

func (m ListModel) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	listWidth, formWidth := m.paneWidths(width)
	var form string
	if m.showSidePane {
		form = m.entryForm.View()
		frameW, frameH := m.formFrame(formWidth, height)
		if frameW != 0 || frameH != 0 {
			// Lip Gloss dimensions include borders. Only the inner form budget
			// subtracts the frame; subtracting here as well would double-count it.
			form = style.GetPaneStyle(m.activePane == createPane).
				Width(formWidth).Height(height).Render(form)
		}
		form = boundedView(form, formWidth, height)
		if listWidth == 0 {
			return form
		}
	}
	prompt := m.promptView()
	left := boundedView(m.table.View(), listWidth, max(0, height-lipgloss.Height(prompt)))
	if prompt != "" {
		if left == "" {
			left = prompt
		} else if m.showSearch {
			left = lipgloss.JoinVertical(lipgloss.Left, prompt, left)
		} else {
			left = lipgloss.JoinVertical(lipgloss.Left, left, prompt)
		}
	}
	if !m.showSidePane {
		return boundedView(left, width, height)
	}
	return boundedView(lipgloss.JoinHorizontal(lipgloss.Top, left, " ", form), width, height)
}

func boundedView(body string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(body)
}
