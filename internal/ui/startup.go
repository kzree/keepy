package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
)

type clipboardInitializedMsg struct{ Error error }

// clipboardExpiredMsg asks to clear the password from copy ID; a newer copy
// supersedes it.
type clipboardExpiredMsg struct{ ID uint64 }

func initializeClipboard() error { return clipboard.Init() }

// writeClipboardText returns a channel that fires once the clipboard is later
// overwritten, or nil if the write failed.
func writeClipboardText(data []byte) <-chan struct{} { return clipboard.Write(clipboard.FmtText, data) }

// scheduleClipboardClear arms clearing for a password copy that returned changed.
func (r *RootModel) scheduleClipboardClear(changed <-chan struct{}) tea.Cmd {
	r.clipboardClearID++
	r.clipboardChanged = nil
	if r.clipboardClearDelay == 0 {
		return nil
	}
	r.clipboardChanged = changed
	id := r.clipboardClearID
	return tea.Tick(r.clipboardClearDelay, func(time.Time) tea.Msg { return clipboardExpiredMsg{ID: id} })
}

// clearCopiedPassword empties the clipboard if it still holds the copied
// password. Content written by another application is left alone.
func (r *RootModel) clearCopiedPassword() {
	changed := r.clipboardChanged
	if changed == nil {
		return
	}
	r.clipboardChanged = nil
	select {
	case <-changed:
		return
	default:
	}
	r.clipboardClearFailed = r.writeClipboard(nil) == nil
}

// quit clears a pending password copy before exiting, so quitting early does
// not leave it in the clipboard past its timeout.
func (r RootModel) quit() (RootModel, tea.Cmd) {
	r.clearCopiedPassword()
	return r, tea.Quit
}

func (r *RootModel) setLocale(l locale.Locale) {
	r.locale = l
	r.t = textprovider.New(l, namespace.Application)
}

// applyConfig resolves preferences and rebuilds the pre-login screens. An
// unavailable config uses English and default keys. Invalid key overrides are
// reported, and defaults stay in effect, without blocking login.
func (r *RootModel) applyConfig(cfg *service.Config) {
	l, overrides := locale.En, map[string][]string(nil)
	if cfg != nil {
		l, overrides = locale.Resolve(cfg.Preferences.Locale), cfg.Keybindings
	}
	r.setLocale(l)
	r.keys, r.keymapError = control.ResolveKeymap(overrides)
	r.clipboardClearDelay, r.clipboardTimeoutError = service.DefaultClipboardTimeout, nil
	if cfg != nil {
		r.clipboardClearDelay, r.clipboardTimeoutError = cfg.Preferences.ClipboardClearDelay()
	}
	r.login = login.NewLoginModel(cfg, r.locale, r.keys)
	r.list = list.NewListModel(r.locale, r.keys)
}

// updateApplicationNotice keeps independent startup/config/copy errors from
// clearing one another as individual operations recover.
func (r *RootModel) updateApplicationNotice() {
	var notices []string
	if r.configLoadError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeConfigLoad), control.Label(r.keys.Reload.Keys()...), r.configLoadError))
	}
	if r.keymapError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeKeymapInvalid), r.keymapError))
	}
	if r.clipboardTimeoutError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeClipboardTimeoutInvalid), r.clipboardClearDelay, r.clipboardTimeoutError))
	}
	if r.clipboardClearFailed {
		notices = append(notices, r.t.Text(key.NoticeClipboardClearFailed))
	}
	if r.clipboardError != nil {
		// golang.design/x/clipboard caches Init's result with sync.Once. Retrying
		// in this process cannot recover an initialization failure.
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeClipboardUnavailable), r.clipboardError))
	}
	var durability *service.ConfigDurabilityError
	if errors.As(r.configSaveError, &durability) {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeConfigSaveUncertain), durability.Err))
	} else if r.configSaveError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeConfigSave), r.configSaveError))
	}
	if r.copyError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeCopyFailed), r.copyError))
	}
	r.bottomBar.SetApplicationNotice(strings.Join(notices, "\n"))
}

func (r RootModel) retryConfiguration() (RootModel, tea.Cmd) {
	cfg, err := service.LoadConfig()
	r.configLoadError = err
	if err == nil {
		// Login is still blocked here, so the empty list can adopt the config too.
		r.cfg = cfg
		r.applyConfig(cfg)
	}
	r.updateApplicationNotice()
	r.resizeViews()
	if err != nil {
		return r, nil
	}
	cmd := r.login.Init()
	return r, cmd
}
