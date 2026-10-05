package molecules

import (
	"strings"
	"testing"
)

func TestResultRow(t *testing.T) {
	t.Run("given_result_then_renders_id_and_score", func(t *testing.T) {
		output := ResultRow("doc_a", 1.5)

		if !strings.Contains(output, "doc_a") {
			t.Fatalf("expected id 'doc_a', got: %s", output)
		}
		if !strings.Contains(output, "1.5000") {
			t.Fatalf("expected score '1.5000', got: %s", output)
		}
	})

	t.Run("given_result_then_ends_with_newline", func(t *testing.T) {
		output := ResultRow("doc_b", 2.0)

		if !strings.HasSuffix(output, "\n") {
			t.Fatalf("expected newline suffix, got: %s", output)
		}
	})
}
