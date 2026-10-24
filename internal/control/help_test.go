package control

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupedKeymapUsesEffectiveNavigationBindings(t *testing.T) {
	groups := KeymapGroups()
	byTitle := make(map[string]KeymapGroup)
	for _, group := range groups {
		require.NotEmpty(t, group.Bindings)
		_, duplicate := byTitle[group.Title]
		assert.False(t, duplicate)
		byTitle[group.Title] = group
		for _, binding := range group.Bindings {
			assert.NotEmpty(t, binding.Keys())
			assert.NotEmpty(t, binding.Help().Desc)
		}
	}
	for _, title := range []string{GlobalKeys, LoginKeys, FilePickerKeys, ListKeys, SearchKeys, EntryFormKeys, ConfirmationKeys, RecoveryKeys, HelpKeys} {
		assert.Contains(t, byTitle, title)
	}
	actual := EntryTableKeyMap()
	assert.Equal(t, []string{"pgup"}, actual.PageUp.Keys())
	assert.Equal(t, []string{"pgdown"}, actual.PageDown.Keys())
	assert.Equal(t, []string{"ctrl+d"}, actual.HalfPageDown.Keys())
	for _, keys := range [][]string{actual.PageUp.Keys(), actual.PageDown.Keys(), actual.HalfPageDown.Keys(), table.DefaultKeyMap().GotoBottom.Keys()} {
		found := false
		for _, entry := range byTitle[ListKeys].Bindings {
			if assert.ObjectsAreEqual(keys, entry.Keys()) {
				found = true
			}
		}
		assert.True(t, found, "navigation binding missing from help")
	}
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Next.Keys(), byTitle[EntryFormKeys].Bindings[0].Keys())
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Prev.Keys(), byTitle[LoginKeys].Bindings[1].Keys())
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Submit.Keys(), byTitle[EntryFormKeys].Bindings[2].Keys())
}
