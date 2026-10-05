package templates

import (
	"strings"
	"testing"
)

func TestEmpty(t *testing.T) {
	t.Run("given_empty_state_then_renders_no_collections_message", func(t *testing.T) {
		output := Empty()

		if !strings.Contains(output, "No collections yet") {
			t.Fatalf("expected empty state message, got: %s", output)
		}
	})

	t.Run("given_empty_state_then_renders_quit_hint", func(t *testing.T) {
		output := Empty()

		if !strings.Contains(output, "q to quit") {
			t.Fatalf("expected quit hint, got: %s", output)
		}
	})
}
