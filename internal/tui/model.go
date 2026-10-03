package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/ffi"
)

type Collection struct {
	Name string
}

type Model struct {
	Collections []Collection
}

func Run() error {
	program := tea.NewProgram(initialModel())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyPressMsg:
		switch message.String() {
		case "q", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	view := tea.NewView(renderView(m))
	view.AltScreen = true
	return view
}

func initialModel() Model {
	return Model{
		Collections: loadCollections(),
	}
}

func loadCollections() []Collection {
	names, err := ffi.List()
	if err != nil {
		return nil
	}

	items := make([]Collection, 0, len(names))
	for _, name := range names {
		items = append(items, Collection{Name: name})
	}

	return items
}
