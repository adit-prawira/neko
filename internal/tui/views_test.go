package tui

import (
	"strings"
	"testing"
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
