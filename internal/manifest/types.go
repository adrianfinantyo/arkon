package manifest

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest defines the root structure of the arkon.yaml configuration file.
type Manifest struct {
	Version    string            `yaml:"version" json:"version"`
	Host       HostConfig        `yaml:"host" json:"host"`
	Circadian  CircadianConfig   `yaml:"circadian" json:"circadian"`
	Teardown   TeardownConfig    `yaml:"teardown" json:"teardown"`
	Hooks      HooksConfig       `yaml:"hooks" json:"hooks"`
	Inhibition InhibitionConfig  `yaml:"inhibition" json:"inhibition"`
}

// HostConfig defines host-level execution and ACPI parameters.
type HostConfig struct {
	Timezone    string `yaml:"timezone" json:"timezone"`
	RTCDevice   string `yaml:"rtc_device" json:"rtc_device"`
	PowerAction string `yaml:"power_action" json:"power_action"`
}

// CircadianConfig defines the scheduled sleep and wake times.
type CircadianConfig struct {
	SleepAt string   `yaml:"sleep_at" json:"sleep_at"`
	WakeAt  string   `yaml:"wake_at" json:"wake_at"`
	Days    []string `yaml:"days" json:"days"`
}

// TeardownConfig defines workload shutdown providers and tiered profile ordering.
type TeardownConfig struct {
	Provider       string         `yaml:"provider" json:"provider"`
	TimeoutSeconds int            `yaml:"timeout_seconds" json:"timeout_seconds"`
	ComposeStacks  []ComposeStack `yaml:"compose_stacks" json:"compose_stacks"`
	ProfileOrder   []string       `yaml:"profile_order" json:"profile_order"`
}

// ComposeStack references a Docker Compose directory.
type ComposeStack struct {
	Path string `yaml:"path" json:"path"`
}

// HooksConfig groups lifecycle hook commands.
type HooksConfig struct {
	PreTeardown  []HookConfig `yaml:"pre_teardown" json:"pre_teardown"`
	PostTeardown []HookConfig `yaml:"post_teardown" json:"post_teardown"`
	PreSleep     []HookConfig `yaml:"pre_sleep" json:"pre_sleep"`
	PostWake     []HookConfig `yaml:"post_wake" json:"post_wake"`
}

// HookConfig defines a command executed during host power transitions.
type HookConfig struct {
	Name           string `yaml:"name" json:"name"`
	Command        string `yaml:"command" json:"command"`
	TimeoutSeconds int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	IgnoreErrors   bool   `yaml:"ignore_errors" json:"ignore_errors"`
}

// UnmarshalYAML supports both structured hook objects and bare command strings.
func (h *HookConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		h.Name = value.Value
		h.Command = value.Value
		h.TimeoutSeconds = 10
		h.IgnoreErrors = false
		return nil
	}
	type rawHook HookConfig
	var raw rawHook
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*h = HookConfig(raw)
	return nil
}

// InhibitionConfig sets defaults for power transition locks and daytime guardrails.
type InhibitionConfig struct {
	DefaultDuration   string `yaml:"default_duration" json:"default_duration"`
	DaytimeProtection *bool  `yaml:"daytime_protection" json:"daytime_protection"`
	DaytimeStart      string `yaml:"daytime_start,omitempty" json:"daytime_start,omitempty"`
	DaytimeEnd        string `yaml:"daytime_end,omitempty" json:"daytime_end,omitempty"`
}

// FieldError records a validation failure against a specific manifest field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError holds all collected validation failures for a manifest.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

func (v *ValidationError) Error() string {
	if len(v.Errors) == 0 {
		return "manifest validation failed"
	}
	if len(v.Errors) == 1 {
		return fmt.Sprintf("manifest validation failed: %s: %s", v.Errors[0].Field, v.Errors[0].Message)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("manifest validation failed with %d errors:", len(v.Errors)))
	for _, err := range v.Errors {
		sb.WriteString(fmt.Sprintf("\n  - %s: %s", err.Field, err.Message))
	}
	return sb.String()
}
