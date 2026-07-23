package cli

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func cliSetup(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "neko_test_cli")
	os.MkdirAll(dir, 0755)
	os.Setenv("NEKO_HOME", dir)
	if err := ffi.Init(dir); err != nil {
		t.Fatalf("engine init failed: %v", err)
	}
	return dir
}

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "neko v0.1.0") {
		t.Errorf("expected version output, got %q", output)
	}
}

func TestCreateCommand(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"create", "test_create", "--dim", "384", "--metric", "cosine"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "collection 'test_create' created") {
		t.Errorf("expected create confirmation, got %q", output)
	}

	ffi.Drop("test_create")
}

func TestCreateCommandMissingDim(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)

	// dim is required, running without it should error
	cmd.SetArgs([]string{"create", "test_missing_dim"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --dim flag")
	}
}

func TestCreateCommandInvalidMetric(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()

	cmd.SetArgs([]string{"create", "test_bad_metric", "--dim", "384", "--metric", "euclidean"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid metric")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid metric") {
		t.Errorf("expected metric error, got: %v", err)
	}
}

func TestListCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_list_cli")
	if err := ffi.Create("test_list_cli", 256, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test_list_cli") {
		t.Errorf("list output missing collection, got: %q", output)
	}
}

func TestDropCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_drop_cli")
	if err := ffi.Create("test_drop_cli", 128, ffi.MetricDot, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"drop", "test_drop_cli"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drop command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "dropped") {
		t.Errorf("expected drop confirmation, got: %q", output)
	}
}

func TestDropCommandNonexistent(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()

	cmd.SetArgs([]string{"drop", "nonexistent_cli"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for dropping nonexistent collection")
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "neko") {
		t.Errorf("help missing command name, got: %q", output)
	}
}

func writeRawF32(path string, values []float32) error {
	data := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	return os.WriteFile(path, data, 0644)
}

func TestInsertCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_insert_tc")
	if err := ffi.Create("cli_test_insert_tc", 3, ffi.MetricCosine, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	tmpFile := filepath.Join(dir, "vector.f32")
	if err := writeRawF32(tmpFile, []float32{0.5, 0.6, 0.7}); err != nil {
		t.Fatalf("write vector file: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"insert", "cli_test_insert_tc", "--id", "doc1", "--file", tmpFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("insert command failed: %v", err)
	}

	vector, err := ffi.Get("cli_test_insert_tc", "doc1", 3)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if vector[0] != 0.5 || vector[1] != 0.6 || vector[2] != 0.7 {
		t.Errorf("vector mismatch: got [%v %v %v]", vector[0], vector[1], vector[2])
	}
}

func TestInsertCommandMissingFile(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"insert", "some_collection", "--id", "doc1", "--file", "/nonexistent/vector.f32"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestGetCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_get_tc")
	if err := ffi.Create("cli_test_get_tc", 3, ffi.MetricCosine, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := ffi.Insert("cli_test_get_tc", "doc1", []float32{0.1, 0.2, 0.3}, ""); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "cli_test_get_tc", "doc1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0.1") || !strings.Contains(output, "0.2") || !strings.Contains(output, "0.3") {
		t.Errorf("get output missing vector values: %q", output)
	}
}

func TestGetCommandNonexistentId(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_getnf_tc")
	if err := ffi.Create("cli_test_getnf_tc", 3, ffi.MetricCosine, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"get", "cli_test_getnf_tc", "no_such_doc"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent id")
	}
}
