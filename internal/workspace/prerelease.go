package workspace

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
)

// PrereleaseGroupState represents the prerelease state for a single release group
type PrereleaseGroupState struct {
	Tag         string `toml:"tag"`
	FromVersion string `toml:"from_version"`
	Counter     int    `toml:"counter"`
}

// PrereleaseState represents the prerelease state for all groups
type PrereleaseState struct {
	Groups map[string]*PrereleaseGroupState `toml:"groups"`
}

// NewPrereleaseState creates a new empty prerelease state
func NewPrereleaseState() *PrereleaseState {
	return &PrereleaseState{
		Groups: make(map[string]*PrereleaseGroupState),
	}
}

// LoadPrereleaseState loads the prerelease state from the workspace
func LoadPrereleaseState(baseDir string) (*PrereleaseState, error) {
	filename := PrereleaseFilename(baseDir)

	_, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return NewPrereleaseState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat prerelease file: %w", err)
	}

	var state PrereleaseState
	_, err = toml.DecodeFile(filename, &state)
	if err != nil {
		return nil, fmt.Errorf("decode prerelease file: %w", err)
	}

	if state.Groups == nil {
		state.Groups = make(map[string]*PrereleaseGroupState)
	}

	return &state, nil
}

// SavePrereleaseState saves the prerelease state to the workspace
func SavePrereleaseState(baseDir string, state *PrereleaseState) error {
	filename := PrereleaseFilename(baseDir)

	// If no groups in prerelease, remove the file
	if len(state.Groups) == 0 {
		err := os.Remove(filename)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove prerelease file: %w", err)
		}
		return nil
	}

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open prerelease file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(state); err != nil {
		return fmt.Errorf("encode prerelease file: %w", err)
	}

	return nil
}

// IsInPrerelease checks if a group is in prerelease mode
func (s *PrereleaseState) IsInPrerelease(groupName string) bool {
	_, ok := s.Groups[groupName]
	return ok
}

// GetGroupState returns the prerelease state for a group, or nil if not in prerelease
func (s *PrereleaseState) GetGroupState(groupName string) *PrereleaseGroupState {
	return s.Groups[groupName]
}

// EnterPrerelease enters prerelease mode for a group
func (s *PrereleaseState) EnterPrerelease(groupName string, tag string, fromVersion string) {
	existing := s.Groups[groupName]
	if existing != nil && existing.Tag == tag {
		// Same tag, keep counter
		return
	}

	s.Groups[groupName] = &PrereleaseGroupState{
		Tag:         tag,
		FromVersion: fromVersion,
		Counter:     0, // Will be set to 1 on first commit
	}
}

// ExitPrerelease exits prerelease mode for a group
func (s *PrereleaseState) ExitPrerelease(groupName string) {
	delete(s.Groups, groupName)
}

// IncrementCounter increments the prerelease counter for a group
func (s *PrereleaseState) IncrementCounter(groupName string) {
	if state := s.Groups[groupName]; state != nil {
		state.Counter++
	}
}

// ResetCounter resets the prerelease counter for a group (used when base version escalates)
func (s *PrereleaseState) ResetCounter(groupName string) {
	if state := s.Groups[groupName]; state != nil {
		state.Counter = 1
	}
}

// SetCounter sets the prerelease counter for a group
func (s *PrereleaseState) SetCounter(groupName string, counter int) {
	if state := s.Groups[groupName]; state != nil {
		state.Counter = counter
	}
}

// CalculatePrereleaseVersion calculates the next prerelease version based on accumulated and pending bumps
func CalculatePrereleaseVersion(fromVersion string, tag string, currentCounter int, accumulatedLevel BumpLevel, pendingLevel BumpLevel) (string, int, error) {
	fromSemver, err := semver.NewVersion(fromVersion)
	if err != nil {
		return "", 0, fmt.Errorf("parse from_version: %w", err)
	}

	// Take the highest of accumulated and pending levels
	highestLevel := max(accumulatedLevel, pendingLevel)

	// Calculate the base version by applying highest level to from_version
	var baseVersion semver.Version
	switch highestLevel {
	case BumpLevelMajor:
		baseVersion = fromSemver.IncMajor()
	case BumpLevelMinor:
		baseVersion = fromSemver.IncMinor()
	case BumpLevelPatch:
		baseVersion = fromSemver.IncPatch()
	default:
		// If no bump level, use from_version + patch as default
		baseVersion = fromSemver.IncPatch()
	}

	// Calculate what the previous base version was (if any)
	var prevBaseVersion *semver.Version
	if currentCounter > 0 {
		// We had a previous prerelease, calculate what its base was
		switch accumulatedLevel {
		case BumpLevelMajor:
			v := fromSemver.IncMajor()
			prevBaseVersion = &v
		case BumpLevelMinor:
			v := fromSemver.IncMinor()
			prevBaseVersion = &v
		case BumpLevelPatch:
			v := fromSemver.IncPatch()
			prevBaseVersion = &v
		}
	}

	// Determine the new counter
	newCounter := 1
	if prevBaseVersion != nil && prevBaseVersion.Equal(&baseVersion) {
		// Base version unchanged, increment counter
		newCounter = currentCounter + 1
	}

	// Create the prerelease version string
	prereleaseVersion := fmt.Sprintf("%d.%d.%d-%s.%d",
		baseVersion.Major(),
		baseVersion.Minor(),
		baseVersion.Patch(),
		tag,
		newCounter,
	)

	return prereleaseVersion, newCounter, nil
}

// GetStableVersionFromPrerelease extracts the stable version from a prerelease version
func GetStableVersionFromPrerelease(prereleaseVersion string) (string, error) {
	v, err := semver.NewVersion(prereleaseVersion)
	if err != nil {
		return "", fmt.Errorf("parse prerelease version: %w", err)
	}

	return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch()), nil
}
