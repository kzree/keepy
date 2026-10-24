// Package list implements the screen for listing vault entries and showing details of a selected entry.
package list

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider"
	tk "kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/list/search"
)

type pane int

const copyFlashDuration = 100 * time.Millisecond

const (
	listPane pane = iota
	createPane
)

type ListModel struct {
	showSearch       bool
	showSidePane     bool
	showDeletePrompt bool
	activePane       pane
	table            table.Model
	copyFlashID      int
	copyFlashActive  bool
	entries          []service.VaultEntry
	filteredEntries  []service.VaultEntry
	filter           string
	saveNotice       error
	saveConflict     bool
	reloading        bool
	deleting         bool
	deleteRequestID  uint64
	openingEdit      bool
	editRequestID    uint64
	reloadRequestID  uint64

	t    *textprovider.TextProvider
	keys control.Keymap

	width  int
	height int

	search    search.SearchModel
	entryForm entryform.EntryFormModel
	confirm   common.ConfirmPromptModel
}

func NewListModel(l locale.Locale, keys control.Keymap) ListModel {
	t := textprovider.New(l, namespace.List)
	return ListModel{
		showSearch:   false,
		showSidePane: false,
		activePane:   listPane,
		table:        createEntryTable(t),
		search:       search.NewSearchModel(t),
		entryForm:    entryform.NewEntryFormModel(l, keys),
		confirm:      common.NewConfirmPrompt(keys),
		t:            t,
		keys:         keys,
	}
}

func (m ListModel) Init() tea.Cmd {
	return m.entryForm.Init()
}

func (m ListModel) IsTextInputActive() bool {
	return m.showSearch || (m.showSidePane && m.activePane == createPane)
}

func (m ListModel) KeymapContext() string {
	if m.showSearch {
		return control.SearchKeys
	}
	if m.showDeletePrompt {
		return control.ConfirmationKeys
	}
	if m.showSidePane && m.activePane == createPane {
		return control.EntryFormKeys
	}
	return control.ListKeys
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case entryform.SubmitSuccessMsg:
		m.openingEdit = false
		m.activePane = listPane
		m.showSidePane = false
		m.saveNotice = msg.Warning
		m.saveConflict = false
		m.resizeTable()
		cmd := m.entryForm.StartNew()
		return m, cmd
	case entryform.SubmitFailedMsg:
		m.openingEdit = false
		m.saveNotice = msg.Error
		m.saveConflict = isSaveConflict(msg.Error)
		m.resizeTable()
		form, cmd := m.entryForm.Update(msg)
		m.entryForm = form
		return m, cmd
	case SaveResultMsg:
		m.deleting = false
		m.openingEdit = false
		m.saveNotice = msg.Error
		m.saveConflict = isSaveConflict(msg.Error)
		m.resizeTable()
		return m, nil
	case ReloadVaultResultMsg:
		m.reloading = false
		if msg.Error != nil {
			m.saveNotice = msg.Error
			m.resizeTable()
			if m.showSidePane && m.activePane == createPane {
				form, cmd := m.entryForm.Update(entryform.ReloadFailedMsg{Error: msg.Error})
				m.entryForm = form
				return m, cmd
			}
			return m, nil
		}
		m.saveNotice = nil
		m.saveConflict = false
		m.resizeTable()
		if m.showSidePane && m.activePane == createPane {
			form, cmd := m.entryForm.Update(entryform.ReloadSuccessMsg{})
			m.entryForm = form
			return m, cmd
		}
		return m, nil
	case copyFlashDoneMsg:
		return m.handleCopyFlashDoneMsg(msg)
	case CopyPasswordSuccessMsg:
		return m.handleCopyPasswordSuccessMsg()
	case OpenEditEntryMsg:
		return m.handleOpenEditEntryMsg(msg)
	case common.CloseConfirmMsg:
		return m.handleCloseDeletePrompt(msg)
	case common.OpenConfirmMsg:
		if !m.showDeletePrompt || m.deleting || msg.RequestID != m.deleteRequestID {
			return m, nil
		}
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		m.resizeTable()
		return m, cmd
	case tea.KeyPressMsg:
		if m.reloading || m.deleting {
			return m, nil
		}
		if key.Matches(msg, m.keys.Reload) && m.saveConflict {
			m.reloading = true
			m.openingEdit = false
			m.reloadRequestID++
			requestID := m.reloadRequestID
			m.resizeTable()
			return m, func() tea.Msg { return ReloadVaultRequestMsg{RequestID: requestID} }
		}
		if msg.String() == "esc" && m.openingEdit {
			m.openingEdit = false
			return m, nil
		}
		if msg.String() == control.CloseEntryForm && m.showSidePane && m.activePane == createPane && !m.entryForm.IsSaving() {
			return m.handleCloseNewEntryForm()
		}
		if m.saveConflict && !m.showSearch && key.Matches(msg, m.keys.NewEntry, m.keys.EditEntry, m.keys.DeleteEntry) && !(m.showSidePane && m.activePane == createPane) {
			return m, nil
		}
		if m.showSidePane && m.activePane == createPane {
			break
		}
		if m.showSearch {
			switch msg.String() {
			case "esc", "enter":
				return m.handleCloseSearch()
			}

			break
		}

		disableInput := m.showSearch || m.showDeletePrompt || (m.showSidePane && m.activePane == createPane)

		if !disableInput {
			switch {
			case key.Matches(msg, m.keys.CopyEntry):
				return m.handleCopyEntry()
			case key.Matches(msg, m.keys.NewEntry):
				return m.handleOpenNewEntryForm()
			case key.Matches(msg, m.keys.EditEntry):
				return m.handleOpenEditEntryForm()
			case key.Matches(msg, m.keys.ClearFilter):
				return m.handleClearFilter()
			case key.Matches(msg, m.keys.Search):
				return m.handleShowSearch()
			case key.Matches(msg, m.keys.DeleteEntry):
				return m.handleOpenDeletePrompt()
			}
		}
	}

	if m.showSearch {
		oldValue := m.search.Value()
		s, cmd := m.search.Update(msg)
		m.search = s
		if m.search.Value() != oldValue {
			m.FilterEntries(m.search.Value())
		}

		cmds = append(cmds, cmd)
	} else if m.showDeletePrompt {
		c, cmd := m.confirm.Update(msg)
		m.confirm = c
		cmds = append(cmds, cmd)
	} else if m.showSidePane && m.activePane == createPane {
		entryForm, cmd := m.entryForm.Update(msg)
		m.entryForm = entryForm
		cmds = append(cmds, cmd)
	} else {
		t, cmd := m.table.Update(msg)
		m.table = t
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func isSaveConflict(err error) bool {
	var outcome *service.SaveError
	return errors.As(err, &outcome) && outcome.Kind == service.SaveConflict
}

func (m ListModel) AcceptsFormSubmission(requestID uint64) bool {
	return !m.saveConflict && !m.reloading && !m.deleting && m.showSidePane && m.activePane == createPane && m.entryForm.AcceptsSubmission(requestID)
}

func (m ListModel) AcceptsDelete(requestID uint64) bool {
	return !m.saveConflict && !m.reloading && m.deleting && m.deleteRequestID == requestID
}

func (m ListModel) AcceptsEditOpen(requestID uint64) bool {
	return !m.saveConflict && !m.reloading && !m.deleting && m.openingEdit && m.editRequestID == requestID
}

func (m ListModel) AcceptsReload(requestID uint64) bool {
	return m.saveConflict && m.reloading && m.reloadRequestID == requestID
}

func entriesToRows(entries []service.VaultEntry) []table.Row {
	rows := make([]table.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, table.Row{
			entry.Title,
			entry.URL,
			entry.Username,
			entry.Group,
		})
	}
	return rows
}

func (m *ListModel) SetEntries(entries []service.VaultEntry) {
	m.entries = entries
	m.FilterEntries(m.filter)
}

func (m *ListModel) FilterEntries(val string) {
	m.filter = val
	filtered := m.entries
	if val != "" {
		filtered = slices.Collect(func(yield func(service.VaultEntry) bool) {
			for _, v := range m.entries {
				title := strings.ToLower(v.Title)
				substr := strings.ToLower(val)
				if strings.Contains(title, substr) || strings.Contains(v.Username, val) {
					if !yield(v) {
						return
					}
				}
			}
		})
	}

	currentCursor := m.table.Cursor()
	var selected gokeepasslib.UUID

	hasSelection := currentCursor >= 0 && currentCursor < len(m.filteredEntries)
	if hasSelection {
		selected = m.filteredEntries[currentCursor].ID
	}

	m.filteredEntries = filtered
	// Retain the same entry across sorting/refreshes. If it disappeared or no
	// longer matches, retain its row position, clamped to the remaining rows.
	target := max(0, min(currentCursor, len(filtered)-1))

	for i, entry := range m.filteredEntries {
		if hasSelection && entry.ID == selected {
			target = i
			break
		}
	}
	m.table.SetRows(entriesToRows(filtered))
	m.restoreTableCursor(target)
}

func clearCopyFlashCmd(id int) tea.Cmd {
	return tea.Tick(copyFlashDuration, func(time.Time) tea.Msg {
		return copyFlashDoneMsg{id: id}
	})
}

// GlobalNotice exposes operation status to the root BottomBar. Draft-specific
// failures remain in the entry form; views never append global status themselves.
func (m ListModel) GlobalNotice() string {
	if m.deleting {
		return m.t.Text(tk.DeletingEntry)
	}
	var body string
	if m.saveNotice != nil && (m.saveConflict || !(m.showSidePane && m.activePane == createPane)) {
		var outcome *service.SaveError
		if errors.As(m.saveNotice, &outcome) && outcome.Committed() {
			body = m.t.Text(tk.SaveCommittedNotice) + "\n"
		}
		if m.saveConflict {
			body = fmt.Sprintf(m.t.Text(tk.ReloadConflict), control.Label(m.keys.Reload.Keys()...)) + "\n"
		}
		body += m.saveNotice.Error()
	}
	if m.reloading {
		body = m.t.Text(tk.ReloadingDatabase)
	}
	return body
}
