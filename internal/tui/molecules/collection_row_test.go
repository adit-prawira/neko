package molecules

import (
	"strings"
	"testing"
)

func TestCollectionRow(t *testing.T) {
	t.Run("given_selected_row_then_renders_name_with_selected_style", func(t *testing.T) {
		output := CollectionRow("alpha", true)

		if !strings.Contains(output, "alpha") {
			t.Fatalf("expected name 'alpha', got: %s", output)
		}
	})

	t.Run("given_unselected_row_then_renders_name_plain", func(t *testing.T) {
		output := CollectionRow("beta", false)

		if output != "beta" {
			t.Fatalf("expected plain 'beta', got: %s", output)
		}
	})
}
