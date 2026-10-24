package list

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
)

func createEntryTable(t *textprovider.TextProvider) table.Model {
	columns := entryTableColumns(t, 42)

	rows := []table.Row{}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(1),
		table.WithWidth(42),
	)
	tbl.KeyMap = control.EntryTableKeyMap()

	setSelectedStyle(&tbl, normalSelectedStyle())

	return tbl
}

func setSelectedStyle(t *table.Model, selected lipgloss.Style) {
	s := table.DefaultStyles()
	padding := tableCellPadding(t.Width())
	s.Header = s.Header.Padding(0, padding)
	s.Cell = s.Cell.Padding(0, padding)
	s.Selected = selected
	t.SetStyles(s)
}

func tableCellPadding(width int) int {
	if width < 3 {
		return 0
	}
	return 1
}

// Bubbles' SetCursor does not scroll to reveal a row. Use navigation when
// restoring selection after either a data refresh or a viewport resize.
func (m *ListModel) restoreTableCursor(cursor int) {
	m.table.GotoTop()
	if len(m.filteredEntries) > 0 {
		m.table.MoveDown(max(0, cursor))
	}
}

// Keep the four-column row schema stable. Bubbles omits columns with width zero,
// so hiding a column never shifts a username or group into the wrong cell.
func entryTableColumns(t *textprovider.TextProvider, width int) []table.Column {
	columns := []table.Column{{Title: t.Text(key.ColumnTitle)}, {Title: t.Text(key.ColumnURL)}, {Title: t.Text(key.ColumnEmail)}, {Title: t.Text(key.ColumnGroup)}}
	visible := []int{0}
	switch {
	case width >= 80:
		visible = []int{0, 1, 2, 3}
	case width >= 60:
		visible = []int{0, 1, 2}
	case width >= 34:
		visible = []int{0, 2}
	}
	content := max(0, width-2*tableCellPadding(width)*len(visible))
	// Title receives the remainder and extra space; secondary columns get a
	// smaller equal share. This keeps titles readable at every breakpoint.
	secondary := content / (len(visible) + 1)
	for _, index := range visible[1:] {
		columns[index].Width = secondary
	}
	columns[0].Width = content - secondary*(len(visible)-1)
	return columns
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
