# Arkon Agent Guide

Welcome to **Arkon** (`arkon`). This document provides autonomous AI agents with operational guidelines, architectural constraints, and tool conventions for interacting with this repository.

---

## 1. Project Mission & Identity

**Arkon** is an open-source, lightweight single-binary CLI tool and systemd orchestrator designed to parameterize host power lifecycles, manage pluggable workload teardowns (Docker Compose, Podman, K8s, systemd) in dependency order, program hardware RTC wakealarms, isolate degraded hardware disks, and maintain GitOps-driven host systemd units.

---

## 2. Agent Skills

### Issue Tracker
Issues, specifications, and Wayfinder maps live as markdown files in `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage Labels
Canonical 5-role triage vocabulary mapped for local markdown issues. See `docs/agents/triage-labels.md`.

### Domain Docs
Single-context domain architecture with `GLOSSARY.md` at root and ADRs in `docs/adr/`. See `docs/agents/domain.md`.

---

## 3. Technology Stack & Constraints

* **Language:** Go 1.23+
* **Target Platforms:** Linux amd64 / arm64 (Systemd & ACPI RTC support)
* **Design Philosophy:** Single static binary, zero runtime dependencies, low memory footprint (< 15MB heap), non-interactive batch-safe execution.
* **Safety Rules:**
  * Never hardcode host-specific paths or domains.
  * Keep all server-specific behavior pluggable via `arkon.yaml`.
  * Ensure all systemd unit generation logic validates unit syntax before filesystem commitment.
  * Interact with host OS, filesystem, time, and environment variables strictly through `env.HostEnvironment`; avoid calling `os.Getenv`, `time.Now`, or raw `os` methods in domain packages.
