package ui

import (
	"fmt"
	"strings"

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

func initializeClipboard() error          { return clipboard.Init() }
func writeClipboardText(data []byte) bool { return clipboard.Write(clipboard.FmtText, data) != nil }

// configLocale resolves the saved preference; an unavailable config uses English.
func configLocale(cfg *service.Config) locale.Locale {
	if cfg == nil {
		return locale.En
	}
	return locale.Resolve(cfg.Preferences.Locale)
}

func (r *RootModel) setLocale(l locale.Locale) {
	r.locale = l
	r.t = textprovider.New(l, namespace.Application)
}

// updateApplicationNotice keeps independent startup/config/copy errors from
// clearing one another as individual operations recover.
func (r *RootModel) updateApplicationNotice() {
	var notices []string
	if r.configLoadError != nil {
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeConfigLoad), control.Label(control.ReloadVault), r.configLoadError))
	}
	if r.clipboardError != nil {
		// golang.design/x/clipboard caches Init's result with sync.Once. Retrying
		// in this process cannot recover an initialization failure.
		notices = append(notices, fmt.Sprintf(r.t.Text(key.NoticeClipboardUnavailable), r.clipboardError))
	}
	if r.configSaveError != nil {
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
		// Login is still blocked here, so the empty list can adopt the locale too.
		r.cfg = cfg
		r.setLocale(configLocale(cfg))
		r.login = login.NewLoginModel(cfg, r.locale)
		r.list = list.NewListModel(r.locale)
	}
	r.updateApplicationNotice()
	r.resizeViews()
	if err != nil {
		return r, nil
	}
	cmd := r.login.Init()
	return r, cmd
}
