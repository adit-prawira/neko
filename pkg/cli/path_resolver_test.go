package cli

import (
	"path/filepath"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func TestResolveDataDirectoryUsesDataDirFlag(t *testing.T) {
	resetDataDirectory(t)
	dataDirectory = "/from/flag"

	if got := resolveDataDirectory(); got != "/from/flag" {
		t.Errorf("resolveDataDirectory() = %q, want %q", got, "/from/flag")
	}
}

func TestResolveDataDirectoryFallsBackToFFIDefault(t *testing.T) {
	resetDataDirectory(t)

	if got := resolveDataDirectory(); got != ffi.DefaultDataDirectory() {
		t.Errorf("resolveDataDirectory() = %q, want ffi.DefaultDataDirectory() = %q", got, ffi.DefaultDataDirectory())
	}
}

func TestResolveConfigPathJoinsConfigToml(t *testing.T) {
	resetDataDirectory(t)
	dataDirectory = "/x"

	want := filepath.Join("/x", "config.toml")
	if got := resolveConfigPath(); got != want {
		t.Errorf("resolveConfigPath() = %q, want %q", got, want)
	}
}
