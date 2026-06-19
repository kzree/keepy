package list

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func createEntryTable() table.Model {
	columns := []table.Column{
		{Title: "Title", Width: 20},
		{Title: "URL", Width: 20},
		{Title: "Email", Width: 20},
		{Title: "Group", Width: 20},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(1),
		table.WithWidth(42),
	)

	setSelectedStyle(&t, normalSelectedStyle())

	return t
}

func setSelectedStyle(t *table.Model, selected lipgloss.Style) {
	s := table.DefaultStyles()
	s.Selected = selected
	t.SetStyles(s)
}

func normalSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.BrightBlue).
		Bold(true)
}

func copiedSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Black).
		Background(lipgloss.BrightBlue).
		Bold(true)
}
