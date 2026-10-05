package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func keyPress(code rune) tea.Msg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func keyPressChar(char rune) tea.Msg {
	return tea.KeyPressMsg(tea.Key{Code: char})
}

func TestUpdate(t *testing.T) {
	t.Run("given_q_key_when_collection_focused_then_returns_quit_command", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.CollectionPanel}
		_, cmd := Update(model, keyPressChar('q'))

		if cmd == nil {
			t.Fatal("expected quit command, got nil")
		}
	})

	t.Run("given_esc_key_when_collection_focused_then_returns_quit_command", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.CollectionPanel}
		_, cmd := Update(model, keyPress(tea.KeyEsc))

		if cmd == nil {
			t.Fatal("expected quit command, got nil")
		}
	})

	t.Run("given_q_key_when_search_focused_then_appends_to_input", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.SearchPanel}
		updated, cmd := Update(model, keyPressChar('q'))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SearchInput != "q" {
			t.Fatalf("expected SearchInput 'q', got %q", updated.SearchInput)
		}
	})

	t.Run("given_esc_key_when_search_focused_then_returns_to_collections", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.SearchPanel}
		updated, cmd := Update(model, keyPress(tea.KeyEsc))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.FocusedPanel != state.CollectionPanel {
			t.Fatalf("expected collection panel, got %d", updated.FocusedPanel)
		}
	})

	t.Run("given_slash_key_when_collection_focused_then_focuses_search", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.CollectionPanel}
		updated, cmd := Update(model, keyPressChar('/'))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.FocusedPanel != state.SearchPanel {
			t.Fatalf("expected search panel, got %d", updated.FocusedPanel)
		}
	})

	t.Run("given_slash_key_when_search_focused_then_appends_to_input", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.SearchPanel}
		updated, cmd := Update(model, keyPressChar('/'))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SearchInput != "/" {
			t.Fatalf("expected SearchInput '/', got %q", updated.SearchInput)
		}
	})

	t.Run("given_down_key_then_increments_selected_index", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 0,
		}
		updated, cmd := Update(model, keyPress(tea.KeyDown))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SelectedIndex != 1 {
			t.Fatalf("expected SelectedIndex=1, got %d", updated.SelectedIndex)
		}
	})

	t.Run("given_up_key_then_decrements_selected_index", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		updated, cmd := Update(model, keyPress(tea.KeyUp))

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SelectedIndex != 0 {
			t.Fatalf("expected SelectedIndex=0, got %d", updated.SelectedIndex)
		}
	})

	t.Run("given_down_key_at_last_index_then_stays_at_last", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		updated, _ := Update(model, keyPress(tea.KeyDown))

		if updated.SelectedIndex != 1 {
			t.Fatalf("expected SelectedIndex=1, got %d", updated.SelectedIndex)
		}
	})

	t.Run("given_up_key_at_first_index_then_stays_at_first", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 0,
		}
		updated, _ := Update(model, keyPress(tea.KeyUp))

		if updated.SelectedIndex != 0 {
			t.Fatalf("expected SelectedIndex=0, got %d", updated.SelectedIndex)
		}
	})

	t.Run("given_tab_key_then_cycles_to_next_panel", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.CollectionPanel}
		updated, _ := Update(model, keyPress(tea.KeyTab))

		if updated.FocusedPanel != state.SearchPanel {
			t.Fatalf("expected search panel, got %d", updated.FocusedPanel)
		}
	})

	t.Run("given_shift_tab_key_then_cycles_to_previous_panel", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.SearchPanel}
		updated, _ := Update(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))

		if updated.FocusedPanel != state.CollectionPanel {
			t.Fatalf("expected collection panel, got %d", updated.FocusedPanel)
		}
	})

	t.Run("given_backspace_key_when_search_focused_then_removes_last_character", func(t *testing.T) {
		model := state.Model{
			FocusedPanel: state.SearchPanel,
			SearchInput:  "abc",
		}
		updated, _ := Update(model, keyPress(tea.KeyBackspace))

		if updated.SearchInput != "ab" {
			t.Fatalf("expected SearchInput 'ab', got %q", updated.SearchInput)
		}
	})
}

func TestUpdatePaste(t *testing.T) {
	t.Run("given_paste_message_when_search_focused_then_appends_text", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.SearchPanel}
		updated, cmd := Update(model, tea.PasteMsg{Content: "/tmp/query.f32"})

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SearchInput != "/tmp/query.f32" {
			t.Fatalf("expected SearchInput '/tmp/query.f32', got %q", updated.SearchInput)
		}
	})

	t.Run("given_paste_message_when_collection_focused_then_ignored", func(t *testing.T) {
		model := state.Model{FocusedPanel: state.CollectionPanel}
		updated, cmd := Update(model, tea.PasteMsg{Content: "ignored"})

		if cmd != nil {
			t.Fatalf("expected nil command, got %T", cmd)
		}
		if updated.SearchInput != "" {
			t.Fatalf("expected empty SearchInput, got %q", updated.SearchInput)
		}
	})
}
