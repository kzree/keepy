package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/style"
)

func (r *RootModel) renderCurrentView(contentWidth, contentHeight int) string {
	switch r.activeView {
	case screenLogin:
		if r.configLoadError != nil {
			return "Configuration unavailable\n\nCorrect the configuration file or its access permissions.\nPress Ctrl+R to retry loading it, or Ctrl+C to quit.\nThe existing file will not be overwritten."
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
