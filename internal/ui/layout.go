package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/textprovider/key"
)

func (r *RootModel) renderCurrentView(contentWidth, contentHeight int) string {
	if r.helpVisible {
		return r.keymapHelp.View()
	}
	switch r.activeView {
	case screenLogin:
		if r.configLoadError != nil {
			return fmt.Sprintf(r.t.Text(key.ConfigUnavailable), control.Label(r.keys.Reload.Keys()...), control.Label(r.keys.Quit.Keys()...))
		}
		return r.login.View()
	case screenList:
		return r.list.View(contentWidth, contentHeight)
	}
	return ""
}

func (r *RootModel) contentLayout() (width, height int, bar string) {
	frameW, frameH := style.Frame.GetFrameSize()
	width = max(0, r.width-frameW)
	height = max(0, r.height-frameH)
	bar = r.renderBottomBar(width, height)
	if bar != "" {
		height -= lipgloss.Height(bar)
	}
	return
}

func (r *RootModel) getContentSize() (int, int) {
	w, h, _ := r.contentLayout()
	return w, h
}

func (r RootModel) renderBottomBar(width, height int) string {
	bar := r.bottomBar
	if r.activeView == screenList {
		status := r.list.GlobalNotice()
		if r.pendingOperation.kind.usesGlobalSpinner() {
			status = r.operationSpinner.View() + " " + status
		}
		bar.SetStatus(status)
	}
	// Keep the footer compact and preserve room for the active view. Extremely
	// small terminals still get one row of prioritized recovery guidance.
	budget := min(height, min(5, max(1, height/4)))
	return bar.View(width, budget)
}

func (r *RootModel) resizeViews() {
	w, h := r.getContentSize()
	r.list.SetListTableSize(w, h)
	r.login.SetSize(w, h)
	if r.helpVisible {
		r.keymapHelp.SetContext(r.keymapContext(), r.pendingOperation.kind != operationNone)
		r.keymapHelp.SetSize(w, h)
	}
}

func (r *RootModel) renderFrame(content string) string {
	return style.Frame.Width(max(0, r.width)).Height(max(0, r.height)).Render(content)
}

func (r RootModel) View() tea.View {
	w, h, bar := r.contentLayout()
	body := r.renderCurrentView(w, h)
	if w <= 0 || h <= 0 {
		body = ""
	} else {
		body = lipgloss.NewStyle().Width(w).Height(h).MaxWidth(w).MaxHeight(h).Render(body)
	}
	if bar != "" {
		if body == "" {
			body = bar
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, body, bar)
		}
	}
	view := tea.NewView(r.renderFrame(body))
	view.AltScreen = true
	return view
}
