package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/tui/state"
)

type model struct {
	state state.Model
}

func (m model) Init() tea.Cmd {
	return Init(m.state)
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	nextState, cmd := Update(m.state, message)
	return model{
		state: nextState,
	}, cmd
}

func (m model) View() tea.View {
	view := tea.NewView(Render(m.state))
	view.AltScreen = true
	return view
}

func Run() error {
	program := tea.NewProgram(initialModel())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func initialModel() model {
	return model{
		state: state.Model{
			Collections:   loadCollections(),
			SelectedIndex: 0,
			FocusedPanel:  state.CollectionPanel,
		},
	}
}
