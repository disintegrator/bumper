package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func GetWd(start string) (wd string, err error) {
	dir, err := filepath.Abs(filepath.Clean(start))
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}

	for {
		bumperDir := Dir(dir)
		info, err := os.Stat(bumperDir)
		switch {
		case errors.Is(err, os.ErrNotExist):
			// continue walking up
		case err != nil:
			return "", fmt.Errorf("stat: %s: %w", bumperDir, err)
		default:
			if info.IsDir() {
				return dir, nil
			}
			// .bumper exists but is not a directory. Keep walking up.
		}

		parent := filepath.Dir(dir)
		if parent == dir || dir == "" {
			break // Reached root
		}
		dir = parent
	}

	return "", errors.New(".bumper directory not found")
}

func Initialize(baseDir string) error {
	dir := Dir(baseDir)
	configFilename := ConfigFilename(baseDir)

	err := os.MkdirAll(dir, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("mkdir: %s: %w", dir, err)
	}

	var cfg Config
	f, err := os.OpenFile(configFilename, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	switch {
	case errors.Is(err, os.ErrExist):
		return nil
	case err != nil:
		return fmt.Errorf("open: %s: %w", configFilename, err)
	default:
		// proceed
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("write: %s: %w", configFilename, err)
	}

	return nil
}

func LoadConfig(baseDir string) (*Config, error) {
	configFilename := ConfigFilename(baseDir)

	var cfg Config
	_, err := toml.DecodeFile(configFilename, &cfg)
	if err != nil {
		return nil, fmt.Errorf("decode: %s: %w", configFilename, err)
	}

	return &cfg, nil
}

func SaveConfig(baseDir string, cfg *Config) error {
	configFilename := ConfigFilename(baseDir)

	f, err := os.OpenFile(configFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open: %s: %w", configFilename, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("write: %s: %w", configFilename, err)
	}

	return nil
}
