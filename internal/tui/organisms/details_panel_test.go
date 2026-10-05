package organisms

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestDetailsPanel(t *testing.T) {
	t.Run("given_selected_collection_then_renders_name", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{{Name: "alpha", Stats: ffi.NekoStats{Dim: 384}}},
		}
		output := DetailsPanel(model)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected name 'alpha', got: %s", output)
		}
	})

	t.Run("given_selected_collection_then_renders_dim", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{{Name: "alpha", Stats: ffi.NekoStats{Dim: 384}}},
		}
		output := DetailsPanel(model)

		if !strings.Contains(output, "384") {
			t.Fatalf("expected dim 384, got: %s", output)
		}
	})

	t.Run("given_selected_collection_then_renders_metric_label", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{{Name: "alpha", Stats: ffi.NekoStats{Metric: ffi.MetricCosine}}},
		}
		output := DetailsPanel(model)

		if !strings.Contains(output, "cosine") {
			t.Fatalf("expected metric label 'cosine', got: %s", output)
		}
	})

	t.Run("given_invalid_selected_index_then_renders_no_collection_message", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}},
			SelectedIndex: 5,
		}
		output := DetailsPanel(model)

		if !strings.Contains(output, "No collection selected") {
			t.Fatalf("expected no collection message, got: %s", output)
		}
	})
}
