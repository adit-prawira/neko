package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/ffi"
)

func TestUpdate(t *testing.T) {
	t.Run("given_q_key_press_then_returns_quit_command", func(t *testing.T) {
		m := Model{}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))

		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
	})

	t.Run("given_esc_key_press_then_returns_quit_command", func(t *testing.T) {
		m := Model{}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))

		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
	})

	t.Run("given_non_quit_key_then_returns_same_model_no_command", func(t *testing.T) {
		m := Model{Collections: []Collection{{Name: "alpha"}}}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a'}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if len(got.Collections) != 1 || got.Collections[0].Name != "alpha" {
			t.Fatalf("expected Collections preserved, got %+v", got.Collections)
		}
	})

	t.Run("given_down_key_then_increments_selected_index", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 0,
		}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SelectedIndex != 1 {
			t.Fatalf("expected SelectedIndex=1, got %d", got.SelectedIndex)
		}
	})

	t.Run("given_up_key_then_decrements_selected_index", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SelectedIndex != 0 {
			t.Fatalf("expected SelectedIndex=0, got %d", got.SelectedIndex)
		}
	})

	t.Run("given_down_key_at_last_index_then_stays_at_last", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SelectedIndex != 1 {
			t.Fatalf("expected SelectedIndex=1, got %d", got.SelectedIndex)
		}
	})

	t.Run("given_up_key_at_first_index_then_stays_at_first", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 0,
		}
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SelectedIndex != 0 {
			t.Fatalf("expected SelectedIndex=0, got %d", got.SelectedIndex)
		}
	})

	t.Run("given_slash_key_when_collection_focused_then_focuses_search_panel", func(t *testing.T) {
		m := Model{FocusedPanel: collectionPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: '/'}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.FocusedPanel != searchPanel {
			t.Fatalf("expected searchPanel, got %d", got.FocusedPanel)
		}
	})

	t.Run("given_slash_key_when_search_focused_then_appends_to_input", func(t *testing.T) {
		m := Model{FocusedPanel: searchPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: '/'}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SearchInput != "/" {
			t.Fatalf("expected SearchInput '/', got %q", got.SearchInput)
		}
	})

	t.Run("given_esc_key_when_search_focused_then_returns_to_collections", func(t *testing.T) {
		m := Model{FocusedPanel: searchPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.FocusedPanel != collectionPanel {
			t.Fatalf("expected collectionPanel, got %d", got.FocusedPanel)
		}
	})

	t.Run("given_esc_key_when_results_focused_then_returns_to_collections", func(t *testing.T) {
		m := Model{FocusedPanel: resultsPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.FocusedPanel != collectionPanel {
			t.Fatalf("expected collectionPanel, got %d", got.FocusedPanel)
		}
	})

	t.Run("given_q_key_when_search_focused_then_appends_to_input", func(t *testing.T) {
		m := Model{FocusedPanel: searchPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SearchInput != "q" {
			t.Fatalf("expected SearchInput 'q', got %q", got.SearchInput)
		}
	})

	t.Run("given_q_key_when_collection_focused_then_returns_quit_command", func(t *testing.T) {
		m := Model{FocusedPanel: collectionPanel}
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))

		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
	})

	t.Run("given_tab_key_then_cycles_to_next_panel", func(t *testing.T) {
		m := Model{FocusedPanel: collectionPanel}
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.FocusedPanel != searchPanel {
			t.Fatalf("expected searchPanel, got %d", got.FocusedPanel)
		}
	})

	t.Run("given_shift_tab_key_then_cycles_to_previous_panel", func(t *testing.T) {
		m := Model{FocusedPanel: searchPanel}
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.FocusedPanel != collectionPanel {
			t.Fatalf("expected collectionPanel, got %d", got.FocusedPanel)
		}
	})
}

func TestUpdatePaste(t *testing.T) {
	t.Run("given_paste_message_when_search_focused_then_appends_text", func(t *testing.T) {
		m := Model{FocusedPanel: searchPanel}
		updated, cmd := m.Update(tea.PasteMsg{Content: "/tmp/query.f32"})

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SearchInput != "/tmp/query.f32" {
			t.Fatalf("expected SearchInput '/tmp/query.f32', got %q", got.SearchInput)
		}
	})

	t.Run("given_paste_message_when_collection_focused_then_ignored", func(t *testing.T) {
		m := Model{FocusedPanel: collectionPanel}
		updated, cmd := m.Update(tea.PasteMsg{Content: "ignored"})

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		got, ok := updated.(Model)
		if !ok {
			t.Fatalf("expected Model, got %T", updated)
		}
		if got.SearchInput != "" {
			t.Fatalf("expected empty SearchInput, got %q", got.SearchInput)
		}
	})
}

func TestView(t *testing.T) {
	t.Run("given_model_then_alt_screen_enabled", func(t *testing.T) {
		m := Model{}
		view := m.View()

		if !view.AltScreen {
			t.Fatal("expected AltScreen true, got false")
		}
		if view.Content == "" {
			t.Fatal("expected non-empty view content")
		}
	})
}

func TestInit(t *testing.T) {
	t.Run("given_model_then_returns_tick_command", func(t *testing.T) {
		m := Model{}
		cmd := m.Init()
		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
	})
}

func TestRefreshSearch(t *testing.T) {
	t.Run("given_collection_panel_focused_then_does_nothing", func(t *testing.T) {
		m := &Model{
			FocusedPanel:  collectionPanel,
			LastQuery:     []float32{0.1, 0.2, 0.3},
			Collections:   []Collection{{Name: "docs"}},
			SearchResults: []ffi.NekoSearchResult{{ID: "old", Score: 1.0}},
		}
		m.refreshSearch()

		if len(m.SearchResults) != 1 || m.SearchResults[0].ID != "old" {
			t.Fatalf("expected search results to remain unchanged, got %v", m.SearchResults)
		}
	})

	t.Run("given_empty_last_query_then_does_nothing", func(t *testing.T) {
		m := &Model{
			FocusedPanel:  resultsPanel,
			LastQuery:     nil,
			Collections:   []Collection{{Name: "docs"}},
			SearchResults: []ffi.NekoSearchResult{{ID: "old", Score: 1.0}},
		}
		m.refreshSearch()

		if len(m.SearchResults) != 1 || m.SearchResults[0].ID != "old" {
			t.Fatalf("expected search results to remain unchanged, got %v", m.SearchResults)
		}
	})

	t.Run("given_no_collections_then_does_nothing", func(t *testing.T) {
		m := &Model{
			FocusedPanel:  resultsPanel,
			LastQuery:     []float32{0.1, 0.2, 0.3},
			Collections:   nil,
			SearchResults: []ffi.NekoSearchResult{{ID: "old", Score: 1.0}},
		}
		m.refreshSearch()

		if len(m.SearchResults) != 1 || m.SearchResults[0].ID != "old" {
			t.Fatalf("expected search results to remain unchanged, got %v", m.SearchResults)
		}
	})

	t.Run("given_invalid_selected_index_then_does_nothing", func(t *testing.T) {
		m := &Model{
			FocusedPanel:  resultsPanel,
			LastQuery:     []float32{0.1, 0.2, 0.3},
			Collections:   []Collection{{Name: "docs"}},
			SelectedIndex: 5,
			SearchResults: []ffi.NekoSearchResult{{ID: "old", Score: 1.0}},
		}
		m.refreshSearch()

		if len(m.SearchResults) != 1 || m.SearchResults[0].ID != "old" {
			t.Fatalf("expected search results to remain unchanged, got %v", m.SearchResults)
		}
	})
}
