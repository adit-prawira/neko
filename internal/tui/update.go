package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func Init(m state.Model) tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return state.TickingMessage{}
	})
}

func Update(m state.Model, message tea.Msg) (state.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.PasteMsg:
		if m.IsSearchPanelFocused() {
			m.SearchInput += message.String()
		}
	case state.TickingMessage:
		if err := ffi.Reload(); err != nil {
			m.StatusMessage = fmt.Sprintf("reload error: %v", err)
			return m, nextTick()
		}
		latestCollections, latestSelectedIndex := reloadCollections(m.Collections, m.SelectedIndex)
		m.Collections = latestCollections
		m.SelectedIndex = latestSelectedIndex
		refreshSearch(&m)
		m.StatusMessage = fmt.Sprintf("refreshed at %s", time.Now().Format("15:04:05"))
		return m, nextTick()
	case tea.KeyPressMsg:
		switch message.String() {
		case "q":
			if !m.IsSearchPanelFocused() {
				return m, tea.Quit
			} else {
				m.SearchInput += "q"
			}
		case "esc":
			if m.IsSearchPanelFocused() || m.IsResultsPanelFocused() {
				m.FocusedPanel = state.CollectionPanel
			} else {
				return m, tea.Quit
			}
		case "/":
			if m.IsSearchPanelFocused() {
				m.SearchInput += "/"
			} else {
				m.FocusedPanel = state.SearchPanel
			}
		case "enter":
			if m.IsSearchPanelFocused() {
				return submitSearch(m)
			}
		case "up":
			isValidIndex := m.SelectedIndex > 0
			shouldGoUp := isValidIndex && m.IsCollectionPanelFocused()
			if shouldGoUp {
				m.SelectedIndex--
			}
		case "down":
			isValidIndex := m.SelectedIndex < len(m.Collections)-1
			shouldGoDown := isValidIndex && m.IsCollectionPanelFocused()
			if shouldGoDown {
				m.SelectedIndex++
			}
		case "backspace":
			hasValues := len(m.SearchInput) > 0
			shouldRemoveCharacter := hasValues && m.IsSearchPanelFocused()
			if shouldRemoveCharacter {
				m.SearchInput = m.SearchInput[:len(m.SearchInput)-1]
			}
		case "tab":
			nextPanel := (int(m.FocusedPanel) + 1) % 3
			m.FocusedPanel = state.Panel(nextPanel)
		case "shift+tab":
			previousPanel := (int(m.FocusedPanel) + 2) % 3
			m.FocusedPanel = state.Panel(previousPanel)
		default:
			if m.IsSearchPanelFocused() {
				text := message.Key().Text
				if text != "" {
					m.SearchInput += text
				}
			}
		}
	}
	return m, nil
}

func loadCollections() []state.Collection {
	names, err := ffi.List()
	if err != nil {
		return nil
	}

	items := make([]state.Collection, 0, len(names))
	for _, name := range names {
		stats, err := ffi.Stats(name)
		if err != nil {
			continue
		}
		items = append(items, state.Collection{
			Name:  name,
			Stats: stats,
		})
	}

	return items
}

func submitSearch(m state.Model) (state.Model, tea.Cmd) {
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
	m.FocusedPanel = state.ResultsPanel
	return m, nil
}

func refreshSearch(m *state.Model) {
	shouldSkipRefresh := !m.IsResultsPanelFocused() || len(m.LastQuery) == 0 || len(m.Collections) == 0
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

func reloadCollections(previousCollections []state.Collection, previousIndex int) ([]state.Collection, int) {
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

func nextTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return state.TickingMessage{}
	})
}
