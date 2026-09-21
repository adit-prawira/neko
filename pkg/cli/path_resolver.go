package cli

import (
	"path/filepath"

	"github.com/adit-prawira/neko/internal/ffi"
)

var dataDirectory string

func resolveDataDirectory() string {
	if dataDirectory != "" {
		return dataDirectory
	}

	return ffi.DefaultDataDirectory()
}

func resolveConfigPath() string {
	return filepath.Join(resolveDataDirectory(), "config.toml")
}
