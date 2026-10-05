package organisms

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestCollectionsPanel(t *testing.T) {
	t.Run("given_collections_then_renders_all_names", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{
				{Name: "alpha"},
				{Name: "beta"},
			},
		}
		output := CollectionsPanel(model)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected alpha, got: %s", output)
		}
		if !strings.Contains(output, "beta") {
			t.Fatalf("expected beta, got: %s", output)
		}
	})

	t.Run("given_selected_index_then_renders_that_collection", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		output := CollectionsPanel(model)

		if !strings.Contains(output, "beta") {
			t.Fatalf("expected beta in output, got: %s", output)
		}
	})
}
