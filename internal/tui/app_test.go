package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestInitialModel(t *testing.T) {
	t.Run("given_initial_model_then_focuses_collection_panel", func(t *testing.T) {
		model := initialModel()

		if model.state.FocusedPanel != state.CollectionPanel {
			t.Fatalf("expected collection panel, got %d", model.state.FocusedPanel)
		}
	})

	t.Run("given_initial_model_then_selected_index_is_zero", func(t *testing.T) {
		model := initialModel()

		if model.state.SelectedIndex != 0 {
			t.Fatalf("expected SelectedIndex=0, got %d", model.state.SelectedIndex)
		}
	})
}

func TestModelWrapper(t *testing.T) {
	t.Run("given_wrapper_then_init_returns_tick_command", func(t *testing.T) {
		wrapper := initialModel()
		cmd := wrapper.Init()

		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
	})

	t.Run("given_wrapper_then_update_delegates_and_returns_wrapper", func(t *testing.T) {
		wrapper := initialModel()
		updated, cmd := wrapper.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))

		if cmd == nil {
			t.Fatal("expected non-nil command, got nil")
		}
		if _, ok := updated.(model); !ok {
			t.Fatalf("expected model wrapper, got %T", updated)
		}
	})

	t.Run("given_wrapper_then_view_returns_non_empty_content", func(t *testing.T) {
		wrapper := initialModel()
		view := wrapper.View()

		if view.Content == "" {
			t.Fatal("expected non-empty view content")
		}
	})

	t.Run("given_wrapper_then_view_uses_alt_screen", func(t *testing.T) {
		wrapper := initialModel()
		view := wrapper.View()

		if !view.AltScreen {
			t.Fatal("expected AltScreen true")
		}
	})
}
