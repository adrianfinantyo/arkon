# 02: Manifest Schema, Discovery Hierarchy & Validation Engine

**What to build:** Complete parsing, default population, discovery, and strict validation of the `arkon.yaml` declarative manifest. Operators can configure schedules, host power actions, teardown profile tiers, and lifecycle hooks with clear diagnostic error reporting whenever a manifest contains syntax or semantic errors.

**Blocked by:** 01: CLI Entrypoint, Environment Seam & Schema Introspection

**Status:** ready-for-agent

- [ ] Resolves manifest location following the strict precedence: `--config <path>` flag, `./arkon.yaml`, `./arkon.yml`, `/etc/arkon/arkon.yaml`, `/etc/arkon/arkon.yml`.
- [ ] Parses manifest version `"1"` containing `host`, `circadian`, `teardown`, `hooks`, and `inhibition` blocks.
- [ ] Populates sensible defaults when optional fields are omitted (default `timeout_seconds: 30`, `default_duration: "2h"`, `daytime_protection: true`).
- [ ] Validates time formats (`HH:MM`), day names, valid power actions (`poweroff`, `suspend`, `hibernate`), and verifies configured IANA timezone identifiers exist in the system tz database.
- [ ] Enforces the mandatory 180-second minimum safety buffer between circadian `sleep_at` and `wake_at` times, rejecting unsafe configurations with exit code 3.
- [ ] Validation errors output descriptive diagnostic messages pointing to specific field errors in text and JSON envelope modes.
