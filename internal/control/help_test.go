package control

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/huh/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tk "kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

func TestGroupedKeymapUsesEffectiveNavigationBindings(t *testing.T) {
	groups := KeymapGroups(locale.En, DefaultKeymap())
	byID := make(map[string]KeymapGroup)
	for _, group := range groups {
		require.NotEmpty(t, group.Bindings)
		assert.NotEmpty(t, group.Title)
		_, duplicate := byID[group.ID]
		assert.False(t, duplicate)
		byID[group.ID] = group
		for _, binding := range group.Bindings {
			assert.NotEmpty(t, binding.Keys())
			assert.NotEmpty(t, binding.Help().Desc)
		}
	}
	for _, id := range []string{GlobalKeys, LoginKeys, FilePickerKeys, ListKeys, SearchKeys, EntryFormKeys, ConfirmationKeys, RecoveryKeys, HelpKeys} {
		assert.Contains(t, byID, id)
	}
	actual := EntryTableKeyMap()
	assert.Equal(t, []string{"pgup"}, actual.PageUp.Keys())
	assert.Equal(t, []string{"pgdown"}, actual.PageDown.Keys())
	assert.Equal(t, []string{"ctrl+d"}, actual.HalfPageDown.Keys())
	for _, keys := range [][]string{actual.PageUp.Keys(), actual.PageDown.Keys(), actual.HalfPageDown.Keys(), table.DefaultKeyMap().GotoBottom.Keys()} {
		found := false
		for _, entry := range byID[ListKeys].Bindings {
			if assert.ObjectsAreEqual(keys, entry.Keys()) {
				found = true
			}
		}
		assert.True(t, found, "navigation binding missing from help")
	}
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Next.Keys(), byID[EntryFormKeys].Bindings[0].Keys())
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Prev.Keys(), byID[LoginKeys].Bindings[1].Keys())
	assert.Equal(t, huh.NewDefaultKeyMap().Input.Submit.Keys(), byID[EntryFormKeys].Bindings[2].Keys())
}

func TestFormKeyMapTranslatesHuhHintsWithoutChangingKeys(t *testing.T) {
	locale.Catalogs["xx"] = locale.Catalog{namespace.Help: {tk.FormNext: "järgmine"}}
	t.Cleanup(func() { delete(locale.Catalogs, "xx") })
	defaults := huh.NewDefaultKeyMap()
	keys := FormKeyMap("xx")
	assert.Equal(t, "järgmine", keys.Input.Next.Help().Desc)
	assert.Equal(t, "submit", keys.Input.Submit.Help().Desc, "missing translations use English")
	assert.Equal(t, defaults.Input.Next.Keys(), keys.Input.Next.Keys())
	assert.Equal(t, defaults.Input.Submit.Keys(), keys.Input.Submit.Keys())
}

func TestLabelFormatsBindingKeys(t *testing.T) {
	assert.Equal(t, "Ctrl+R", Label("ctrl+r"))
	assert.Equal(t, "Esc/?/F1", Label("esc", "?", "f1"))
	assert.Equal(t, "q", Label("q"), "bare letters must not suggest Shift")
	assert.Equal(t, "↑/↓/PgUp/PgDn", Label("up", "down", "pgup", "pgdown"))
}
