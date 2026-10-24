package ui

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/util/testutil"
)

// Wide and long, so translated text exercises layout budgets. Accented vowels
// expose untranslated English while leaving format verbs (%s, %v) intact.
const pseudoPrefix = "全角ŴŴŴŴŴŴ "

var pseudoVowels = strings.NewReplacer("a", "á", "e", "é", "i", "ï", "o", "ö", "u", "ü")

var rawTextKey = regexp.MustCompile(`\b[A-Z]{2,}(_[A-Z]+)+\b`)

func installPseudoLocale(t *testing.T) locale.Locale {
	t.Helper()
	const pseudo = "zz-pseudo"
	catalog := locale.Catalog{}
	for ns, values := range locale.EnMap {
		translated := make(map[string]string, len(values))
		for k, v := range values {
			translated[k] = pseudoPrefix + pseudoVowels.Replace(v)
		}
		catalog[ns] = translated
	}
	locale.Catalogs[pseudo] = catalog
	t.Cleanup(func() { delete(locale.Catalogs, pseudo) })
	return pseudo
}

// visitLocalizedScreens starts from saved config and returns each screen a user
// can reach: login, pickers, help, notices, list, form, prompts, and recovery.
func visitLocalizedScreens(t *testing.T, preference string, width, height int) []string {
	t.Helper()
	testutil.TempHome(t)
	require.NoError(t, service.SaveConfig(&service.Config{Preferences: service.Preferences{Locale: preference}, Credentials: service.Credentials{DBPath: "vault.kdbx"}}))
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Entry"}}, passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r := NewRootModel()
	r.db = vault
	r.initClipboard = func() error { return errors.New("clipboardfailure") }
	drainRoot(t, &r, r.Init())
	sendRoot(t, &r, tea.WindowSizeMsg{Width: width, Height: height})
	var views []string
	capture := func(keys ...tea.Key) {
		t.Helper()
		for _, k := range keys {
			sendRoot(t, &r, tea.KeyPressMsg(k))
		}
		views = append(views, r.View().Content)
	}
	escape := tea.Key{Code: tea.KeyEscape}
	capture()
	capture(tea.Key{Code: tea.KeyF2})
	capture(escape, tea.Key{Code: tea.KeyF1})
	capture(escape, tea.Key{Code: tea.KeyF4})
	// Enabling the key file reveals the path fields; submit from the database path.
	capture(tea.Key{Code: tea.KeyF4}, tea.Key{Code: 's', Text: "s"}, tea.Key{Code: tea.KeyEnter}, tea.Key{Code: tea.KeyEnter})
	require.Equal(t, screenList, r.activeView)
	capture(tea.Key{Code: 'c'})
	capture(tea.Key{Code: 'n'})
	capture(tea.Key{Code: tea.KeyF1})
	capture(escape, escape, tea.Key{Code: 'f'})
	capture(escape, tea.Key{Code: 'd'})
	vault.saveErr = &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}
	capture(tea.Key{Code: 'y'})
	return views
}

func TestSavedLocaleReachesEveryScreenWithinLayoutBudgets(t *testing.T) {
	pseudo := installPseudoLocale(t)
	for _, size := range [][2]int{{140, 40}, {60, 20}, {24, 8}} {
		for i, view := range visitLocalizedScreens(t, " "+pseudo+" ", size[0], size[1]) {
			text := ansi.Strip(view)
			assert.LessOrEqual(t, lipgloss.Width(view), size[0], "screen %d", i)
			assert.LessOrEqual(t, lipgloss.Height(view), size[1], "screen %d", i)
			assert.NotRegexp(t, rawTextKey, text, "screen %d", i)
			if size[0] == 140 {
				assert.Contains(t, text, pseudoPrefix, "screen %d", i)
				assert.NotContains(t, text, "Password", "screen %d shows English text", i)
			}
		}
	}
}

func TestUnsupportedLocaleUsesCompleteEnglishText(t *testing.T) {
	for _, preference := range []string{"", "En", "unsupported"} {
		views := visitLocalizedScreens(t, preference, 140, 40)
		assert.Contains(t, ansi.Strip(views[0]), "Password")
		for i, view := range views {
			assert.NotRegexp(t, rawTextKey, ansi.Strip(view), "%q screen %d", preference, i)
		}
	}
}
