package tui

import (
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func TestRenderEmpty(t *testing.T) {
	t.Run("given_render_empty_then_displays_empty_message", func(t *testing.T) {
		out := renderEmpty()

		if !strings.Contains(out, "No collections yet") {
			t.Fatalf("expected empty state message, got: %s", out)
		}
	})

	t.Run("given_render_empty_then_displays_quit_hint", func(t *testing.T) {
		out := renderEmpty()

		if !strings.Contains(out, "q to quit") {
			t.Fatalf("expected quit hint, got: %s", out)
		}
	})
}

func TestRenderView(t *testing.T) {
	t.Run("given_no_collections_then_renders_empty_state", func(t *testing.T) {
		m := Model{}
		out := renderView(m)

		if !strings.Contains(out, "No collections yet") {
			t.Fatalf("expected empty state, got: %s", out)
		}
	})

	t.Run("given_one_collection_then_lists_its_name", func(t *testing.T) {
		m := Model{Collections: []Collection{{Name: "alpha"}}}
		out := renderView(m)

		if !strings.Contains(out, "alpha") {
			t.Fatalf("expected collection name, got: %s", out)
		}
	})

	t.Run("given_multiple_collections_then_lists_all_names", func(t *testing.T) {
		m := Model{Collections: []Collection{{Name: "alpha"}, {Name: "beta"}}}
		out := renderView(m)

		if !strings.Contains(out, "alpha") {
			t.Fatalf("expected alpha, got: %s", out)
		}
		if !strings.Contains(out, "beta") {
			t.Fatalf("expected beta, got: %s", out)
		}
	})
}

func TestRenderDetails(t *testing.T) {
	t.Run("given_selected_collection_then_shows_its_name", func(t *testing.T) {
		m := Model{
			Collections: []Collection{{Name: "alpha", Stats: ffi.NekoStats{Dim: 384, Metric: ffi.MetricL2, VectorCount: 10, StorageBytes: 2048}}},
		}
		out := renderDetails(m)

		if !strings.Contains(out, "alpha") {
			t.Fatalf("expected collection name, got: %s", out)
		}
	})

	t.Run("given_selected_collection_then_shows_dim", func(t *testing.T) {
		m := Model{
			Collections: []Collection{{Name: "alpha", Stats: ffi.NekoStats{Dim: 384}}},
		}
		out := renderDetails(m)

		if !strings.Contains(out, "384") {
			t.Fatalf("expected dim 384, got: %s", out)
		}
	})

	t.Run("given_selected_collection_then_shows_metric_label", func(t *testing.T) {
		m := Model{
			Collections: []Collection{{Name: "alpha", Stats: ffi.NekoStats{Metric: ffi.MetricCosine}}},
		}
		out := renderDetails(m)

		if !strings.Contains(out, "cosine") {
			t.Fatalf("expected metric label 'cosine', got: %s", out)
		}
	})

	t.Run("given_selected_collection_then_shows_vector_count", func(t *testing.T) {
		m := Model{
			Collections: []Collection{{Name: "alpha", Stats: ffi.NekoStats{VectorCount: 42}}},
		}
		out := renderDetails(m)

		if !strings.Contains(out, "42") {
			t.Fatalf("expected vector count 42, got: %s", out)
		}
	})

	t.Run("given_selected_collection_then_shows_storage_bytes", func(t *testing.T) {
		m := Model{
			Collections: []Collection{{Name: "alpha", Stats: ffi.NekoStats{StorageBytes: 6144}}},
		}
		out := renderDetails(m)

		if !strings.Contains(out, "6KB") {
			t.Fatalf("expected storage '6KB', got: %s", out)
		}
	})

	t.Run("given_invalid_selected_index_then_shows_no_collection_message", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}},
			SelectedIndex: 5,
		}
		out := renderDetails(m)

		if !strings.Contains(out, "No collection selected") {
			t.Fatalf("expected no collection message, got: %s", out)
		}
	})
}

func TestRenderCollections(t *testing.T) {
	t.Run("given_selected_index_then_renders_that_collection_name", func(t *testing.T) {
		m := Model{
			Collections:   []Collection{{Name: "alpha"}, {Name: "beta"}},
			SelectedIndex: 1,
		}
		out := renderCollections(m)

		if !strings.Contains(out, "alpha") {
			t.Fatalf("expected alpha in output, got: %s", out)
		}
		if !strings.Contains(out, "beta") {
			t.Fatalf("expected beta in output, got: %s", out)
		}
	})
}
