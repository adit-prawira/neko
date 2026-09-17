package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultReturnsDocumentedValues(t *testing.T) {
	cfg := Default()

	expected := Config{
		Port:          3434,
		DataDirectory: "",
		LogLevel:      "info",
		WalRotateMB:   64,
		MaxSegments:   32,
		MaxDim:        4096,
	}
	if cfg != expected {
		t.Errorf("Default() = %+v, want %+v", cfg, expected)
	}
}

func TestLoadReturnsDefaultsWhenFileMissing(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "does-not-exist.toml")

	cfg, err := Load(missingPath)

	if err != nil {
		t.Fatalf("Load(missing file) error = %v, want nil", err)
	}
	if cfg.Port != 3434 || cfg.LogLevel != "info" || cfg.WalRotateMB != 64 || cfg.MaxSegments != 32 || cfg.MaxDim != 4096 {
		t.Errorf("Load(missing file) = %+v, want documented defaults", cfg)
	}
}

func TestLoadReturnsDefaultsWhenParentDirectoryMissing(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "no-such-parent")
	missingPath := filepath.Join(parent, "config.toml")

	cfg, err := Load(missingPath)

	if err != nil {
		t.Fatalf("Load(missing dir) error = %v, want nil", err)
	}
	if cfg.Port != 3434 || cfg.LogLevel != "info" || cfg.WalRotateMB != 64 || cfg.MaxSegments != 32 || cfg.MaxDim != 4096 {
		t.Errorf("Load(missing dir) = %+v, want documented defaults", cfg)
	}
}

func TestLoadReadsAllFieldsFromValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `port = 8080
data_dir = "/var/neko"
log_level = "warn"
wal_rotate_mb = 128
max_segments = 64
max_dim = 1024
`
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Load(path)

	if err != nil {
		t.Fatalf("Load(valid file) error = %v, want nil", err)
	}
	want := Config{
		Port:          8080,
		DataDirectory: "/var/neko",
		LogLevel:      "warn",
		WalRotateMB:   128,
		MaxSegments:   64,
		MaxDim:        1024,
	}
	if cfg != want {
		t.Errorf("Load(valid file) = %+v, want %+v", cfg, want)
	}
}

func TestLoadReturnsErrorForMalformedToml(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("not_valid = = toml = [ ]"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("Load(malformed file) error = nil, want parse error")
	}
}

func TestLoadRejectsInvalidPortInFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("port = 0\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("Load(port=0) error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("Load(port=0) error = %q, want it to mention 'invalid port'", err.Error())
	}
}

func TestValidateRejectsPortZero(t *testing.T) {
	cfg := Default()
	cfg.Port = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for port=0")
	}
}

func TestValidateRejectsPortAbove65535(t *testing.T) {
	cfg := Default()
	cfg.Port = 70000

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for port=70000")
	}
}

func TestValidateRejectsWalRotateMBZero(t *testing.T) {
	cfg := Default()
	cfg.WalRotateMB = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for wal_rotate_mb=0")
	}
}

func TestValidateRejectsMaxSegmentsZero(t *testing.T) {
	cfg := Default()
	cfg.MaxSegments = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for max_segments=0")
	}
}

func TestValidateRejectsMaxDimZero(t *testing.T) {
	cfg := Default()
	cfg.MaxDim = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for max_dim=0")
	}
}

func TestValidateRejectsMaxDimAbove4096(t *testing.T) {
	cfg := Default()
	cfg.MaxDim = 4097

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for max_dim=4097")
	}
}

func TestValidateRejectsUnknownLogLevel(t *testing.T) {
	cfg := Default()
	cfg.LogLevel = "fatal"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for log_level='fatal'")
	}
}

func TestResolveUsesConfigPortWhenCLIFlagUnchanged(t *testing.T) {
	cfg := Default()
	cfg.Port = 9999

	resolved, err := Resolve(cfg, ResolveDTO{Port: Property[int]{Value: 3434, IsChanged: false}})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.Port != 9999 {
		t.Errorf("resolved.Port = %d, want 9999 (from config)", resolved.Port)
	}
}

func TestResolveUsesDefaultPortWhenConfigAndCLIBothUnchanged(t *testing.T) {
	cfg := Default()

	resolved, err := Resolve(cfg, ResolveDTO{Port: Property[int]{Value: 3434, IsChanged: false}})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.Port != 3434 {
		t.Errorf("resolved.Port = %d, want 3434 (default)", resolved.Port)
	}
}

func TestResolveCLIPortOverridesConfig(t *testing.T) {
	cfg := Default()
	cfg.Port = 9999

	resolved, err := Resolve(cfg, ResolveDTO{Port: Property[int]{Value: 8080, IsChanged: true}})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.Port != 8080 {
		t.Errorf("resolved.Port = %d, want 8080 (CLI should beat config)", resolved.Port)
	}
}

func TestResolveEnvDataDirectoryOverridesConfig(t *testing.T) {
	cfg := Default()
	cfg.DataDirectory = "/from-config"

	resolved, err := Resolve(cfg, ResolveDTO{EnvDataDirectory: "/from-env"})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.DataDirectory != "/from-env" {
		t.Errorf("resolved.DataDirectory = %q, want %q (env should beat config)", resolved.DataDirectory, "/from-env")
	}
}

func TestResolveCLIDataDirectoryOverridesEnvAndConfig(t *testing.T) {
	cfg := Default()
	cfg.DataDirectory = "/from-config"

	resolved, err := Resolve(cfg, ResolveDTO{
		DataDirectory:    Property[string]{Value: "/from-cli", IsChanged: true},
		EnvDataDirectory: "/from-env",
	})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.DataDirectory != "/from-cli" {
		t.Errorf("resolved.DataDirectory = %q, want %q (CLI should beat env and config)", resolved.DataDirectory, "/from-cli")
	}
}

func TestResolveKeepsConfigWhenNoEnvOrCLIOverride(t *testing.T) {
	cfg := Default()
	cfg.DataDirectory = "/from-config"

	resolved, err := Resolve(cfg, ResolveDTO{
		DataDirectory: Property[string]{Value: "", IsChanged: false},
	})

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved.DataDirectory != "/from-config" {
		t.Errorf("resolved.DataDirectory = %q, want %q (config should be kept)", resolved.DataDirectory, "/from-config")
	}
}

func TestResolveRejectsInvalidPortFromCLI(t *testing.T) {
	cfg := Default()

	_, err := Resolve(cfg, ResolveDTO{Port: Property[int]{Value: 70000, IsChanged: true}})

	if err == nil {
		t.Fatal("Resolve() error = nil, want error for CLI port=70000")
	}
}
