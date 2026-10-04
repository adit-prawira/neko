package tui

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseQuery(t *testing.T) {
	t.Run("given_comma_separated_floats_then_returns_vector", func(t *testing.T) {
		input := "0.1,0.2,0.3"
		query, err := ParseQuery(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(query) != 3 {
			t.Fatalf("expected 3 floats, got %d", len(query))
		}
		expected := []float32{0.1, 0.2, 0.3}
		for i := range expected {
			if query[i] != expected[i] {
				t.Fatalf("expected float %d to be %v, got %v", i, expected[i], query[i])
			}
		}
	})

	t.Run("given_space_separated_floats_then_returns_vector", func(t *testing.T) {
		input := "0.1 0.2 0.3"
		query, err := ParseQuery(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(query) != 3 {
			t.Fatalf("expected 3 floats, got %d", len(query))
		}
	})

	t.Run("given_mixed_comma_and_space_floats_then_returns_vector", func(t *testing.T) {
		input := "0.1, 0.2, 0.3"
		query, err := ParseQuery(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(query) != 3 {
			t.Fatalf("expected 3 floats, got %d", len(query))
		}
	})

	t.Run("given_file_path_then_reads_vector_from_file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "query.f32")

		expected := []float32{0.1, 0.2, 0.3}
		data := make([]byte, len(expected)*4)
		for i, value := range expected {
			binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(value))
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		query, err := ParseQuery(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(query) != 3 {
			t.Fatalf("expected 3 floats, got %d", len(query))
		}
	})

	t.Run("given_invalid_float_then_returns_error", func(t *testing.T) {
		input := "0.1,abc,0.3"
		query, err := ParseQuery(input)

		if err == nil {
			t.Fatalf("expected error, got %v", query)
		}
	})

	t.Run("given_empty_input_then_returns_error", func(t *testing.T) {
		input := "   "
		query, err := ParseQuery(input)

		if err == nil {
			t.Fatalf("expected error, got %v", query)
		}
	})

	t.Run("given_missing_file_path_then_returns_error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.f32")
		query, err := ParseQuery(path)

		if err == nil {
			t.Fatalf("expected error, got %v", query)
		}
	})
}
