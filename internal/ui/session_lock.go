package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
)

// lockCheckMsg asks whether the session from check ID has been idle long
// enough to lock. A relogin or lock starts a new ID, so older checks are ignored.
type lockCheckMsg struct{ ID uint64 }

// startLockTimer begins inactivity tracking for a newly opened vault. One check
// is pending at a time; input only records its time instead of resetting a timer.
func (r *RootModel) startLockTimer() tea.Cmd {
	r.lockCheckID++
	r.lastActivity = r.now()
	return r.scheduleLockCheck(r.lockDelay)
}

func (r *RootModel) scheduleLockCheck(after time.Duration) tea.Cmd {
	if r.lockDelay == 0 {
		return nil
	}
	id := r.lockCheckID
	return tea.Tick(after, func(time.Time) tea.Msg { return lockCheckMsg{ID: id} })
}

// handleLockCheck locks an idle vault, or checks again when the session can
// next become idle. An accepted vault operation postpones locking, since its
// result must still be applied to the open session.
func (r RootModel) handleLockCheck(msg lockCheckMsg) (RootModel, tea.Cmd) {
	if msg.ID != r.lockCheckID || r.lockDelay == 0 || r.activeView != screenList {
		return r, nil
	}
	if r.pendingOperation.kind != operationNone || r.quitAfterOperation {
		return r, r.scheduleLockCheck(r.lockDelay)
	}
	if idle := r.now().Sub(r.lastActivity); idle < r.lockDelay {
		return r, r.scheduleLockCheck(r.lockDelay - idle)
	}
	return r.lockVault()
}

// lockVault forgets the decrypted vault and every view holding its data,
// including an unsaved entry draft, and returns to login with saved paths.
func (r RootModel) lockVault() (RootModel, tea.Cmd) {
	r.lockCheckID++
	r.clearCopiedPassword()
	r.db.Close()
	r.lockedAfterInactivity = true
	r.lockDiscardedDraft = r.list.KeymapContext() == control.EntryFormKeys
	r.helpVisible = false
	r.activeView = screenLogin
	r.login = login.NewLoginModel(r.cfg, r.locale, r.keys)
	r.list = list.NewListModel(r.locale, r.keys)
	r.updateApplicationNotice()
	r.resizeViews()
	return r, r.login.Init()
}
