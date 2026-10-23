package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/login"
)

type clipboardInitializedMsg struct{ Error error }

func initializeClipboard() error          { return clipboard.Init() }
func writeClipboardText(data []byte) bool { return clipboard.Write(clipboard.FmtText, data) != nil }

// updateApplicationNotice keeps independent startup/config/copy errors from
// clearing one another as individual operations recover.
func (r *RootModel) updateApplicationNotice() {
	var notices []string
	if r.configLoadError != nil {
		notices = append(notices, "Ctrl+R: retry configuration loading. "+r.configLoadError.Error())
	}
	if r.clipboardError != nil {
		// golang.design/x/clipboard caches Init's result with sync.Once. Retrying
		// in this process cannot recover an initialization failure.
		notices = append(notices, "Clipboard unavailable. Restore clipboard access and restart to enable copying: "+r.clipboardError.Error())
	}
	if r.configSaveError != nil {
		notices = append(notices, "Vault opened, but settings could not be saved: "+r.configSaveError.Error())
	}
	if r.copyError != nil {
		notices = append(notices, "Copy failed: "+r.copyError.Error())
	}
	r.bottomBar.SetApplicationNotice(strings.Join(notices, "\n"))
}

func (r RootModel) retryConfiguration() (RootModel, tea.Cmd) {
	cfg, err := service.LoadConfig()
	r.configLoadError = err
	if err == nil {
		r.cfg = cfg
		r.login = login.NewLoginModel(cfg)
	}
	r.updateApplicationNotice()
	r.resizeViews()
	if err != nil {
		return r, nil
	}
	cmd := r.login.Init()
	return r, cmd
}
