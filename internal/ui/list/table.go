package list

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
)

func createEntryTable(t *textprovider.TextProvider) table.Model {
	columns := []table.Column{
		{Title: t.Text(key.ColumnTitle), Width: 20},
		{Title: t.Text(key.ColumnURL), Width: 20},
		{Title: t.Text(key.ColumnEmail), Width: 20},
		{Title: t.Text(key.ColumnGroup), Width: 20},
	}

	rows := []table.Row{}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(1),
		table.WithWidth(42),
	)
	keys := table.DefaultKeyMap()
	keys.PageUp.SetKeys("pgup")
	keys.PageDown.SetKeys("pgdown")
	tbl.KeyMap = keys

	setSelectedStyle(&tbl, normalSelectedStyle())

	return tbl
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
