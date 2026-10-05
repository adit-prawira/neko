package state

import "testing"

func TestIsSearchPanelFocused(t *testing.T) {
	t.Run("given_search_panel_focused_then_returns_true", func(t *testing.T) {
		model := Model{FocusedPanel: SearchPanel}

		if !model.IsSearchPanelFocused() {
			t.Fatal("expected search panel to be focused")
		}
	})

	t.Run("given_collection_panel_focused_then_returns_false", func(t *testing.T) {
		model := Model{FocusedPanel: CollectionPanel}

		if model.IsSearchPanelFocused() {
			t.Fatal("expected search panel not to be focused")
		}
	})
}

func TestIsCollectionPanelFocused(t *testing.T) {
	t.Run("given_collection_panel_focused_then_returns_true", func(t *testing.T) {
		model := Model{FocusedPanel: CollectionPanel}

		if !model.IsCollectionPanelFocused() {
			t.Fatal("expected collection panel to be focused")
		}
	})
}

func TestIsResultsPanelFocused(t *testing.T) {
	t.Run("given_results_panel_focused_then_returns_true", func(t *testing.T) {
		model := Model{FocusedPanel: ResultsPanel}

		if !model.IsResultsPanelFocused() {
			t.Fatal("expected results panel to be focused")
		}
	})
}
