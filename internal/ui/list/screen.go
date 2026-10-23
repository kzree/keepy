// Package list implements the screen for listing vault entries and showing details of a selected entry.
package list

import (
	"errors"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/list/search"
	"kzree.com/keepy/internal/util"
)

type pane int

const copyFlashDuration = 100 * time.Millisecond

const (
	listPane pane = iota
	createPane
)

const (
	paneGap      = 1
	promptHeight = 2
)

type ListModel struct {
	showSearch       bool
	showSidePane     bool
	showDeletePrompt bool
	activePane       pane
	table            table.Model
	copyFlashID      int
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

	t *textprovider.TextProvider

	width  int
	height int

	search    search.SearchModel
	entryForm entryform.EntryFormModel
	confirm   common.ConfirmPromptModel
}

func NewListModel() ListModel {
	t := textprovider.New(locale.En, namespace.List)
	return ListModel{
		showSearch:   false,
		showSidePane: false,
		activePane:   listPane,
		table:        createEntryTable(t),
		search:       search.NewSearchModel(t),
		entryForm:    entryform.NewEntryFormModel(),
		confirm:      common.NewConfirmPrompt(),
		t:            t,
	}
}

func (m ListModel) Init() tea.Cmd {
	return m.entryForm.Init()
}

func (m ListModel) IsTextInputActive() bool {
	return m.showSearch || (m.showSidePane && m.activePane == createPane)
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
	case tea.KeyPressMsg:
		if m.reloading || m.deleting {
			return m, nil
		}
		if msg.String() == control.ReloadVault && m.saveConflict {
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
		if m.saveConflict && (msg.String() == control.NewEntry || msg.String() == control.EditEntry || msg.String() == control.DeleteEntity) && !(m.showSidePane && m.activePane == createPane) {
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
			switch msg.String() {
			case control.CopyEntry:
				return m.handleCopyEntry()
			case control.NewEntry:
				return m.handleOpenNewEntryForm()
			case control.EditEntry:
				return m.handleOpenEditEntryForm()
			case control.ClearFilter:
				return m.handleClearFilter()
			case control.Search:
				return m.handleShowSearch()
			case control.DeleteEntity:
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
	// SetCursor alone does not scroll Bubbles' viewport to reveal the row.
	m.table.GotoTop()
	if len(filtered) > 0 {
		m.table.MoveDown(target)
	}
}

func clearCopyFlashCmd(id int) tea.Cmd {
	return tea.Tick(copyFlashDuration, func(time.Time) tea.Msg {
		return copyFlashDoneMsg{id: id}
	})
}

func (m ListModel) renderPane(isActive bool, width, height int, content string) string {
	style := style.GetPaneStyle(isActive)
	frameW, frameH := style.GetFrameSize()
	return style.
		Width(max(0, width-frameW)).
		Height(max(0, height-frameH)).
		Render(content)
}

func (m *ListModel) getPaneWidths(contentWidth int) (int, int) {
	availableWidth := max(0, contentWidth-paneGap)
	third := availableWidth / 3

	leftWidth := util.Ternary(m.showSidePane, third*2, availableWidth)
	rightWidth := third

	return leftWidth, rightWidth
}

func (m *ListModel) resizeEntryForm(rightWidth int) {
	paneStyle := style.GetPaneStyle(m.activePane == createPane)
	frameW, frameH := paneStyle.GetFrameSize()
	m.entryForm.SetSize(
		max(0, rightWidth-frameW),
		max(0, m.height-frameH),
	)
}

func (m *ListModel) SetListTableSize(contentWidth, contentHeight int) {
	if m.width == contentWidth && m.height == contentHeight {
		return
	}
	m.width = contentWidth
	m.height = contentHeight

	m.resizeTable()
}

func (m *ListModel) resizeTable() {
	leftWidth, rightWidth := m.getPaneWidths(m.width)
	baseHeight := max(1, m.height)
	m.table.SetWidth(leftWidth)
	m.table.SetHeight(util.Ternary(m.showSearch || m.showDeletePrompt, baseHeight-promptHeight, baseHeight))

	if m.showSidePane {
		m.resizeEntryForm(rightWidth)
	}
}

func (m ListModel) View(contentWidth, contentHeight int) string {
	leftWidth, rightWidth := m.getPaneWidths(contentWidth)
	paneHeight := max(0, contentHeight)

	left := m.table.View()
	if m.showSearch {
		left = lipgloss.JoinVertical(lipgloss.Left, m.search.View(), m.table.View())
	} else if m.showDeletePrompt {
		left = lipgloss.JoinVertical(lipgloss.Left, m.table.View(), m.confirm.View())
	}
	left = boundedView(left, leftWidth, paneHeight)
	right := ""
	if m.showSidePane && paneHeight > 0 {
		right = boundedView(m.renderPane(m.activePane == createPane, rightWidth, paneHeight, m.entryForm.View()), rightWidth, paneHeight)
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", paneGap),
		right,
	)
	return body
}

func boundedView(body string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(body)
}

// GlobalNotice exposes operation status to the root BottomBar. Draft-specific
// failures remain in the entry form; views never append global status themselves.
func (m ListModel) GlobalNotice() string {
	var body string
	if m.saveNotice != nil && (m.saveConflict || !(m.showSidePane && m.activePane == createPane)) {
		var outcome *service.SaveError
		if errors.As(m.saveNotice, &outcome) && outcome.Committed() {
			body = "Save committed. Do not repeat the operation; durability could not be confirmed.\n"
		}
		if m.saveConflict {
			body = "ctrl+r: reload latest database before trying another mutation.\n"
		}
		body += m.saveNotice.Error()
	}
	if m.reloading {
		body = "Reloading database..."
	}
	return body
}
