package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/ffi"
)

type panel int
type TickingMessage struct{}

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
	LastQuery     []float32
}

func Run() error {
	program := tea.NewProgram(initialModel())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickingMessage{}
	})
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.PasteMsg:
		if m.isSearchPanelFocused() {
			m.SearchInput += message.String()
		}
	case TickingMessage:
		if err := ffi.Reload(); err != nil {
			m.StatusMessage = fmt.Sprintf("reload error: %v", err)
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return TickingMessage{}
			})
		}
		latestCollections, latestSelectedIndex := reloadCollections(m.Collections, m.SelectedIndex)
		m.Collections = latestCollections
		m.SelectedIndex = latestSelectedIndex
		m.refreshSearch()
		m.StatusMessage = fmt.Sprintf("refreshed at %s", time.Now().Format("15:04:05"))
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return TickingMessage{}
		})
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
		m.StatusMessage = "enter a vector file path, or comma-separated floats"
		return m, nil
	}

	query, err := ParseQuery(m.SearchInput)
	if err != nil {
		m.StatusMessage = fmt.Sprintf("error :%v", err)
		return m, nil
	}

	m.LastQuery = query
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

func (m *Model) refreshSearch() {
	shouldSkipRefresh := !m.isResultsPanelFocused() || len(m.LastQuery) == 0 || len(m.Collections) == 0
	if shouldSkipRefresh {
		return
	}

	isValidIndex := m.SelectedIndex < len(m.Collections)
	if !isValidIndex {
		return
	}

	collection := m.Collections[m.SelectedIndex]
	results, err := ffi.Search(collection.Name, m.LastQuery, 10)
	if err == nil {
		m.SearchResults = results
	}
}

func reloadCollections(previousCollections []Collection, previousIndex int) ([]Collection, int) {
	currentCollections := loadCollections()
	if previousIndex >= len(previousCollections) {
		return currentCollections, 0
	}

	selectedName := previousCollections[previousIndex].Name
	for i, collection := range currentCollections {
		if collection.Name == selectedName {
			return currentCollections, i
		}
	}
	return currentCollections, 0
}
