package tui

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestRender(t *testing.T) {
	t.Run("given_no_collections_then_renders_empty_state", func(t *testing.T) {
		model := state.Model{}
		output := Render(model)

		if !strings.Contains(output, "No collections yet") {
			t.Fatalf("expected empty state message, got: %s", output)
		}
	})

	t.Run("given_one_collection_then_renders_its_name", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{{Name: "alpha"}},
		}
		output := Render(model)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected collection name, got: %s", output)
		}
	})

	t.Run("given_multiple_collections_then_renders_all_names", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{
				{Name: "alpha"},
				{Name: "beta"},
			},
		}
		output := Render(model)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected alpha, got: %s", output)
		}
		if !strings.Contains(output, "beta") {
			t.Fatalf("expected beta, got: %s", output)
		}
	})
}
