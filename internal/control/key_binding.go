// Package control holds the key binding definitions for the application.
package control

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/huh/v2"
)

// CloseEntryForm is fixed: Esc also closes search, help, and pickers.
const CloseEntryForm = "esc"

// Keymap holds the effective, validated application bindings. Dispatch, help,
// and shortcut hints must all read the same Keymap.
type Keymap struct {
	Quit                key.Binding
	QuitAlt             key.Binding
	ShowHelp            key.Binding
	ShowHelpAnywhere    key.Binding
	LoginTogglePaths    key.Binding
	LoginBrowseDatabase key.Binding
	LoginBrowseKeyFile  key.Binding
	LoginToggleKeyFile  key.Binding
	CopyEntry           key.Binding
	NewEntry            key.Binding
	EditEntry           key.Binding
	DeleteEntry         key.Binding
	Search              key.Binding
	ClearFilter         key.Binding
	GeneratePassword    key.Binding
	Reload              key.Binding
	Confirm             key.Binding
	Cancel              key.Binding
}

var (
	allContexts          = []string{LoginKeys, FilePickerKeys, ListKeys, SearchKeys, EntryFormKeys, ConfirmationKeys, RecoveryKeys, HelpKeys}
	textInputContexts    = []string{LoginKeys, SearchKeys, EntryFormKeys}
	outsideTextInputs    = []string{FilePickerKeys, ListKeys, ConfirmationKeys, RecoveryKeys, HelpKeys}
	reloadContexts       = []string{FilePickerKeys, ListKeys, SearchKeys, EntryFormKeys, RecoveryKeys}
	quitAltContexts      = []string{ListKeys, ConfirmationKeys, HelpKeys}
	loginContexts        = []string{LoginKeys}
	listContexts         = []string{ListKeys}
	entryFormContexts    = []string{EntryFormKeys}
	confirmationContexts = []string{ConfirmationKeys}
)

// actionSpec names a configurable action (its [keybindings] config key) and
// every context in which dispatch accepts it. Conflict validation relies on
// these contexts matching the key handlers.
type actionSpec struct {
	name     string
	defaults []string
	contexts []string
	field    func(*Keymap) *key.Binding
}

var actionSpecs = []actionSpec{
	{"quit", []string{"ctrl+c"}, allContexts, func(k *Keymap) *key.Binding { return &k.Quit }},
	{"quit_alt", []string{"q"}, quitAltContexts, func(k *Keymap) *key.Binding { return &k.QuitAlt }},
	{"show_help", []string{"?"}, outsideTextInputs, func(k *Keymap) *key.Binding { return &k.ShowHelp }},
	{"show_help_anywhere", []string{"f1"}, allContexts, func(k *Keymap) *key.Binding { return &k.ShowHelpAnywhere }},
	{"login_toggle_paths", []string{"ctrl+t"}, loginContexts, func(k *Keymap) *key.Binding { return &k.LoginTogglePaths }},
	{"login_browse_database", []string{"f2"}, loginContexts, func(k *Keymap) *key.Binding { return &k.LoginBrowseDatabase }},
	{"login_browse_key_file", []string{"f3"}, loginContexts, func(k *Keymap) *key.Binding { return &k.LoginBrowseKeyFile }},
	{"login_toggle_key_file", []string{"f4"}, loginContexts, func(k *Keymap) *key.Binding { return &k.LoginToggleKeyFile }},
	{"copy_entry", []string{"c"}, listContexts, func(k *Keymap) *key.Binding { return &k.CopyEntry }},
	{"new_entry", []string{"n"}, listContexts, func(k *Keymap) *key.Binding { return &k.NewEntry }},
	{"edit_entry", []string{"e"}, listContexts, func(k *Keymap) *key.Binding { return &k.EditEntry }},
	{"delete_entry", []string{"d"}, listContexts, func(k *Keymap) *key.Binding { return &k.DeleteEntry }},
	{"search", []string{"f"}, listContexts, func(k *Keymap) *key.Binding { return &k.Search }},
	{"clear_filter", []string{"x"}, listContexts, func(k *Keymap) *key.Binding { return &k.ClearFilter }},
	{"generate_password", []string{"ctrl+g"}, entryFormContexts, func(k *Keymap) *key.Binding { return &k.GeneratePassword }},
	{"reload", []string{"ctrl+r"}, reloadContexts, func(k *Keymap) *key.Binding { return &k.Reload }},
	{"confirm", []string{"y"}, confirmationContexts, func(k *Keymap) *key.Binding { return &k.Confirm }},
	{"cancel", []string{"n"}, confirmationContexts, func(k *Keymap) *key.Binding { return &k.Cancel }},
}

func DefaultKeymap() Keymap {
	var keys Keymap
	for _, spec := range actionSpecs {
		*spec.field(&keys) = key.NewBinding(key.WithKeys(spec.defaults...))
	}
	return keys
}

// ResolveKeymap applies [keybindings] overrides to the defaults. Any invalid
// override rejects all overrides, so a partial keymap can never introduce an
// ambiguous action or remove access to help or recovery.
func ResolveKeymap(overrides map[string][]string) (Keymap, error) {
	keys := DefaultKeymap()
	var problems []error
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		i := slices.IndexFunc(actionSpecs, func(spec actionSpec) bool { return spec.name == name })
		if i < 0 {
			problems = append(problems, fmt.Errorf("unknown action %q", name))
			continue
		}
		if len(overrides[name]) == 0 {
			problems = append(problems, fmt.Errorf("%s: at least one key is required", name))
			continue
		}
		var normalized []string
		for _, k := range overrides[name] {
			n, err := normalizeKey(k)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", name, err))
			} else if !slices.Contains(normalized, n) {
				normalized = append(normalized, n)
			}
		}
		*actionSpecs[i].field(&keys) = key.NewBinding(key.WithKeys(normalized...))
	}
	if len(problems) == 0 {
		problems = validateKeymap(keys)
	}
	if len(problems) > 0 {
		return DefaultKeymap(), errors.Join(problems...)
	}
	return keys, nil
}

func validateKeymap(keys Keymap) []error {
	var problems []error
	for _, context := range allContexts {
		owners := make(map[string]string)
		for _, k := range reservedKeys(context) {
			owners[k] = "a fixed key"
		}
		for _, spec := range actionSpecs {
			if !slices.Contains(spec.contexts, context) {
				continue
			}
			for _, k := range spec.field(&keys).Keys() {
				if owner, taken := owners[k]; taken {
					problems = append(problems, fmt.Errorf("%s: %q is already %s in the %s context", spec.name, k, owner, context))
					continue
				}
				owners[k] = "bound to " + spec.name
				if slices.Contains(textInputContexts, context) && printable(k) {
					problems = append(problems, fmt.Errorf("%s: %q would type text in the %s context", spec.name, k, context))
				}
			}
		}
	}
	return problems
}

// reservedKeys lists fixed component and navigation keys handled in a context.
func reservedKeys(context string) []string {
	var bindings []key.Binding
	var fixed []string
	text := textinput.DefaultKeyMap()
	textBindings := []key.Binding{
		text.CharacterForward, text.CharacterBackward, text.WordForward, text.WordBackward,
		text.DeleteWordBackward, text.DeleteWordForward, text.DeleteAfterCursor, text.DeleteBeforeCursor,
		text.DeleteCharacterBackward, text.DeleteCharacterForward, text.LineStart, text.LineEnd,
		text.Paste, text.AcceptSuggestion, text.NextSuggestion, text.PrevSuggestion,
	}
	form := huh.NewDefaultKeyMap().Input
	formBindings := []key.Binding{form.Next, form.Prev, form.Submit, form.AcceptSuggestion}
	switch context {
	case LoginKeys:
		bindings = append(textBindings, formBindings...)
	case EntryFormKeys:
		bindings = append(textBindings, formBindings...)
		fixed = []string{CloseEntryForm}
	case SearchKeys:
		bindings = textBindings
		fixed = []string{"enter", CloseEntryForm}
	case FilePickerKeys:
		p := LoginPickerKeyMap()
		bindings = []key.Binding{p.GoToTop, p.GoToLast, p.Down, p.Up, p.PageUp, p.PageDown, p.Back, p.Open, p.Select}
		fixed = []string{CloseEntryForm, "enter", "l", "right"}
	case ListKeys:
		t := EntryTableKeyMap()
		bindings = []key.Binding{t.LineUp, t.LineDown, t.PageUp, t.PageDown, t.HalfPageUp, t.HalfPageDown, t.GotoTop, t.GotoBottom}
		fixed = []string{CloseEntryForm}
	case HelpKeys:
		v := HelpViewportKeyMap()
		bindings = []key.Binding{v.Up, v.Down, v.PageUp, v.PageDown}
		fixed = []string{"home", "end", CloseEntryForm}
	}
	for _, b := range bindings {
		fixed = append(fixed, b.Keys()...)
	}
	return fixed
}

var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

var namedKeys = func() map[string]bool {
	names := map[string]bool{}
	for _, n := range strings.Fields("enter tab backspace esc space up down left right begin find insert delete select pgup pgdown home end") {
		names[n] = true
	}
	for i := 1; i <= 24; i++ {
		names[fmt.Sprintf("f%d", i)] = true
	}
	return names
}()

// normalizeKey converts a configured key to the form tea.KeyPressMsg.String()
// reports: lowercase canonical-order modifiers and named keys. Bare characters
// keep their case because Bubble Tea reports shifted text (e.g. "A", "?").
func normalizeKey(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("empty key")
	}
	base, mods := s, []string{}
	if i := strings.LastIndex(s[:len(s)-1], "+"); i >= 0 {
		base = s[i+1:]
		for _, m := range strings.Split(s[:i], "+") {
			m = strings.ToLower(m)
			if !slices.Contains(modifierOrder, m) || slices.Contains(mods, m) {
				return "", fmt.Errorf("invalid modifier in key %q", raw)
			}
			mods = append(mods, m)
		}
	}
	if lower := strings.ToLower(base); namedKeys[lower] {
		base = lower
	} else if utf8.RuneCountInString(base) != 1 || base == " " {
		return "", fmt.Errorf("unknown key %q", raw)
	} else if len(mods) > 0 {
		if slices.Equal(mods, []string{"shift"}) {
			return "", fmt.Errorf("key %q: use the shifted character instead, e.g. \"A\" or \"?\"", raw)
		}
		base = strings.ToLower(base)
	}
	slices.SortFunc(mods, func(a, b string) int { return slices.Index(modifierOrder, a) - slices.Index(modifierOrder, b) })
	return strings.Join(append(mods, base), "+"), nil
}

// printable reports keys that a focused text input would insert as text.
func printable(k string) bool {
	return k == "space" || (utf8.RuneCountInString(k) == 1)
}
