package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
	t.Run("given_model_then_returns_no_command", func(t *testing.T) {
		m := Model{}
		if cmd := m.Init(); cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
	})
}
