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

	if m.Circadian.SleepAt == "" {
		m.Circadian.SleepAt = "23:00"
	}
	if m.Circadian.WakeAt == "" {
		m.Circadian.WakeAt = "07:00"
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

	if m.Startup.Provider == "" {
		if m.Teardown.Provider != "" {
			m.Startup.Provider = m.Teardown.Provider
		} else {
			m.Startup.Provider = "docker-compose"
		}
	}
	if m.Startup.TimeoutSeconds <= 0 {
		m.Startup.TimeoutSeconds = 120
	}
	if len(m.Startup.ComposeStacks) == 0 && len(m.Teardown.ComposeStacks) > 0 {
		m.Startup.ComposeStacks = make([]ComposeStack, len(m.Teardown.ComposeStacks))
		copy(m.Startup.ComposeStacks, m.Teardown.ComposeStacks)
	}
	if len(m.Startup.Profiles) == 0 && len(m.Teardown.ProfileOrder) > 0 {
		m.Startup.Profiles = make([]string, len(m.Teardown.ProfileOrder))
		for i, p := range m.Teardown.ProfileOrder {
			m.Startup.Profiles[len(m.Teardown.ProfileOrder)-1-i] = p
		}
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
	if m.Inhibition.DaytimeStart == "" {
		m.Inhibition.DaytimeStart = m.Circadian.WakeAt
	}
	if m.Inhibition.DaytimeEnd == "" {
		m.Inhibition.DaytimeEnd = m.Circadian.SleepAt
	}
}

func applyHookDefaults(hooks []HookConfig) {
	for i := range hooks {
		if hooks[i].TimeoutSeconds <= 0 {
			hooks[i].TimeoutSeconds = 10
		}
	}
}
