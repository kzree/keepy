package control

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/textprovider/locale"
)

func TestDefaultKeymapIsValidAndUsedWithoutOverrides(t *testing.T) {
	assert.Empty(t, validateKeymap(DefaultKeymap()))
	for _, overrides := range []map[string][]string{nil, {}} {
		keys, err := ResolveKeymap(overrides)
		require.NoError(t, err)
		assert.Equal(t, DefaultKeymap(), keys)
	}
}

func TestResolveKeymapAppliesNormalizedOverridesToDispatchAndHelp(t *testing.T) {
	keys, err := ResolveKeymap(map[string][]string{
		"copy_entry":         {" Ctrl+Y ", "C", "ctrl+y"},
		"show_help_anywhere": {"F12"},
		"quit":               {"shift+ctrl+q"},
		// Disjoint contexts may share a key: cancel is only active in the
		// confirmation prompt, and reload is never active in the login form.
		"cancel": {"x"},
		"reload": {"ctrl+t"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"ctrl+y", "C"}, keys.CopyEntry.Keys())
	assert.Equal(t, []string{"f12"}, keys.ShowHelpAnywhere.Keys())
	assert.Equal(t, []string{"ctrl+shift+q"}, keys.Quit.Keys())
	assert.Equal(t, DefaultKeymap().NewEntry, keys.NewEntry, "unlisted actions keep defaults")

	var listKeys []string
	for _, group := range KeymapGroups(locale.En, keys) {
		if group.ID == ListKeys {
			listKeys = group.Bindings[0].Keys()
		}
	}
	assert.Equal(t, keys.CopyEntry.Keys(), listKeys, "help shows the effective binding")
}

func TestResolveKeymapRejectsInvalidOverridesAndKeepsDefaults(t *testing.T) {
	for name, overrides := range map[string]map[string][]string{
		"unknown action":                {"copy": {"y"}},
		"empty key list":                {"reload": {}},
		"blank key":                     {"search": {" "}},
		"unknown key name":              {"search": {"slash"}},
		"missing base key":              {"search": {"ctrl+"}},
		"invalid modifier":              {"search": {"control+s"}},
		"repeated modifier":             {"search": {"ctrl+ctrl+s"}},
		"shift with a character":        {"search": {"shift+s"}},
		"same context conflict":         {"copy_entry": {"n"}},
		"two overrides conflict":        {"copy_entry": {"ctrl+y"}, "edit_entry": {"ctrl+y"}},
		"table navigation conflict":     {"search": {"j"}},
		"help shadowed in list":         {"clear_filter": {"?"}},
		"quit shadowed in confirmation": {"confirm": {"q"}},
		"text typed in form":            {"generate_password": {"g"}},
		"help anywhere types text":      {"show_help_anywhere": {"h"}},
		"text editing key in login":     {"login_toggle_paths": {"ctrl+a"}},
		"fixed escape key":              {"reload": {"esc"}},
	} {
		t.Run(name, func(t *testing.T) {
			keys, err := ResolveKeymap(overrides)
			require.Error(t, err)
			for action := range overrides {
				assert.Contains(t, err.Error(), action)
			}
			assert.Equal(t, DefaultKeymap(), keys)
		})
	}
}
