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

func PrereleaseFilename(base string) string {
	return filepath.Join(Dir(base), "prerelease.toml")
}

func PrereleaseBumpDir(base string) string {
	return filepath.Join(Dir(base), "prerelease")
}

func PrereleaseBumpFilename(base string, suffix string) string {
	return filepath.Join(PrereleaseBumpDir(base), fmt.Sprintf("bump-%s.md", suffix))
}
