package list

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
)

func TestResponsiveColumnsKeepRowSchemaAndFitAvailableWidth(t *testing.T) {
	entry := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "NeedleTitle", Username: "alice-user", URL: "https://example.test", Group: "PrivateGroup"}
	m := NewListModel()
	m.SetEntries([]service.VaultEntry{entry})
	for _, width := range []int{0, 1, 2, 3, 20, 33, 34, 59, 60, 79, 80, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m.SetListTableSize(width, 12)
			used, visible := 0, 0
			for _, column := range m.table.Columns() {
				if column.Width > 0 {
					visible++
					used += column.Width + 2*tableCellPadding(width)
				}
			}
			assert.LessOrEqual(t, used, width)
			if width > 0 {
				assert.Positive(t, m.table.Columns()[0].Width)
			}
			assert.Equal(t, []string{entry.Title, entry.URL, entry.Username, entry.Group}, []string(m.table.SelectedRow()))
			view := m.View(width, 12)
			if width == 0 {
				assert.Empty(t, view)
				return
			}
			assert.LessOrEqual(t, lipgloss.Width(view), width)
			assert.LessOrEqual(t, lipgloss.Height(view), 12)
			if width >= 20 {
				assert.Contains(t, view, "NeedleTitle")
			}
			switch {
			case width >= 80:
				assert.Equal(t, 4, visible)
				assert.Contains(t, view, "PrivateGroup")
			case width >= 60:
				assert.Equal(t, 3, visible)
				assert.NotContains(t, view, "PrivateGroup")
			case width >= 34:
				assert.Equal(t, 2, visible)
				assert.Contains(t, view, "alice-user")
				assert.NotContains(t, view, "https://")
			default:
				assert.Equal(t, 1, visible)
				assert.NotContains(t, view, "alice-user")
			}
		})
	}
}

func TestResizePreservesFilterAndScrolledSelection(t *testing.T) {
	m := NewListModel()
	entries := make([]service.VaultEntry, 30)
	for i := range entries {
		entries[i] = service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: fmt.Sprintf("Row%02d", i), Username: "matching-user"}
	}
	m.SetEntries(entries)
	m.FilterEntries("matching-user")
	m.SetListTableSize(120, 12)
	m.table.MoveDown(23)
	for _, size := range [][2]int{{20, 4}, {80, 10}, {35, 3}, {140, 20}} {
		m.SetListTableSize(size[0], size[1])
		require.NotNil(t, m.getEntryOnCursor())
		assert.Equal(t, entries[23].ID, m.getEntryOnCursor().ID)
		assert.Equal(t, "matching-user", m.filter)
		assert.Contains(t, m.table.View(), "Row23", "selected row must remain visible after reflow")
		assert.LessOrEqual(t, lipgloss.Height(m.View(size[0], size[1])), size[1])
	}
}

func TestFormUsesFullWidthBelowSidebarBreakpoint(t *testing.T) {
	m := NewListModel()
	m.SetEntries([]service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "TableMarker"}})
	m.SetListTableSize(130, 24)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n'}))
	assert.Contains(t, m.View(130, 24), "TableMarker")
	assert.Equal(t, 130, lipgloss.Width(m.View(130, 24)), "pane dimensions include their borders")
	for _, width := range []int{60, 40, 20, 1, 130} {
		m.SetListTableSize(width, 16)
		view := m.View(width, 16)
		assert.LessOrEqual(t, lipgloss.Width(view), width)
		assert.LessOrEqual(t, lipgloss.Height(view), 16)
		if width < minListPaneWidth+paneGap+minFormPaneWidth {
			assert.NotContains(t, view, "TableMarker")
		} else {
			assert.Contains(t, view, "TableMarker")
		}
	}
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m.SetListTableSize(40, 16)
	assert.Contains(t, m.View(40, 16), "TableMarker")
	assert.Equal(t, 40, m.table.Width())
}

func TestTinySearchAndConfirmationStayBoundedAndKeepState(t *testing.T) {
	m := NewListModel()
	id := gokeepasslib.NewUUID()
	m.SetEntries([]service.VaultEntry{{ID: id, Title: "Marker"}})
	m.SetListTableSize(30, 5)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'f'}))
	for _, char := range "Marker" {
		m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	for _, size := range [][2]int{{8, 2}, {1, 1}, {30, 5}} {
		m.SetListTableSize(size[0], size[1])
		assert.Equal(t, "Marker", m.search.Value())
		assert.LessOrEqual(t, lipgloss.Width(m.View(size[0], size[1])), size[0])
		assert.LessOrEqual(t, lipgloss.Height(m.View(size[0], size[1])), size[1])
	}
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	require.NotNil(t, cmd)
	m, _ = m.Update(cmd())
	m.SetListTableSize(14, 3)
	assert.Contains(t, m.View(14, 3), "y/n")
	assert.LessOrEqual(t, lipgloss.Height(m.View(14, 3)), 3)
	m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n'}))
	require.NotNil(t, cmd)
	m, _ = m.Update(cmd())
	assert.False(t, m.showDeletePrompt)
	assert.Equal(t, id, m.getEntryOnCursor().ID)
}
