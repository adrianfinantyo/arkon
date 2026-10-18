package manifest

import (
	"fmt"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/env"
	"gopkg.in/yaml.v3"
)

// DiscoveryPaths defines the fallback search order when no explicit manifest path is provided.
var DiscoveryPaths = []string{
	"./arkon.yaml",
	"./arkon.yml",
	"/etc/arkon/arkon.yaml",
	"/etc/arkon/arkon.yml",
}

// DiscoverPath resolves the manifest file path according to the strict discovery precedence.
func DiscoverPath(e env.HostEnvironment, explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := e.ReadFile(explicitPath); err != nil {
			return "", fmt.Errorf("manifest not found at %s: %w", explicitPath, err)
		}
		return explicitPath, nil
	}

	for _, candidate := range DiscoveryPaths {
		if _, err := e.ReadFile(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no manifest found in search locations: %s", strings.Join(DiscoveryPaths, ", "))
}

// ParseAndValidate unmarshals manifest YAML, populates defaults, and performs strict validation.
func ParseAndValidate(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	m.ApplyDefaults()

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// Load discovers, reads, parses, applies defaults, and validates a manifest from the environment.
func Load(e env.HostEnvironment, explicitPath string) (*Manifest, string, error) {
	path, err := DiscoverPath(e, explicitPath)
	if err != nil {
		return nil, "", err
	}

	data, err := e.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("failed to read manifest at %s: %w", path, err)
	}

	m, err := ParseAndValidate(data)
	if err != nil {
		return nil, path, err
	}

	return m, path, nil
}
