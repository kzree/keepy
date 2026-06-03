package screens

import tea "charm.land/bubbletea/v2"

type ListModel struct{}

func NewListModel() ListModel {
	return ListModel{}
}

func (m ListModel) Init() tea.Cmd {
	return nil
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	return m, nil
}

func (m ListModel) View() tea.View {
	return tea.NewView("List")
}
