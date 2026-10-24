package control

import (
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	"charm.land/huh/v2"
	"kzree.com/keepy/internal/textprovider"
	tk "kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

// Stable keymap context IDs. Display headings are translated separately.
const (
	GlobalKeys       = "global"
	LoginKeys        = "login"
	ListKeys         = "list"
	SearchKeys       = "search"
	EntryFormKeys    = "entry-form"
	ConfirmationKeys = "confirmation"
	RecoveryKeys     = "recovery"
	HelpKeys         = "help"
	FilePickerKeys   = "file-picker"
)

type KeymapGroup struct {
	ID       string
	Title    string
	Bindings []HelpEntry
}

// HelpEntry is a displayed binding. Default is set only for a configurable
// action whose effective keys differ from its defaults.
type HelpEntry struct {
	key.Binding
	Default []string
}

var keyLabels = map[string]string{"up": "↑", "down": "↓", "pgup": "PgUp", "pgdown": "PgDn"}

// Label formats binding keys for display, e.g. "ctrl+r" as "Ctrl+R". Bare
// single characters stay as typed so "q" is not mistaken for Shift+Q.
func Label(keys ...string) string {
	labels := make([]string, len(keys))
	for i, k := range keys {
		if label, ok := keyLabels[k]; ok {
			labels[i] = label
			continue
		}
		parts := strings.Split(k, "+")
		for j, part := range parts {
			if part == "" || (utf8.RuneCountInString(part) == 1 && len(parts) == 1) {
				continue
			}
			parts[j] = strings.ToUpper(part[:1]) + part[1:]
		}
		labels[i] = strings.Join(parts, "+")
	}
	return strings.Join(labels, "/")
}

// Hint formats a compact "Keys: description" shortcut hint.
func Hint(description string, keys ...string) string {
	return Label(keys...) + ": " + description
}

// FormKeyMap localizes the help Huh generates for form inputs while keeping
// Huh's default keys.
func FormKeyMap(l locale.Locale) *huh.KeyMap {
	t := textprovider.New(l, namespace.Help)
	keys := huh.NewDefaultKeyMap()
	for _, b := range []struct {
		binding *key.Binding
		text    string
	}{
		{&keys.Quit, tk.FormQuit},
		{&keys.Input.Next, tk.FormNext},
		{&keys.Input.Prev, tk.FormBack},
		{&keys.Input.Submit, tk.FormSubmit},
		{&keys.Input.AcceptSuggestion, tk.FormComplete},
	} {
		b.binding.SetHelp(b.binding.Help().Key, t.Text(b.text))
	}
	return keys
}

func LoginPickerKeyMap() filepicker.KeyMap {
	keys := filepicker.DefaultKeyMap()
	keys.Back.SetKeys("h", "backspace", "left")
	return keys
}

func binding(description string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(strings.Join(keys, " / "), description))
}

// EntryTableKeyMap is shared by dispatch and discovery. Bare d/f are application
// actions, so table navigation must not advertise them as paging shortcuts.
func EntryTableKeyMap() table.KeyMap {
	keys := table.DefaultKeyMap()
	keys.PageUp.SetKeys("pgup")
	keys.PageUp.SetHelp("pgup", keys.PageUp.Help().Desc)
	keys.PageDown.SetKeys("pgdown")
	keys.PageDown.SetHelp("pgdown", keys.PageDown.Help().Desc)
	keys.HalfPageDown.SetKeys("ctrl+d")
	keys.HalfPageDown.SetHelp("ctrl+d", keys.HalfPageDown.Help().Desc)
	return keys
}

func HelpViewportKeyMap() viewport.KeyMap {
	keys := viewport.DefaultKeyMap()
	keys.PageUp.SetKeys("pgup")
	keys.PageUp.SetHelp("pgup", keys.PageUp.Help().Desc)
	keys.PageDown.SetKeys("pgdown")
	keys.PageDown.SetHelp("pgdown", keys.PageDown.Help().Desc)
	keys.HalfPageUp.SetEnabled(false)
	keys.HalfPageDown.SetEnabled(false)
	keys.Left.SetEnabled(false)
	keys.Right.SetEnabled(false)
	return keys
}

// KeymapGroups uses application constants and the actual table/form navigation
// maps. It describes conditional actions rather than implying every key is
// enabled in every mode (or while a vault operation is pending).
func KeymapGroups(l locale.Locale, keys Keymap) []KeymapGroup {
	t := textprovider.New(l, namespace.Help)
	bind := func(text string, keys ...string) HelpEntry { return HelpEntry{Binding: binding(t.Text(text), keys...)} }
	defaults := DefaultKeymap()
	action := func(text string, effective, initial key.Binding) HelpEntry {
		entry := bind(text, effective.Keys()...)
		if !slices.Equal(effective.Keys(), initial.Keys()) {
			entry.Default = initial.Keys()
		}
		return entry
	}
	form := huh.NewDefaultKeyMap().Input
	navigation := []HelpEntry{
		bind(tk.BindNextField, form.Next.Keys()...),
		bind(tk.BindPrevField, form.Prev.Keys()...),
		bind(tk.BindSubmitLast, form.Submit.Keys()...),
	}
	tableKeys := EntryTableKeyMap()
	help := HelpViewportKeyMap()
	picker := LoginPickerKeyMap()
	return []KeymapGroup{
		{GlobalKeys, t.Text(tk.GroupGlobal), []HelpEntry{
			action(tk.BindHelpAnywhere, keys.ShowHelpAnywhere, defaults.ShowHelpAnywhere),
			action(tk.BindHelp, keys.ShowHelp, defaults.ShowHelp),
			action(tk.BindQuit, keys.Quit, defaults.Quit),
			action(tk.BindQuitAlt, keys.QuitAlt, defaults.QuitAlt),
		}},
		{LoginKeys, t.Text(tk.GroupLogin), append(append([]HelpEntry{}, navigation...),
			action(tk.BindTogglePaths, keys.LoginTogglePaths, defaults.LoginTogglePaths),
			action(tk.BindBrowseDatabase, keys.LoginBrowseDatabase, defaults.LoginBrowseDatabase),
			action(tk.BindBrowseKeyFile, keys.LoginBrowseKeyFile, defaults.LoginBrowseKeyFile),
			action(tk.BindToggleKeyFile, keys.LoginToggleKeyFile, defaults.LoginToggleKeyFile))},
		{FilePickerKeys, t.Text(tk.GroupFilePicker), []HelpEntry{
			bind(tk.BindUp, picker.Up.Keys()...), bind(tk.BindDown, picker.Down.Keys()...),
			bind(tk.BindPageUp, picker.PageUp.Keys()...), bind(tk.BindPageDown, picker.PageDown.Keys()...),
			bind(tk.BindFirst, picker.GoToTop.Keys()...), bind(tk.BindLast, picker.GoToLast.Keys()...),
			bind(tk.BindParentDirectory, picker.Back.Keys()...),
			bind(tk.BindOpenOrSelect, "enter"), bind(tk.BindOpenDirectory, "l", "right"),
			bind(tk.BindCancelBrowsing, CloseEntryForm), action(tk.BindRetryDirectory, keys.Reload, defaults.Reload),
		}},
		{ListKeys, t.Text(tk.GroupList), []HelpEntry{
			action(tk.BindCopy, keys.CopyEntry, defaults.CopyEntry),
			action(tk.BindCreate, keys.NewEntry, defaults.NewEntry), action(tk.BindEdit, keys.EditEntry, defaults.EditEntry),
			action(tk.BindDelete, keys.DeleteEntry, defaults.DeleteEntry),
			action(tk.BindSearch, keys.Search, defaults.Search), action(tk.BindClearFilter, keys.ClearFilter, defaults.ClearFilter),
			bind(tk.BindUp, tableKeys.LineUp.Keys()...), bind(tk.BindDown, tableKeys.LineDown.Keys()...),
			bind(tk.BindPageUp, tableKeys.PageUp.Keys()...), bind(tk.BindPageDown, tableKeys.PageDown.Keys()...),
			bind(tk.BindHalfPageUp, tableKeys.HalfPageUp.Keys()...), bind(tk.BindHalfPageDown, tableKeys.HalfPageDown.Keys()...),
			bind(tk.BindFirst, tableKeys.GotoTop.Keys()...), bind(tk.BindLast, tableKeys.GotoBottom.Keys()...),
		}},
		{SearchKeys, t.Text(tk.GroupSearch), []HelpEntry{bind(tk.BindCloseSearch, "enter", "esc")}},
		{EntryFormKeys, t.Text(tk.GroupEntryForm), append(append([]HelpEntry{}, navigation...),
			action(tk.BindGeneratePassword, keys.GeneratePassword, defaults.GeneratePassword), bind(tk.BindCancelDraft, CloseEntryForm))},
		{ConfirmationKeys, t.Text(tk.GroupConfirmation), []HelpEntry{action(tk.BindConfirmDelete, keys.Confirm, defaults.Confirm), action(tk.BindCancelDelete, keys.Cancel, defaults.Cancel)}},
		{RecoveryKeys, t.Text(tk.GroupRecovery), []HelpEntry{action(tk.BindRecovery, keys.Reload, defaults.Reload)}},
		{HelpKeys, t.Text(tk.GroupHelp), []HelpEntry{
			bind(tk.BindCloseHelp, append([]string{CloseEntryForm}, append(keys.ShowHelp.Keys(), keys.ShowHelpAnywhere.Keys()...)...)...),
			bind(tk.BindUp, help.Up.Keys()...), bind(tk.BindDown, help.Down.Keys()...),
			bind(tk.BindPageUp, help.PageUp.Keys()...), bind(tk.BindPageDown, help.PageDown.Keys()...),
			bind(tk.BindFirstLastPage, "home", "end"),
		}},
	}
}
