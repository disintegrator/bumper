package workspace

import (
	"fmt"
	"path/filepath"
)

func Dir(base string) string {
	return filepath.Join(base, ".bumper")
}

func ConfigFilename(base string) string {
	return filepath.Join(Dir(base), "config.toml")
}

func VersionFilename(base string) string {
	return filepath.Join(Dir(base), "versions.toml")
}

func BumpFilename(base string, suffix string) string {
	return filepath.Join(Dir(base), fmt.Sprintf("bump-%s.md", suffix))
}
