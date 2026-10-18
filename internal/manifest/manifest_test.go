package manifest_test

import (
	"strings"
	"testing"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

const sampleValidYAML = `
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0"
  power_action: "poweroff"

circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]

teardown:
  timeout_seconds: 30
  compose_stacks:
    - path: "/opt/stacks/apps"
  profile_order:
    - "apps"
    - "infra"

hooks:
  pre_teardown: []
  post_teardown:
    - name: "filesystem-sync"
      command: "sync"
      timeout_seconds: 10
      ignore_errors: false
  pre_sleep: []
  post_wake: []

inhibition:
  default_duration: "2h"
  daytime_protection: true
`

func TestParseValidManifest(t *testing.T) {
	m, err := manifest.ParseAndValidate([]byte(sampleValidYAML))
	if err != nil {
		t.Fatalf("expected valid manifest to parse cleanly, got: %v", err)
	}

	if m.Version != "1" {
		t.Fatalf("expected version '1', got %q", m.Version)
	}
	if m.Host.Timezone != "Asia/Jakarta" {
		t.Fatalf("expected timezone 'Asia/Jakarta', got %q", m.Host.Timezone)
	}
	if m.Host.RTCDevice != "/sys/class/rtc/rtc0" {
		t.Fatalf("expected rtc_device '/sys/class/rtc/rtc0', got %q", m.Host.RTCDevice)
	}
	if m.Host.PowerAction != "poweroff" {
		t.Fatalf("expected power_action 'poweroff', got %q", m.Host.PowerAction)
	}
	if m.Circadian.SleepAt != "23:00" || m.Circadian.WakeAt != "07:00" {
		t.Fatalf("expected 23:00/07:00, got %s/%s", m.Circadian.SleepAt, m.Circadian.WakeAt)
	}
	if len(m.Circadian.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(m.Circadian.Days))
	}
	if m.Teardown.TimeoutSeconds != 30 {
		t.Fatalf("expected timeout 30, got %d", m.Teardown.TimeoutSeconds)
	}
	if len(m.Teardown.ProfileOrder) != 2 || m.Teardown.ProfileOrder[0] != "apps" {
		t.Fatalf("expected profile order [apps, infra], got %v", m.Teardown.ProfileOrder)
	}
	if len(m.Teardown.ComposeStacks) != 1 || m.Teardown.ComposeStacks[0].Path != "/opt/stacks/apps" {
		t.Fatalf("expected 1 compose stack with path /opt/stacks/apps, got %v", m.Teardown.ComposeStacks)
	}
	if len(m.Hooks.PostTeardown) != 1 || m.Hooks.PostTeardown[0].Name != "filesystem-sync" {
		t.Fatalf("expected post_teardown hook, got %v", m.Hooks.PostTeardown)
	}
	if m.Inhibition.DefaultDuration != "2h" {
		t.Fatalf("expected default_duration '2h', got %q", m.Inhibition.DefaultDuration)
	}
	if m.Inhibition.DaytimeProtection == nil || !*m.Inhibition.DaytimeProtection {
		t.Fatal("expected daytime_protection true")
	}
}

func TestParseBareStringHookShorthand(t *testing.T) {
	yamlData := `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "22:00"
  wake_at: "06:00"
teardown:
  profile_order: ["apps"]
hooks:
  post_teardown:
    - "sync"
`
	m, err := manifest.ParseAndValidate([]byte(yamlData))
	if err != nil {
		t.Fatalf("expected bare string hook to parse cleanly, got: %v", err)
	}
	if len(m.Hooks.PostTeardown) != 1 {
		t.Fatalf("expected 1 post_teardown hook, got %d", len(m.Hooks.PostTeardown))
	}
	h := m.Hooks.PostTeardown[0]
	if h.Name != "sync" || h.Command != "sync" {
		t.Fatalf("expected name and command to be 'sync', got name=%q, command=%q", h.Name, h.Command)
	}
	if h.TimeoutSeconds != 10 {
		t.Fatalf("expected default hook timeout 10s, got %d", h.TimeoutSeconds)
	}
	if h.IgnoreErrors {
		t.Fatal("expected ignore_errors false")
	}
}

func TestDefaultValuesPopulation(t *testing.T) {
	minimalYAML := `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
teardown:
  profile_order: ["apps"]
`
	m, err := manifest.ParseAndValidate([]byte(minimalYAML))
	if err != nil {
		t.Fatalf("expected minimal manifest to parse and validate, got: %v", err)
	}

	if m.Host.RTCDevice != "/sys/class/rtc/rtc0" {
		t.Fatalf("expected default rtc_device '/sys/class/rtc/rtc0', got %q", m.Host.RTCDevice)
	}
	if m.Host.PowerAction != "poweroff" {
		t.Fatalf("expected default power_action 'poweroff', got %q", m.Host.PowerAction)
	}
	if len(m.Circadian.Days) != 7 {
		t.Fatalf("expected default 7 days, got %d", len(m.Circadian.Days))
	}
	expectedDays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for i, d := range expectedDays {
		if m.Circadian.Days[i] != d {
			t.Fatalf("expected day %s at index %d, got %s", d, i, m.Circadian.Days[i])
		}
	}
	if m.Teardown.Provider != "docker-compose" {
		t.Fatalf("expected default provider 'docker-compose', got %q", m.Teardown.Provider)
	}
	if m.Teardown.TimeoutSeconds != 30 {
		t.Fatalf("expected default teardown timeout 30, got %d", m.Teardown.TimeoutSeconds)
	}
	if m.Inhibition.DefaultDuration != "2h" {
		t.Fatalf("expected default duration '2h', got %q", m.Inhibition.DefaultDuration)
	}
	if m.Inhibition.DaytimeProtection == nil || !*m.Inhibition.DaytimeProtection {
		t.Fatal("expected default daytime_protection to be true")
	}
}

func TestDefaultValuesPreserveExplicitOverrides(t *testing.T) {
	overrideYAML := `
version: "1"
host:
  timezone: "UTC"
  rtc_device: "/sys/class/rtc/rtc1"
  power_action: "suspend"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Wed", "Fri"]
teardown:
  provider: "podman-compose"
  timeout_seconds: 60
  profile_order: ["core"]
inhibition:
  default_duration: "4h"
  daytime_protection: false
`
	m, err := manifest.ParseAndValidate([]byte(overrideYAML))
	if err != nil {
		t.Fatalf("expected manifest with explicit overrides to validate, got: %v", err)
	}

	if m.Host.RTCDevice != "/sys/class/rtc/rtc1" {
		t.Fatalf("expected overridden rtc_device, got %q", m.Host.RTCDevice)
	}
	if m.Host.PowerAction != "suspend" {
		t.Fatalf("expected overridden power_action 'suspend', got %q", m.Host.PowerAction)
	}
	if len(m.Circadian.Days) != 3 {
		t.Fatalf("expected 3 days, got %d", len(m.Circadian.Days))
	}
	if m.Teardown.Provider != "podman-compose" {
		t.Fatalf("expected provider 'podman-compose', got %q", m.Teardown.Provider)
	}
	if m.Teardown.TimeoutSeconds != 60 {
		t.Fatalf("expected timeout 60, got %d", m.Teardown.TimeoutSeconds)
	}
	if m.Inhibition.DefaultDuration != "4h" {
		t.Fatalf("expected duration '4h', got %q", m.Inhibition.DefaultDuration)
	}
	if m.Inhibition.DaytimeProtection == nil || *m.Inhibition.DaytimeProtection != false {
		t.Fatal("expected daytime_protection false to be preserved")
	}
}

func TestDiscoveryPrecedence(t *testing.T) {
	t.Run("explicit path flag has highest precedence", func(t *testing.T) {
		mem := env.NewMemEnv()
		_ = mem.WriteFile("/custom/config.yaml", []byte(sampleValidYAML), 0644)
		_ = mem.WriteFile("./arkon.yaml", []byte("invalid yaml"), 0644)

		m, path, err := manifest.Load(mem, "/custom/config.yaml")
		if err != nil {
			t.Fatalf("expected explicit path to load cleanly, got: %v", err)
		}
		if path != "/custom/config.yaml" {
			t.Fatalf("expected path '/custom/config.yaml', got %q", path)
		}
		if m.Host.Timezone != "Asia/Jakarta" {
			t.Fatalf("expected timezone 'Asia/Jakarta', got %q", m.Host.Timezone)
		}
	})

	t.Run("explicit path not found returns error", func(t *testing.T) {
		mem := env.NewMemEnv()
		_ = mem.WriteFile("./arkon.yaml", []byte(sampleValidYAML), 0644)

		_, _, err := manifest.Load(mem, "/nonexistent/config.yaml")
		if err == nil {
			t.Fatal("expected error when explicit config does not exist")
		}
	})

	t.Run("precedence ./arkon.yaml before ./arkon.yml", func(t *testing.T) {
		mem := env.NewMemEnv()
		validYaml := strings.Replace(sampleValidYAML, "Asia/Jakarta", "UTC", 1)
		_ = mem.WriteFile("./arkon.yaml", []byte(validYaml), 0644)
		_ = mem.WriteFile("./arkon.yml", []byte(sampleValidYAML), 0644)

		m, path, err := manifest.Load(mem, "")
		if err != nil {
			t.Fatalf("expected discovery to succeed, got: %v", err)
		}
		if path != "./arkon.yaml" {
			t.Fatalf("expected ./arkon.yaml, got %q", path)
		}
		if m.Host.Timezone != "UTC" {
			t.Fatalf("expected timezone 'UTC' from ./arkon.yaml, got %q", m.Host.Timezone)
		}
	})

	t.Run("precedence ./arkon.yml when ./arkon.yaml absent", func(t *testing.T) {
		mem := env.NewMemEnv()
		_ = mem.WriteFile("./arkon.yml", []byte(sampleValidYAML), 0644)
		_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte("version: 1"), 0644)

		m, path, err := manifest.Load(mem, "")
		if err != nil {
			t.Fatalf("expected discovery to succeed, got: %v", err)
		}
		if path != "./arkon.yml" {
			t.Fatalf("expected ./arkon.yml, got %q", path)
		}
		if m.Host.Timezone != "Asia/Jakarta" {
			t.Fatalf("expected timezone 'Asia/Jakarta', got %q", m.Host.Timezone)
		}
	})

	t.Run("precedence /etc/arkon/arkon.yaml when local configs absent", func(t *testing.T) {
		mem := env.NewMemEnv()
		_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(sampleValidYAML), 0644)
		_ = mem.WriteFile("/etc/arkon/arkon.yml", []byte("version: 1"), 0644)

		_, path, err := manifest.Load(mem, "")
		if err != nil {
			t.Fatalf("expected discovery to succeed, got: %v", err)
		}
		if path != "/etc/arkon/arkon.yaml" {
			t.Fatalf("expected /etc/arkon/arkon.yaml, got %q", path)
		}
	})

	t.Run("precedence /etc/arkon/arkon.yml when all prior absent", func(t *testing.T) {
		mem := env.NewMemEnv()
		_ = mem.WriteFile("/etc/arkon/arkon.yml", []byte(sampleValidYAML), 0644)

		_, path, err := manifest.Load(mem, "")
		if err != nil {
			t.Fatalf("expected discovery to succeed, got: %v", err)
		}
		if path != "/etc/arkon/arkon.yml" {
			t.Fatalf("expected /etc/arkon/arkon.yml, got %q", path)
		}
	})

	t.Run("discovery failure when no candidate exists", func(t *testing.T) {
		mem := env.NewMemEnv()

		_, _, err := manifest.Load(mem, "")
		if err == nil {
			t.Fatal("expected discovery failure when no candidates exist")
		}
		if !strings.Contains(err.Error(), "no manifest found") {
			t.Fatalf("expected informative discovery error, got: %v", err)
		}
	})
}

func TestValidationVersion(t *testing.T) {
	testCases := []struct {
		name    string
		version string
	}{
		{"wrong version 2", "2"},
		{"empty version", ""},
		{"wrong version string", "v1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(sampleValidYAML, `version: "1"`, `version: "`+tc.version+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err == nil {
				t.Fatalf("expected validation failure for version %q", tc.version)
			}
			valErr, ok := err.(*manifest.ValidationError)
			if !ok {
				t.Fatalf("expected *manifest.ValidationError, got %T: %v", err, err)
			}
			found := false
			for _, fe := range valErr.Errors {
				if fe.Field == "version" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected field error on 'version', got: %v", valErr.Errors)
			}
		})
	}
}

func TestValidationTimezone(t *testing.T) {
	t.Run("empty timezone rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `timezone: "Asia/Jakarta"`, `timezone: ""`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected validation error on empty timezone")
		}
	})

	t.Run("non-existent IANA timezone rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `timezone: "Asia/Jakarta"`, `timezone: "Invalid/NotRealCity"`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected validation error on invalid IANA timezone")
		}
		valErr := err.(*manifest.ValidationError)
		found := false
		for _, fe := range valErr.Errors {
			if fe.Field == "host.timezone" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected field error on host.timezone, got: %v", valErr.Errors)
		}
	})

	t.Run("valid IANA timezones accepted", func(t *testing.T) {
		validZones := []string{"UTC", "America/New_York", "Europe/London", "Asia/Tokyo"}
		for _, tz := range validZones {
			raw := strings.Replace(sampleValidYAML, `timezone: "Asia/Jakarta"`, `timezone: "`+tz+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err != nil {
				t.Fatalf("expected timezone %q to be valid, got: %v", tz, err)
			}
		}
	})
}

func TestValidationPowerAction(t *testing.T) {
	t.Run("invalid power actions rejected", func(t *testing.T) {
		invalidActions := []string{"reboot", "shutdown", "destroy", "halt"}
		for _, pa := range invalidActions {
			raw := strings.Replace(sampleValidYAML, `power_action: "poweroff"`, `power_action: "`+pa+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err == nil {
				t.Fatalf("expected power_action %q to be rejected", pa)
			}
		}

		// Direct validation with empty power action
		m, _ := manifest.ParseAndValidate([]byte(sampleValidYAML))
		m.Host.PowerAction = ""
		if err := m.Validate(); err == nil {
			t.Fatal("expected empty power_action to fail direct validation")
		}
	})

	t.Run("valid power actions accepted", func(t *testing.T) {
		validActions := []string{"poweroff", "suspend", "hibernate"}
		for _, pa := range validActions {
			raw := strings.Replace(sampleValidYAML, `power_action: "poweroff"`, `power_action: "`+pa+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err != nil {
				t.Fatalf("expected power_action %q to be valid, got: %v", pa, err)
			}
		}
	})
}

func TestValidationTimeFormat(t *testing.T) {
	invalidTimes := []string{"24:00", "12:60", "9:00", "09:5", "invalid", "25:01", "12-00"}
	for _, it := range invalidTimes {
		t.Run("invalid sleep_at "+it, func(t *testing.T) {
			raw := strings.Replace(sampleValidYAML, `sleep_at: "23:00"`, `sleep_at: "`+it+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err == nil {
				t.Fatalf("expected sleep_at %q to be rejected", it)
			}
		})
		t.Run("invalid wake_at "+it, func(t *testing.T) {
			raw := strings.Replace(sampleValidYAML, `wake_at: "07:00"`, `wake_at: "`+it+`"`, 1)
			_, err := manifest.ParseAndValidate([]byte(raw))
			if err == nil {
				t.Fatalf("expected wake_at %q to be rejected", it)
			}
		})
	}
}

func TestValidationDays(t *testing.T) {
	t.Run("invalid day name rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]`, `["Mon", "Funday"]`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected invalid day to be rejected")
		}
	})

	t.Run("empty days list rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]`, `[]`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected empty days array to be rejected")
		}
	})
}

func TestValidationSafetyBuffer(t *testing.T) {
	testCases := []struct {
		name      string
		sleepAt   string
		wakeAt    string
		expectErr bool
	}{
		{"identical sleep and wake (0s buffer)", "23:00", "23:00", true},
		{"sleep 23:00, wake 23:01 (60s buffer < 180s)", "23:00", "23:01", true},
		{"sleep 23:00, wake 23:02 (120s buffer < 180s)", "23:00", "23:02", true},
		{"sleep 23:59, wake 00:01 (120s wrap buffer < 180s)", "23:59", "00:01", true},
		{"sleep 23:59, wake 00:00 (60s wrap buffer < 180s)", "23:59", "00:00", true},
		{"sleep 23:00, wake 23:03 (exactly 180s buffer)", "23:00", "23:03", false},
		{"sleep 23:59, wake 00:02 (exactly 180s wrap buffer)", "23:59", "00:02", false},
		{"sleep 23:00, wake 07:00 (standard homelab 8h buffer)", "23:00", "07:00", false},
		{"sleep 01:00, wake 05:00 (4h buffer)", "01:00", "05:00", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(sampleValidYAML, `sleep_at: "23:00"`, `sleep_at: "`+tc.sleepAt+`"`, 1)
			raw = strings.Replace(raw, `wake_at: "07:00"`, `wake_at: "`+tc.wakeAt+`"`, 1)

			_, err := manifest.ParseAndValidate([]byte(raw))
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected safety buffer error for sleep=%s, wake=%s", tc.sleepAt, tc.wakeAt)
				}
				valErr, ok := err.(*manifest.ValidationError)
				if !ok {
					t.Fatalf("expected *manifest.ValidationError, got: %v", err)
				}
				found := false
				for _, fe := range valErr.Errors {
					if fe.Field == "circadian" && strings.Contains(fe.Message, "180") {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected safety buffer diagnostic mentioning 180s, got: %v", valErr.Errors)
				}
			} else {
				if err != nil {
					t.Fatalf("expected valid schedule for sleep=%s, wake=%s, got: %v", tc.sleepAt, tc.wakeAt, err)
				}
			}
		})
	}
}

func TestValidationTeardownAndInhibition(t *testing.T) {
	t.Run("empty profile_order rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `- "apps"`+"\n"+`    - "infra"`, `[]`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected empty profile_order to be rejected")
		}
	})

	t.Run("invalid teardown provider rejected", func(t *testing.T) {
		raw := sampleValidYAML + "\n  provider: \"unknown-provider\"\n"
		// insert into teardown section
		raw = strings.Replace(sampleValidYAML, "teardown:\n", "teardown:\n  provider: \"unknown-provider\"\n", 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected unknown teardown provider to be rejected")
		}
	})

	t.Run("invalid inhibition duration rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `default_duration: "2h"`, `default_duration: "invalid-duration"`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected invalid duration to be rejected")
		}
	})

	t.Run("negative inhibition duration rejected", func(t *testing.T) {
		raw := strings.Replace(sampleValidYAML, `default_duration: "2h"`, `default_duration: "-1h"`, 1)
		_, err := manifest.ParseAndValidate([]byte(raw))
		if err == nil {
			t.Fatal("expected negative duration to be rejected")
		}
	})
}

func TestStructuredValidationErrorsFormat(t *testing.T) {
	valErr := &manifest.ValidationError{
		Errors: []manifest.FieldError{
			{Field: "host.timezone", Message: "timezone not found"},
			{Field: "circadian", Message: "safety buffer violation (60s < 180s)"},
		},
	}

	msg := valErr.Error()
	if !strings.Contains(msg, "host.timezone") || !strings.Contains(msg, "timezone not found") {
		t.Fatalf("expected error text to format host.timezone, got: %s", msg)
	}
	if !strings.Contains(msg, "circadian") || !strings.Contains(msg, "safety buffer violation") {
		t.Fatalf("expected error text to format circadian, got: %s", msg)
	}
}

func TestLoadFileWithSyntaxError(t *testing.T) {
	mem := env.NewMemEnv()
	_ = mem.WriteFile("./arkon.yaml", []byte("version: [unclosed"), 0644)

	_, _, err := manifest.Load(mem, "")
	if err == nil {
		t.Fatal("expected syntax error on malformed YAML")
	}
}
