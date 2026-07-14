package cli

import (
	"bytes"
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
