package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// - r = read (4)
// - w = write (2)
// - x = execute/traverse (1)
// - - = no permission (0)

// rwxr-xr-x
const directoryPermission = 0755

// rw-r--r--
const filePermission = 0644

func DefaultTemplate() string {
	return `# neko configuration — every field is optional.
# Precedence: CLI flags > NEKO_HOME env > this file > built-in defaults.
# A missing file is treated identically to this exact content.

# HTTP port the server binds to.
port = 3434

# Data directory when neither --data-dir nor NEKO_HOME is set.
data_dir = "/var/neko"

# Log level: one of debug, info, warn, error.
log_level = "info"

# Roll the WAL when it exceeds this size in MB.
wal_rotate_mb = 64

# Maximum segments per collection.
max_segments = 32

# Maximum vector dimension per collection (capped at 4096).
max_dim = 4096
`
}

func WriteDefault(path string, shouldForceOverwrite bool) error {
	if !shouldForceOverwrite {
		_, err := os.Stat(path)
		isAlreadyExist := err == nil
		if isAlreadyExist {
			return fmt.Errorf("config: %s already exists; use --force to overwrite", path)
		}
	}

	directoryPath := filepath.Dir(path)
	if err := os.MkdirAll(directoryPath, directoryPermission); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", directoryPath, err)
	}

	if err := os.WriteFile(path, []byte(DefaultTemplate()), filePermission); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}

	return nil
}
