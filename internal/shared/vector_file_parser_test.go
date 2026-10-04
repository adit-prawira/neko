package shared

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVectorFile(t *testing.T) {
	t.Run("given_valid_vector_file_then_returns_parsed_floats", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "vectors.f32")

		expected := []float32{0.1, 0.2, 0.3}
		data := make([]byte, len(expected)*4)
		for i, value := range expected {
			binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(value))
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		parsed, err := ParseVectorFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parsed) != len(expected) {
			t.Fatalf("expected %d floats, got %d", len(expected), len(parsed))
		}
		for i := range expected {
			if parsed[i] != expected[i] {
				t.Fatalf("expected float %d to be %v, got %v", i, expected[i], parsed[i])
			}
		}
	})

	t.Run("given_file_with_invalid_size_then_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.f32")
		if err := os.WriteFile(path, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		parsed, err := ParseVectorFile(path)
		if err == nil {
			t.Fatalf("expected error for invalid size, got %v", parsed)
		}
	})

	t.Run("given_missing_file_then_returns_error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.f32")

		parsed, err := ParseVectorFile(path)
		if err == nil {
			t.Fatalf("expected error for missing file, got %v", parsed)
		}
	})
}
