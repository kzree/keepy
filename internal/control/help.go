package control

import (
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	"charm.land/huh/v2"
)

const (
	GlobalKeys       = "Global"
	LoginKeys        = "Login"
	ListKeys         = "Entry list"
	SearchKeys       = "Search"
	EntryFormKeys    = "Entry form"
	ConfirmationKeys = "Delete confirmation"
	RecoveryKeys     = "Error recovery"
	HelpKeys         = "Help"
	FilePickerKeys   = "Login file picker"
)

type KeymapGroup struct {
	Title    string
	Bindings []key.Binding
}

func LoginPickerKeyMap() filepicker.KeyMap {
	keys := filepicker.DefaultKeyMap()
	keys.Back.SetKeys("h", "backspace", "left")
	keys.Back.SetHelp("h / backspace / left", "parent directory")
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
func KeymapGroups() []KeymapGroup {
	form := huh.NewDefaultKeyMap().Input
	navigation := []key.Binding{
		binding("next field (before the last field)", form.Next.Keys()...),
		binding("previous field", form.Prev.Keys()...),
		binding("submit on the last field", form.Submit.Keys()...),
	}
	tableKeys := EntryTableKeyMap()
	help := HelpViewportKeyMap()
	picker := LoginPickerKeyMap()
	return []KeymapGroup{
		{GlobalKeys, []key.Binding{
			binding("open help everywhere", ShowHelpAnywhere),
			binding("open help outside text inputs", ShowHelp),
			binding("quit; waits for accepted vault work", QuitApplication),
			binding("quit from list/help, outside text inputs; waits for work", QuitApplicationAlt),
		}},
		{LoginKeys, append(append([]key.Binding{}, navigation...),
			binding("toggle manual path fields", LoginToggle),
			binding("browse for a database", LoginBrowseDatabase),
			binding("browse for a key file; enable it on selection", LoginBrowseKeyFile),
			binding("enable/disable optional key file (retains path)", LoginToggleKeyFile))},
		{FilePickerKeys, []key.Binding{
			picker.Up, picker.Down, picker.PageUp, picker.PageDown,
			picker.GoToTop, picker.GoToLast, picker.Back,
			binding("open directory / select file", "enter"), binding("open directory", "l", "right"),
			binding("cancel browsing; retain form values", CloseEntryForm), binding("retry reading directory", ReloadVault),
		}},
		{ListKeys, []key.Binding{
			binding("copy selected password (clipboard must be ready)", CopyEntry),
			binding("create entry", NewEntry), binding("edit selected entry", EditEntry),
			binding("confirm deletion of selected entry", DeleteEntity),
			binding("search entries", Search), binding("clear filter", ClearFilter),
			tableKeys.LineUp, tableKeys.LineDown, tableKeys.PageUp, tableKeys.PageDown,
			tableKeys.HalfPageUp, tableKeys.HalfPageDown, tableKeys.GotoTop, tableKeys.GotoBottom,
		}},
		{SearchKeys, []key.Binding{binding("close search; keep filter", "enter", "esc")}},
		{EntryFormKeys, append(append([]key.Binding{}, navigation...),
			binding("generate a 24-character password", GeneratePassword), binding("cancel and discard draft", CloseEntryForm))},
		{ConfirmationKeys, []key.Binding{binding("confirm deletion", Confirm), binding("cancel deletion", Cancel)}},
		{RecoveryKeys, []key.Binding{binding("retry config loading at startup / reload vault after conflict; preserves draft for review", ReloadVault)}},
		{HelpKeys, []key.Binding{
			binding("return to the underlying view", "esc", ShowHelp, ShowHelpAnywhere),
			help.Up, help.Down, help.PageUp, help.PageDown,
			binding("first / last help page", "home", "end"),
		}},
	}
}
