package manifest

// Standard default days of week for circadian schedules.
var DefaultDays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// ApplyDefaults populates default values for optional fields when omitted.
func (m *Manifest) ApplyDefaults() {
	if m.Host.RTCDevice == "" {
		m.Host.RTCDevice = "/sys/class/rtc/rtc0"
	}
	if m.Host.PowerAction == "" {
		m.Host.PowerAction = "poweroff"
	}

	if m.Circadian.Days == nil {
		m.Circadian.Days = make([]string, len(DefaultDays))
		copy(m.Circadian.Days, DefaultDays)
	}

	if m.Teardown.Provider == "" {
		m.Teardown.Provider = "docker-compose"
	}
	if m.Teardown.TimeoutSeconds <= 0 {
		m.Teardown.TimeoutSeconds = 30
	}

	applyHookDefaults(m.Hooks.PreTeardown)
	applyHookDefaults(m.Hooks.PostTeardown)
	applyHookDefaults(m.Hooks.PreSleep)
	applyHookDefaults(m.Hooks.PostWake)

	if m.Inhibition.DefaultDuration == "" {
		m.Inhibition.DefaultDuration = "2h"
	}
	if m.Inhibition.DaytimeProtection == nil {
		defaultTrue := true
		m.Inhibition.DaytimeProtection = &defaultTrue
	}
}

func applyHookDefaults(hooks []HookConfig) {
	for i := range hooks {
		if hooks[i].TimeoutSeconds <= 0 {
			hooks[i].TimeoutSeconds = 10
		}
	}
}
