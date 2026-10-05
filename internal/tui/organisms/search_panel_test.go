package organisms

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestSearchPanel(t *testing.T) {
	t.Run("given_search_focused_then_renders_input_and_hint", func(t *testing.T) {
		model := state.Model{
			FocusedPanel: state.SearchPanel,
			SearchInput:  "/tmp/query.f32",
		}
		output := SearchPanel(model)

		if !strings.Contains(output, "/tmp/query.f32") {
			t.Fatalf("expected search input, got: %s", output)
		}
		if !strings.Contains(output, "enter to search") {
			t.Fatalf("expected search hint, got: %s", output)
		}
	})

	t.Run("given_search_not_focused_then_renders_input_without_cursor", func(t *testing.T) {
		model := state.Model{
			FocusedPanel: state.CollectionPanel,
			SearchInput:  "/tmp/query.f32",
		}
		output := SearchPanel(model)

		if !strings.Contains(output, "/tmp/query.f32") {
			t.Fatalf("expected search input, got: %s", output)
		}
	})
}
