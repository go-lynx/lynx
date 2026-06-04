package plugins

import (
	"fmt"
	"regexp"
	"strings"
)

// Package id provides plugin ID format helpers. The format validated by ValidatePluginID
// (org.plugin.name.vX or org.plugin.name.vX.Y.Z) is recommended for new plugins; existing
// plugins that use other IDs (e.g. "my-plugin-v1") can skip calling ValidatePluginID if needed.

// ID format constants
const (
	// DefaultOrg is the default organization identifier
	DefaultOrg = "go-lynx"
	// ComponentType represents the plugin component type
	ComponentType = "plugin"
)

// IDFormat represents the components of a plugin ID
type IDFormat struct {
	Organization string // e.g., "go-lynx"
	Type         string // e.g., "plugin"
	Name         string // e.g., "http"
	Version      string // e.g., "v1" or "v1.0.0"
}

// ParsePluginID parses a plugin ID string into its components
func ParsePluginID(id string) (*IDFormat, error) {
	parts := strings.Split(id, ".")
	if len(parts) != 4 {
		return nil, ErrInvalidPluginID
	}

	format := &IDFormat{
		Organization: parts[0],
		Type:         parts[1],
		Name:         parts[2],
		Version:      parts[3],
	}

	if err := ValidatePluginID(id); err != nil {
		return nil, err
	}

	return format, nil
}

// GeneratePluginID generates a standard format plugin ID
func GeneratePluginID(org, name, version string) string {
	if org == "" {
		org = DefaultOrg
	}

	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return fmt.Sprintf("%s.%s.%s.%s", org, ComponentType, name, version)
}

// ValidatePluginID validates the recommended plugin ID format (org.plugin.name.vX[.Y.Z]).
// Non-standard IDs used by existing plugins may not pass; use only when enforcing the standard format.
func ValidatePluginID(id string) error {
	// Regular expression pattern explanation:
	// ^                     String start
	// [\w-]+               Organization name (word characters and hyphens)
	// \.plugin\.           Literal ".plugin."
	// [a-z0-9-]+           Plugin name (lowercase letters, numbers, hyphens)
	// \.v\d+               Version number starting with 'v'
	// (?:\.\d+\.\d+)?      Optional patch version number (e.g., .0.0)
	// $                     String end
	pattern := `^[\w-]+\.plugin\.[a-z0-9-]+\.v\d+(?:\.\d+\.\d+)?$`

	match, _ := regexp.MatchString(pattern, id)
	if !match {
		return ErrInvalidPluginID
	}
	return nil
}

// GetPluginMainVersion extracts the main version number from a plugin ID
func GetPluginMainVersion(id string) (string, error) {
	format, err := ParsePluginID(id)
	if err != nil {
		return "", err
	}

	// Main version is the leading component, e.g. v1 from v1.0.0.
	parts := strings.Split(format.Version, ".")
	return parts[0], nil
}

// IsPluginVersionCompatible reports whether two plugin versions share a major version.
func IsPluginVersionCompatible(v1, v2 string) bool {
	v1Main, err1 := GetPluginMainVersion(v1)
	v2Main, err2 := GetPluginMainVersion(v2)
	if err1 != nil || err2 != nil {
		return false
	}
	return v1Main == v2Main
}
