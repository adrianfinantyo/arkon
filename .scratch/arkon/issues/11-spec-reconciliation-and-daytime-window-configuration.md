# 11: Spec Reconciliation & Daytime Window Configuration

**What to build:** Add optional `daytime_start` and `daytime_end` configuration fields to `InhibitionConfig` in `arkon.yaml`, defaulting to circadian `wake_at` and `sleep_at` when omitted. Reconcile `.scratch/arkon/spec.md` with implemented features: document canonical `host.power_action`, `host.rtc_device` default (`/sys/class/rtc/rtc0`), the `arkon wake` command, scalar hook shorthand syntax, and dynamic OS hostname discovery.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Add `DaytimeStart` and `DaytimeEnd` string fields to `manifest.InhibitionConfig` (`daytime_start`, `daytime_end`).
- [x] In `manifest.ApplyDefaults`, populate `DaytimeStart` and `DaytimeEnd` from `circadian.wake_at` and `circadian.sleep_at` when omitted.
- [x] Add validation in `manifest.Validate` enforcing valid 24h `HH:MM` time formats on `daytime_start` and `daytime_end` when explicitly specified.
- [x] Update `state.DaytimeConfig` and `lifecycle.Coordinator` to use the configured daytime window boundaries.
- [x] Reconcile `.scratch/arkon/spec.md`:
  - Document `daytime_start` and `daytime_end` in the manifest schema.
  - Document `host.power_action` as the canonical location for host power transitions.
  - Set `rtc_device` default to `/sys/class/rtc/rtc0`.
  - Document `arkon wake` subcommand for executing post-wake lifecycle hooks.
  - Document scalar hook shorthand (`- "sync"`) alongside structured hook objects.
  - Clarify that hostname is discovered from host environment rather than statically configured in YAML.
- [x] Unit and integration tests verify explicit daytime window configuration and default inheritance.
