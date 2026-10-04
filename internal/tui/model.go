package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
)

type panel int

const (
	collectionPanel panel = iota
	searchPanel
	resultsPanel
)

type Collection struct {
	Name  string
	Stats ffi.NekoStats
}

type Model struct {
	Collections   []Collection
	SelectedIndex int
	FocusedPanel  panel
	SearchInput   string
	SearchResults []ffi.NekoSearchResult
	StatusMessage string
	LastLatency   time.Duration
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
	case tea.PasteMsg:
		if m.isSearchPanelFocused() {
			m.SearchInput += message.String()
		}
	case tea.KeyPressMsg:
		switch message.String() {
		case "q":
			if !m.isSearchPanelFocused() {
				return m, tea.Quit
			} else {
				m.SearchInput += "q"
			}
		case "esc":
			if m.isSearchPanelFocused() || m.isResultsPanelFocused() {
				m.FocusedPanel = collectionPanel
			} else {
				return m, tea.Quit
			}
		case "/":
			if m.isSearchPanelFocused() {
				m.SearchInput += "/"
			} else {
				m.FocusedPanel = searchPanel
			}
		case "enter":
			if m.isSearchPanelFocused() {
				return m.submitSearch()
			}
		case "up":
			isValidIndex := m.SelectedIndex > 0
			shouldGoUp := isValidIndex && m.isCollectionPanelFocused()
			if shouldGoUp {
				m.SelectedIndex--
			}
		case "down":
			isValidIndex := m.SelectedIndex < len(m.Collections)-1
			shouldGoDown := isValidIndex && m.isCollectionPanelFocused()
			if shouldGoDown {
				m.SelectedIndex++
			}
		case "backspace":
			hasValues := len(m.SearchInput) > 0
			shouldRemoveCharacter := hasValues && m.isSearchPanelFocused()
			if shouldRemoveCharacter {
				m.SearchInput = m.SearchInput[:len(m.SearchInput)-1]
			}
		case "tab":
			nextPanel := (int(m.FocusedPanel) + 1) % 3
			m.FocusedPanel = panel(nextPanel)
		case "shift+tab":
			previousPanel := (int(m.FocusedPanel) + 2) % 3
			m.FocusedPanel = panel(previousPanel)
		default:
			if m.isSearchPanelFocused() {
				text := message.Key().Text
				if text != "" {
					m.SearchInput += text
				}
			}
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
		Collections:   loadCollections(),
		SelectedIndex: 0,
		FocusedPanel:  collectionPanel,
	}
}

func loadCollections() []Collection {
	names, err := ffi.List()
	if err != nil {
		return nil
	}

	items := make([]Collection, 0, len(names))
	for _, name := range names {
		stats, err := ffi.Stats(name)
		if err != nil {
			continue
		}
		items = append(items, Collection{
			Name:  name,
			Stats: stats,
		})
	}

	return items
}

func (m Model) isSearchPanelFocused() bool {
	return m.FocusedPanel == searchPanel
}

func (m Model) isCollectionPanelFocused() bool {
	return m.FocusedPanel == collectionPanel
}

func (m Model) isResultsPanelFocused() bool {
	return m.FocusedPanel == resultsPanel
}

func (m Model) submitSearch() (tea.Model, tea.Cmd) {
	if len(m.Collections) == 0 {
		m.StatusMessage = "no collections to search"
		return m, nil
	}

	if m.SearchInput == "" {
		m.StatusMessage = "enter a vector file path"
		return m, nil
	}

	query, err := shared.ParseVectorFile(m.SearchInput)
	if err != nil {
		m.StatusMessage = fmt.Sprintf("error :%v", err)
		return m, nil
	}

	start := time.Now()
	collection := m.Collections[m.SelectedIndex]
	results, err := ffi.Search(collection.Name, query, 10)
	m.LastLatency = time.Since(start)

	if err != nil {
		m.StatusMessage = fmt.Sprintf("search error: %v", err)
		return m, nil
	}

	m.SearchResults = results
	m.StatusMessage = fmt.Sprintf("found %d results in %s", len(results), m.LastLatency)
	m.FocusedPanel = resultsPanel
	return m, nil
}
