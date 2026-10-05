package templates

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestDashboard(t *testing.T) {
	t.Run("given_collections_then_renders_title", func(t *testing.T) {
		model := state.Model{Collections: []state.Collection{{Name: "alpha"}}}
		output := Dashboard(model)

		if !strings.Contains(output, "neko tui") {
			t.Fatalf("expected title, got: %s", output)
		}
	})

	t.Run("given_collection_then_renders_its_name_in_collections_panel", func(t *testing.T) {
		model := state.Model{Collections: []state.Collection{{Name: "alpha"}}}
		output := Dashboard(model)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected collection name, got: %s", output)
		}
	})

	t.Run("given_search_input_then_renders_it_in_search_panel", func(t *testing.T) {
		model := state.Model{
			Collections: []state.Collection{{Name: "alpha"}},
			SearchInput: "/tmp/query.f32",
		}
		output := Dashboard(model)

		if !strings.Contains(output, "/tmp/query.f32") {
			t.Fatalf("expected search input, got: %s", output)
		}
	})

	t.Run("given_results_then_renders_them_in_results_panel", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}},
			SearchResults: []ffi.NekoSearchResult{{ID: "doc_a", Score: 1.0}},
		}
		output := Dashboard(model)

		if !strings.Contains(output, "doc_a") {
			t.Fatalf("expected result id, got: %s", output)
		}
	})

	t.Run("given_status_message_then_renders_status_bar", func(t *testing.T) {
		model := state.Model{
			Collections:   []state.Collection{{Name: "alpha"}},
			StatusMessage: "ready",
		}
		output := Dashboard(model)

		if !strings.Contains(output, "ready") {
			t.Fatalf("expected status message, got: %s", output)
		}
	})
}
