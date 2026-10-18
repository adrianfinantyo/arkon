# Contributing to Arkon

Thank you for your interest in contributing to Arkon. Arkon is an open-source, lightweight CLI tool and systemd orchestrator designed to parameterize host power lifecycles, manage Docker Compose profile teardowns, and program hardware RTC wakealarms.

---

## Development Prerequisites

* **Go:** Version 1.23 or newer
* **Target OS:** Linux (amd64 or arm64). For development on macOS or Windows, use a Linux VM or container.
* **Linters:** `golangci-lint` or standard `go vet`

---

## Local Setup

1. **Clone the repository:**
   ```bash
   git clone git@github.com:adrianfinantyo/arkon.git
   cd arkon
   ```

2. **Run tests:**
   ```bash
   go test -v ./...
   ```

3. **Build the binary:**
   ```bash
   go build -o bin/arkon ./cmd/arkon
   ```

---

## Hardware Abstraction & Testing Guidelines

Arkon interacts with low-level Linux kernel interfaces (`/sys/class/rtc/rtc0/wakealarm`, `/run/systemd/system/`, and Docker sockets). 

* **Never require root privileges for unit tests:** All hardware and system interactions must be abstracted behind interfaces. Unit tests should run against mock RTC devices and temporary directories without requiring `sudo`.
* **Testing RTC wakealarm:** Set `ARKON_RTC_DEVICE=/tmp/fake-wakealarm` in tests to verify read and write operations without affecting host hardware.
* **Zero Runtime Dependencies:** Arkon must remain a single, statically compiled binary with no CGO dependencies and no external runtime requirements beyond systemd and Docker.

---

## Reporting Hardware Compatibility

Because ACPI sleep states (S3/S5) and motherboard RTC implementations vary widely between vendors, community hardware reports are invaluable.

If you have tested Arkon on your server hardware, please open an issue using the **Hardware Compatibility Report** template with:
* Motherboard / system model (e.g. Dell OptiPlex 7010, Intel NUC 8, custom Mini-ITX)
* CPU model
* Linux distribution and kernel version
* Tested power action (`poweroff` via ACPI S5 vs `suspend` via ACPI S3)
* RTC wakealarm behavior (did it wake up on schedule?)

---

## Pull Request Guidelines

1. **Keep PRs focused:** One bug fix or feature per pull request.
2. **Verify tests pass:** Run `go test ./...` and `go vet ./...` before submitting.
3. **Descriptive commit messages:** Use conventional commit prefixes (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`).
4. **Preserve documentation integrity:** Update relevant documentation if CLI flags or configuration parameters change.

---

## Code of Conduct

Be welcoming, respectful, and constructive. We prioritize clear technical communication and helpful collaboration.
