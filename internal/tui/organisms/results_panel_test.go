package organisms

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func TestResultsPanel(t *testing.T) {
	t.Run("given_no_results_then_renders_no_search_message", func(t *testing.T) {
		model := state.Model{}
		output := ResultsPanel(model)

		if !strings.Contains(output, "no search yet") {
			t.Fatalf("expected no search message, got: %s", output)
		}
	})

	t.Run("given_results_then_renders_ids_and_scores", func(t *testing.T) {
		model := state.Model{
			SearchResults: []ffi.NekoSearchResult{
				{ID: "doc_a", Score: 1.5},
				{ID: "doc_b", Score: 2.5},
			},
		}
		output := ResultsPanel(model)

		if !strings.Contains(output, "doc_a") {
			t.Fatalf("expected doc_a, got: %s", output)
		}
		if !strings.Contains(output, "doc_b") {
			t.Fatalf("expected doc_b, got: %s", output)
		}
		if !strings.Contains(output, "1.5000") {
			t.Fatalf("expected score 1.5000, got: %s", output)
		}
	})

	t.Run("given_multiple_results_then_each_row_is_on_its_own_line", func(t *testing.T) {
		model := state.Model{
			SearchResults: []ffi.NekoSearchResult{
				{ID: "doc_a", Score: 1.0},
				{ID: "doc_b", Score: 2.0},
			},
		}
		output := ResultsPanel(model)

		if !strings.Contains(output, "doc_a\t1.0000\n") {
			t.Fatalf("expected doc_a row ending with newline, got: %s", output)
		}
		if !strings.Contains(output, "doc_b\t2.0000") {
			t.Fatalf("expected doc_b row, got: %s", output)
		}
	})
}
