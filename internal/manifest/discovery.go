package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"gopkg.in/yaml.v3"
)

// Canonical system manifest paths and state locations.
const (
	CanonicalManifestPath    = "/etc/arkon/arkon.yaml"
	CanonicalManifestAltPath = "/etc/arkon/arkon.yml"
	defaultStateDir          = "/var/lib/arkon"
)

type hostStateRecord struct {
	ActiveManifest string     `json:"active_manifest"`
	AppliedAt      *time.Time `json:"applied_at"`
	AppliedBy      string     `json:"applied_by"`
}

// DiscoveryPaths defines the fallback search order when no explicit manifest path is provided.
var DiscoveryPaths = []string{
	CanonicalManifestPath,
	CanonicalManifestAltPath,
	"./arkon.yaml",
	"./arkon.yml",
}

// ResolutionResult holds the outcome of resolving a manifest from the environment.
type ResolutionResult struct {
	Path         string
	IsActive     bool
	IsExplicit   bool
	DriftNotice  string
	BrokenActive bool
	ActivePath   string
	AppliedAt    *time.Time
	AppliedBy    string
}

// ResolveManifest resolves the authoritative manifest according to host precedence.
// For operational commands, the active host manifest is prioritized over directory-local files.
func ResolveManifest(e env.HostEnvironment, explicitPath string, stateDir string) (*ResolutionResult, error) {
	if explicitPath != "" {
		if _, err := e.ReadFile(explicitPath); err != nil {
			return nil, fmt.Errorf("manifest not found at %s: %w", explicitPath, err)
		}
		return &ResolutionResult{
			Path:       explicitPath,
			IsExplicit: true,
		}, nil
	}

	if stateDir == "" {
		stateDir = defaultStateDir
	}

	// 1. Check state file for registered active manifest
	stateFile := filepath.Join(stateDir, "state.json")
	if data, err := e.ReadFile(stateFile); err == nil && len(data) > 0 {
		var st hostStateRecord
		if err := json.Unmarshal(data, &st); err == nil && st.ActiveManifest != "" {
			registeredPath := st.ActiveManifest
			if _, readErr := e.ReadFile(registeredPath); readErr != nil {
				return &ResolutionResult{
					BrokenActive: true,
					ActivePath:   registeredPath,
					AppliedAt:    st.AppliedAt,
					AppliedBy:    st.AppliedBy,
				}, nil
			}

			driftNotice := detectLocalDrift(e, registeredPath)
			return &ResolutionResult{
				Path:        registeredPath,
				IsActive:    true,
				ActivePath:  registeredPath,
				DriftNotice: driftNotice,
				AppliedAt:   st.AppliedAt,
				AppliedBy:   st.AppliedBy,
			}, nil
		}
	}

	// 2. Check canonical system manifest (/etc/arkon/arkon.yaml or .yml)
	for _, canonical := range []string{CanonicalManifestPath, CanonicalManifestAltPath} {
		if _, err := e.ReadFile(canonical); err == nil {
			driftNotice := detectLocalDrift(e, canonical)
			return &ResolutionResult{
				Path:        canonical,
				IsActive:    true,
				ActivePath:  canonical,
				DriftNotice: driftNotice,
			}, nil
		}
	}

	// 3. Fall back to current working directory if unconfigured on host
	for _, local := range []string{"./arkon.yaml", "./arkon.yml"} {
		if _, err := e.ReadFile(local); err == nil {
			return &ResolutionResult{
				Path:     local,
				IsActive: false,
			}, nil
		}
	}

	return nil, fmt.Errorf("no manifest found in search locations: %s", strings.Join(DiscoveryPaths, ", "))
}

// detectLocalDrift checks if a local manifest exists in CWD that differs from the active manifest.
func detectLocalDrift(e env.HostEnvironment, activePath string) string {
	cleanActive := filepath.Clean(activePath)
	for _, localCandidate := range []string{"./arkon.yaml", "./arkon.yml"} {
		if _, err := e.ReadFile(localCandidate); err == nil {
			cleanLocal := filepath.Clean(localCandidate)
			absLocal, _ := filepath.Abs(localCandidate)
			if cleanActive != cleanLocal && cleanActive != absLocal {
				return fmt.Sprintf("Found local %s that differs from active host manifest (%s). Run 'sudo arkon apply' to activate it, or pass -c %s to inspect it.", localCandidate, cleanActive, localCandidate)
			}
		}
	}
	return ""
}

// ResolveApplySource resolves the manifest specifically for 'arkon apply'.
// It prioritizes explicit -c, then local ./arkon.yaml (to promote local stack),
// then the registered active host manifest (to re-synchronize), and errors if none exist.
func ResolveApplySource(e env.HostEnvironment, explicitPath string, stateDir string) (string, error) {
	if explicitPath != "" {
		if _, err := e.ReadFile(explicitPath); err != nil {
			return "", fmt.Errorf("manifest not found at %s: %w", explicitPath, err)
		}
		return explicitPath, nil
	}

	// 1. Prioritize local manifest in CWD to promote stack
	for _, local := range []string{"./arkon.yaml", "./arkon.yml"} {
		if _, err := e.ReadFile(local); err == nil {
			return local, nil
		}
	}

	// 2. Fall back to existing active host manifest for re-sync
	if stateDir == "" {
		stateDir = defaultStateDir
	}
	stateFile := filepath.Join(stateDir, "state.json")
	if data, err := e.ReadFile(stateFile); err == nil && len(data) > 0 {
		var st hostStateRecord
		if err := json.Unmarshal(data, &st); err == nil && st.ActiveManifest != "" {
			if _, readErr := e.ReadFile(st.ActiveManifest); readErr == nil {
				return st.ActiveManifest, nil
			}
		}
	}

	for _, canonical := range []string{CanonicalManifestPath, CanonicalManifestAltPath} {
		if _, err := e.ReadFile(canonical); err == nil {
			return canonical, nil
		}
	}

	return "", fmt.Errorf("no manifest found in current directory or system state; specify a manifest path via 'sudo arkon apply -c <path>'")
}

// DiscoverPath resolves the manifest file path according to the discovery precedence.
func DiscoverPath(e env.HostEnvironment, explicitPath string) (string, error) {
	res, err := ResolveManifest(e, explicitPath, "")
	if err != nil {
		return "", err
	}
	if res.BrokenActive {
		return "", fmt.Errorf("active host manifest at %s is missing or unreadable", res.ActivePath)
	}
	return res.Path, nil
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
