package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	DefaultPort        = 3434
	DefaultLogLevel    = "info"
	DefaultWalRotateMB = uint64(64)
	DefaultMaxSegments = uint32(32)
	DefaultMaxDim      = uint32(4096)
)

var allowedLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

type Config struct {
	Port          int    `toml:"port"`
	DataDirectory string `toml:"data_dir"`
	LogLevel      string `toml:"log_level"`
	WalRotateMB   uint64 `toml:"wal_rotate_mb"`
	MaxSegments   uint32 `toml:"max_segments"`
	MaxDim        uint32 `toml:"max_dim"`
}

func Default() Config {
	return Config{
		Port:          DefaultPort,
		DataDirectory: "",
		LogLevel:      DefaultLogLevel,
		WalRotateMB:   DefaultWalRotateMB,
		MaxSegments:   DefaultMaxSegments,
		MaxDim:        DefaultMaxDim,
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if !c.isValidPort() {
		return fmt.Errorf("config: invalid port %d: must be in [1, 65535]", c.Port)
	}

	if !c.isValidWalRotateMB() {
		return fmt.Errorf("config: invalid wal_rotate_mb %d: must be > 0", c.WalRotateMB)
	}

	if !c.isValidMaxSegments() {
		return fmt.Errorf("config: invalid max_segments %d: must be > 0", c.MaxSegments)
	}

	if !c.isValidMaxDim() {
		return fmt.Errorf("config: invalid max_dim %d: must be [1, 4096]", c.MaxDim)
	}

	if !c.isValidLogLevel() {
		return fmt.Errorf("config: invalid log_level %q: must be one debug, info, warn, or error", c.LogLevel)
	}
	return nil
}

type Property[T any] struct {
	Value     T
	IsChanged bool
}

type ResolveDTO struct {
	Port             Property[int]
	DataDirectory    Property[string]
	EnvDataDirectory string
}

func (r ResolveDTO) shouldResolvePort() bool {
	return r.Port.IsChanged
}

func (r ResolveDTO) shouldResolveEnvDataDirectory() bool {
	return r.EnvDataDirectory != ""
}

func (r ResolveDTO) shouldResolveDataDirectory() bool {
	return r.DataDirectory.IsChanged && r.DataDirectory.Value != ""
}

func Resolve(c Config, resolveDTO ResolveDTO) (Config, error) {
	resolved := c

	if resolveDTO.shouldResolveEnvDataDirectory() {
		resolved.DataDirectory = resolveDTO.EnvDataDirectory
	}

	if resolveDTO.shouldResolvePort() {
		resolved.Port = resolveDTO.Port.Value
	}

	if resolveDTO.shouldResolveDataDirectory() {
		resolved.DataDirectory = resolveDTO.DataDirectory.Value
	}

	if err := resolved.Validate(); err != nil {
		return Config{}, err
	}

	return resolved, nil
}

func (c Config) isValidPort() bool {
	return c.Port >= 1 && c.Port <= 65535
}

func (c Config) isValidWalRotateMB() bool {
	return c.WalRotateMB > 0
}

func (c Config) isValidMaxSegments() bool {
	return c.MaxSegments > 0
}

func (c Config) isValidMaxDim() bool {
	return c.MaxDim >= 1 && c.MaxDim <= 4096
}

func (c Config) isValidLogLevel() bool {
	_, ok := allowedLogLevels[c.LogLevel]
	return ok
}
