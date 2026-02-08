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
	// Split the plugin ID string using dots
	parts := strings.Split(id, ".")
	// Check if the number of parts after splitting is 4, if not return invalid plugin ID error
	if len(parts) != 4 {
		return nil, ErrInvalidPluginID
	}

	// Initialize the IDFormat struct
	format := &IDFormat{
		Organization: parts[0],
		Type:         parts[1],
		Name:         parts[2],
		Version:      parts[3],
	}

	// Validate the plugin ID format
	if err := ValidatePluginID(id); err != nil {
		return nil, err
	}

	return format, nil
}

// GeneratePluginID generates a standard format plugin ID
func GeneratePluginID(org, name, version string) string {
	// If organization name is empty, use default organization name
	if org == "" {
		org = DefaultOrg
	}

	// Ensure version starts with 'v', if not add 'v'
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// Concatenate plugin ID according to standard format
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

	// Use regular expression to match plugin ID
	match, _ := regexp.MatchString(pattern, id)
	if !match {
		return ErrInvalidPluginID
	}
	return nil
}

// GetPluginMainVersion extracts the main version number from a plugin ID
func GetPluginMainVersion(id string) (string, error) {
	// Parse plugin ID
	format, err := ParsePluginID(id)
	if err != nil {
		return "", err
	}

	// Split version by dots and extract main version (e.g., extract v1 from v1.0.0)
	parts := strings.Split(format.Version, ".")
	return parts[0], nil
}

// IsPluginVersionCompatible checks if two plugin versions are compatible
func IsPluginVersionCompatible(v1, v2 string) bool {
	// Get main version numbers of two plugin versions
	v1Main, err1 := GetPluginMainVersion(v1)
	v2Main, err2 := GetPluginMainVersion(v2)

	// If errors occur during getting main version numbers, consider versions incompatible
	if err1 != nil || err2 != nil {
		return false
	}

	// Compare if two main version numbers are the same
	return v1Main == v2Main
}
