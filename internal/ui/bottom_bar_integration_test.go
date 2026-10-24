package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
)

func TestRootBottomBarOwnsGlobalNoticesAndRestoresViewSpace(t *testing.T) {
	r := rootWithVault(t, &outcomeVault{})
	r.resizeViews()
	_, initialHeight := r.getContentSize()
	r.bottomBar.SetApplicationNotice("settingsfailure: " + strings.Repeat("details ", 10))
	r.list, _ = r.list.Update(list.SaveResultMsg{Error: &service.SaveError{Kind: service.SaveDurabilityUncertain, Err: errors.New("flush failure")}})
	r.resizeViews()
	w, h := r.getContentSize()
	assert.Less(t, h, initialHeight)
	assert.NotContains(t, r.list.View(w, h), "settingsfailure")
	assert.NotContains(t, r.list.View(w, h), "Do not repeat")
	assert.Contains(t, r.View().Content, "Do not repeat")
	assert.Contains(t, r.View().Content, "settingsfailure")
	// The bar is anchored below a padded active view, rather than attached to
	// whichever child happens to render the error.
	lines := strings.Split(ansi.Strip(r.View().Content), "\n")
	assert.Contains(t, lines[len(lines)-2], "details")
	assert.Equal(t, r.height, lipgloss.Height(r.View().Content))
	r.list, _ = r.list.Update(list.SaveResultMsg{})
	r.bottomBar.SetApplicationNotice("")
	r.resizeViews()
	_, restoredHeight := r.getContentSize()
	assert.Equal(t, initialHeight, restoredHeight)
}

func TestRootBottomBarRemainsGlobalAcrossScreensAndNarrowResizes(t *testing.T) {
	r := rootWithVault(t, &outcomeVault{})
	r.bottomBar.SetApplicationNotice("settingsfailure: " + strings.Repeat("details ", 12))
	for _, view := range []screen{screenList, screenLogin} {
		r.activeView = view
		if view == screenLogin {
			r.login = login.NewLoginModel(&service.Config{}, locale.En, control.DefaultKeymap())
			drainRoot(t, &r, r.login.Init())
		}
		for _, size := range [][2]int{{120, 30}, {60, 18}, {20, 6}} {
			sendRoot(t, &r, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			assert.Contains(t, r.View().Content, "settingsfailure")
			assert.LessOrEqual(t, lipgloss.Height(r.View().Content), size[1])
			assert.LessOrEqual(t, lipgloss.Width(r.View().Content), size[0])
			_, viewHeight := r.getContentSize()
			assert.Positive(t, viewHeight, "a long notice must leave active-view space")
			if view == screenLogin {
				assert.Contains(t, r.View().Content, "Password")
			} else {
				assert.Contains(t, r.View().Content, "Title")
			}
		}
		sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
		_, grownHeight := r.getContentSize()
		assert.Greater(t, grownHeight, 20)
	}
}

func TestFailedReloadAppearsGloballyWithoutLosingDraft(t *testing.T) {
	vault := &outcomeVault{
		saveErr:   &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")},
		reloadErr: errors.New("reloadfailure"),
	}
	r := rootWithVault(t, vault)
	r.bottomBar.SetApplicationNotice("settingsfailure")
	submitThroughUI(t, &r, "add")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	w, h := r.getContentSize()
	assert.Contains(t, r.View().Content, "reloadfailure")
	assert.Contains(t, r.View().Content, "Ctrl+R")
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.NotContains(t, r.list.View(w, h), "reloadfailure")
	assert.Contains(t, r.list.View(w, h), "Committed")
	assert.True(t, r.list.IsTextInputActive())
	assert.Equal(t, 1, vault.mutations)
	assert.Equal(t, 1, vault.reloads)
}

func TestConflictAndReloadStatusUseBottomBarWhileDraftStaysInForm(t *testing.T) {
	vault := &outcomeVault{saveErr: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external-change-cause")}}
	r := rootWithVault(t, vault)
	r.bottomBar.SetApplicationNotice("settingsfailure")
	submitThroughUI(t, &r, "add")
	w, h := r.getContentSize()
	assert.Contains(t, r.View().Content, "Ctrl+R")
	assert.Contains(t, r.View().Content, "external-change-cause")
	assert.NotContains(t, r.list.View(w, h), "external-change-cause")
	assert.Contains(t, r.list.View(w, h), "Committed", "draft stays in the form")
	model, request := r.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	r = model.(RootModel)
	assert.Contains(t, r.View().Content, "Reloading database")
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.NotNil(t, request)
	if request != nil {
		sendRoot(t, &r, request())
	}
	assert.NotContains(t, r.View().Content, "Reloading database")
	assert.NotContains(t, r.View().Content, "external-change-cause")
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.True(t, r.list.IsTextInputActive())
	assert.Equal(t, 1, vault.mutations)
	assert.Equal(t, 1, vault.reloads)
}

func TestDraftSpecificFailureStaysInForm(t *testing.T) {
	r := rootWithVault(t, &outcomeVault{saveErr: &service.SaveError{Kind: service.SaveFailed, Err: errors.New("draftfail")}})
	submitThroughUI(t, &r, "add")
	w, h := r.getContentSize()
	assert.Contains(t, r.list.View(w, h), "draftfail")
	assert.Empty(t, r.renderBottomBar(w, h))
}
