package list

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
)

func TestRefreshPreservesQueryAndSelectedEntry(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	a := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Match A"}
	b := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Match B"}
	other := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Other"}
	m.SetEntries([]service.VaultEntry{a, other, b})
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'f'}))
	for _, char := range "match" {
		m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m.table.SetCursor(1)
	b.Title = "Match B edited"
	m.SetEntries([]service.VaultEntry{b, other, a})
	assert.Equal(t, "match", m.search.Value())
	assert.Equal(t, []service.VaultEntry{b, a}, m.filteredEntries)
	require.NotNil(t, m.getEntryOnCursor())
	assert.Equal(t, b.ID, m.getEntryOnCursor().ID)
	assert.Equal(t, 0, m.table.Cursor())
}

func TestRestoredSelectionIsVisibleAfterClearingAndReordering(t *testing.T) {
	for _, action := range []string{"clear filter", "reorder refresh"} {
		t.Run(action, func(t *testing.T) {
			m := NewListModel(locale.En, control.DefaultKeymap())
			m.SetListTableSize(100, 6)
			entries := make([]service.VaultEntry, 20)
			for i := range entries {
				entries[i] = service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: fmt.Sprintf("Entry %02d", i)}
			}
			entries[15].Title = "Selected target"
			selected := entries[15]
			if action == "clear filter" {
				m.SetEntries(entries)
				m.FilterEntries("Selected target")
				m.FilterEntries("")
			} else {
				first := append([]service.VaultEntry{selected}, entries[:15]...)
				first = append(first, entries[16:]...)
				m.SetEntries(first)
				m.SetEntries(entries)
			}
			require.NotNil(t, m.getEntryOnCursor())
			assert.Equal(t, selected.ID, m.getEntryOnCursor().ID)
			assert.Equal(t, 15, m.table.Cursor())
			assert.Contains(t, m.table.View(), "Selected target")
		})
	}
}

func TestEditedSelectionFallsBackWithinMatchingResults(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	entries := []service.VaultEntry{
		{ID: gokeepasslib.NewUUID(), Title: "Match A"},
		{ID: gokeepasslib.NewUUID(), Title: "Match B"},
		{ID: gokeepasslib.NewUUID(), Title: "Match C"},
	}
	m.SetEntries(entries)
	m.FilterEntries("match")
	m.table.MoveDown(1)
	updated := append([]service.VaultEntry{}, entries...)
	updated[1].Title = "Edited elsewhere"
	m.SetEntries(updated)
	require.Len(t, m.filteredEntries, 2)
	assert.Equal(t, entries[2].ID, m.getEntryOnCursor().ID)
	assert.Equal(t, 1, m.table.Cursor())
}

func TestRefreshFallsBackToNearbyRow(t *testing.T) {
	for _, removedRow := range []int{0, 1, 2} {
		m := NewListModel(locale.En, control.DefaultKeymap())
		entries := []service.VaultEntry{
			{ID: gokeepasslib.NewUUID(), Title: "Match A"},
			{ID: gokeepasslib.NewUUID(), Title: "Match B"},
			{ID: gokeepasslib.NewUUID(), Title: "Match C"},
		}
		m.SetEntries(entries)
		m.FilterEntries("match")
		m.table.SetCursor(removedRow)
		remaining := append([]service.VaultEntry{}, entries[:removedRow]...)
		remaining = append(remaining, entries[removedRow+1:]...)
		m.SetEntries(remaining)
		assert.Equal(t, min(removedRow, 1), m.table.Cursor())
		require.NotNil(t, m.getEntryOnCursor())
		assert.Equal(t, remaining[min(removedRow, 1)].ID, m.getEntryOnCursor().ID)
	}
}

func TestFilteringAndClearingPreserveSelectionAndHandleNoMatches(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	a := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Other"}
	b := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Match"}
	m.SetEntries([]service.VaultEntry{a, b})
	m.table.SetCursor(1)
	m.FilterEntries("match")
	assert.Equal(t, b.ID, m.getEntryOnCursor().ID)
	m.FilterEntries("")
	assert.Equal(t, 1, m.table.Cursor())
	assert.Equal(t, b.ID, m.getEntryOnCursor().ID)
	b.Title = "Renamed elsewhere"
	m.FilterEntries("match")
	m.SetEntries([]service.VaultEntry{a, b})
	assert.Empty(t, m.filteredEntries)
	assert.Nil(t, m.getEntryOnCursor())
	// Use a query with genuinely no matches, then refresh back to matching data.
	m.FilterEntries("missing")
	m.SetEntries([]service.VaultEntry{a, b})
	assert.Empty(t, m.filteredEntries)
	assert.Nil(t, m.getEntryOnCursor())
	b.Title = "Missing now present"
	m.SetEntries([]service.VaultEntry{a, b})
	require.Len(t, m.filteredEntries, 1)
	assert.Equal(t, b.ID, m.getEntryOnCursor().ID)
}
