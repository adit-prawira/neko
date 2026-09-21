package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultTemplateContainsAllDocumentedDefaults(t *testing.T) {
	template := DefaultTemplate()

	expectedSubstrings := []string{
		"port = 3434",
		`data_dir = "/var/neko"`,
		`log_level = "info"`,
		"wal_rotate_mb = 64",
		"max_segments = 32",
		"max_dim = 4096",
		"Precedence:",
		"NEKO_HOME",
	}
	for _, expected := range expectedSubstrings {
		if !strings.Contains(template, expected) {
			t.Errorf("DefaultTemplate() missing %q, got:\n%s", expected, template)
		}
	}
}

func TestWriteDefaultCreatesFileInExistingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	if err := WriteDefault(path, false); err != nil {
		t.Fatalf("WriteDefault returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(got, []byte(DefaultTemplate())) {
		t.Errorf("written file contents do not match DefaultTemplate();\nwant:\n%s\ngot:\n%s", DefaultTemplate(), string(got))
	}
}

func TestWriteDefaultRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	originalContents := []byte("# existing user config — must not be clobbered\nport = 9999\n")
	if err := os.WriteFile(path, originalContents, filePermission); err != nil {
		t.Fatalf("WriteFile setup failed: %v", err)
	}

	err := WriteDefault(path, false)
	if err == nil {
		t.Fatal("WriteDefault returned nil error; want error when file exists and force=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("WriteDefault error %q does not mention 'already exists'", err.Error())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(got, originalContents) {
		t.Errorf("existing file was modified despite refusal;\nwant:\n%s\ngot:\n%s", originalContents, got)
	}
}

func TestWriteDefaultOverwritesWithForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	originalContents := []byte("# existing user config — must be replaced\nport = 9999\n")
	if err := os.WriteFile(path, originalContents, filePermission); err != nil {
		t.Fatalf("WriteFile setup failed: %v", err)
	}

	if err := WriteDefault(path, true); err != nil {
		t.Fatalf("WriteDefault(path, true) returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(got, []byte(DefaultTemplate())) {
		t.Errorf("file was not overwritten with template;\nwant:\n%s\ngot:\n%s", DefaultTemplate(), string(got))
	}
}

func TestWriteDefaultCreatesMissingParentDirs(t *testing.T) {
	deepPath := filepath.Join(t.TempDir(), "deep", "nested", "dir", "config.toml")
	parentDir := filepath.Dir(deepPath)

	if _, err := os.Stat(parentDir); !os.IsNotExist(err) {
		t.Fatalf("precondition failed: parent dir %s should not exist; stat err = %v", parentDir, err)
	}

	if err := WriteDefault(deepPath, false); err != nil {
		t.Fatalf("WriteDefault returned error: %v", err)
	}

	if _, err := os.Stat(parentDir); err != nil {
		t.Errorf("parent dir %s was not created: %v", parentDir, err)
	}
	if _, err := os.Stat(deepPath); err != nil {
		t.Errorf("config.toml at %s was not created: %v", deepPath, err)
	}
}
