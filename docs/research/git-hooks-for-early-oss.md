# Git Hooks and Local Linting Strategy for Early Open-Source Go Projects

## Executive Summary

Early-stage open-source Go projects face a distinct tension: maintain high code quality across distributed contributors without introducing friction that degrades contributor velocity or drives developers to bypass tooling.

This document establishes the Git hook and local verification strategy for Arkon. The recommended model divides checks into three tiers based on execution frequency and latency tolerance:
1. `pre-commit`: Sub-second, staged-only hygiene (code formatting and anti-slop rules).
2. `pre-push`: Boundary verification under five seconds (dependency integrity, static analysis, unit test suite, and whole-tree hygiene).
3. Continuous Integration (CI): Canonical cleanroom environment (cross-compilation matrix for `linux/amd64` and `linux/arm64`, OS-level hardware mocks, and branch protection).

The strategy relies on Git's native `core.hooksPath` configuration introduced in Git 2.9, avoiding external package managers (Node.js/Husky, Python/pre-commit). It preserves developer ergonomics by never mutating staged files, respecting partial staging (`git add -p`), supporting standard `--no-verify` escape hatches, and providing a standalone parity script (`./scripts/check-ci.sh`) that mirrors CI locally.

---

## 1. Division of Responsibility: Pre-Commit, Pre-Push, and CI

Effective Git workflows map checks to the developer's feedback loop. A check placed in the wrong stage harms adoption.

### 1.1 Pre-Commit (Inner Loop: Sub-Second Latency)

* **Execution Frequency:** High (every commit, amend, or micro-commit during local iteration).
* **Latency Budget:** Under 500 milliseconds.
* **Execution Scope:** Staged files only (`git diff --cached --name-only --diff-filter=ACMR`).

#### What belongs in `pre-commit`:
* **Go formatting verification (`gofmt -l`):** Confirms that staged `.go` files comply with standard Go formatting without modifying unstaged work.
* **Anti-slop and style verification (`scripts/deslop.py --staged`):** Validates that staged changes avoid AI-generated filler, em-dashes, and banned buzzwords per `.agents/rules/deslop.md`.
* **File hygiene:** Rejects unresolved merge conflict markers (`<<<<<<<`), unintended binary files, or committed secrets.

#### What does NOT belong in `pre-commit`:
* **Compilation and type checking (`go build`, `go vet`):** Parsing full package dependency graphs across an entire repository takes seconds, which disrupts the inner commit loop.
* **Test execution (`go test`):** Even short unit tests introduce 1 to 3 seconds of lag. Running them on every micro-commit frustrates developers.
* **Network operations (`go mod download`):** Pre-commit hooks must remain offline and instantaneous.

### 1.2 Pre-Push (Transport Boundary: 2 to 10 Seconds)

* **Execution Frequency:** Moderate (a few times per work session before publishing upstream).
* **Latency Budget:** 2 to 10 seconds.
* **Execution Scope:** The entire repository for the branches being pushed (skipping ref deletions).

#### What belongs in `pre-push`:
* **Dependency integrity (`go mod verify`):** Ensures that local `go.mod` and `go.sum` dependencies match expected cryptographic checksums.
* **Static analysis (`go vet ./...`):** Validates package-wide invariants, catching common Go errors (suspicious constructs, printf argument mismatches, struct tag issues, and mutex copying).
* **Fast unit test suite (`go test -v ./...`):** Ensures that all unit tests pass before pushing code to a remote branch or opening a pull request. In Arkon, unit tests finish in ~0.5s cached and ~2.8s cold.
* **Full repository anti-slop verification (`python3 scripts/deslop.py --all --force`):** Confirms entire repository compliance before code leaves the local machine.

#### Why `pre-push` is the correct seam for tests and vet:
* Developers commit frequently while experimenting or breaking work down into small commits. Blocking broken intermediate commits prevents exploration.
* Pushing to a remote repository signals intent to share code, trigger CI runs, or request review. Spending 2 to 5 seconds here prevents failed CI runs and avoids notification noise for team members.

### 1.3 Continuous Integration (Cleanroom Verification: 1 to 5 Minutes)

* **Execution Frequency:** Asynchronous (per push, pull request, or merge to main).
* **Latency Budget:** 1 to 5 minutes.
* **Execution Scope:** Clean, isolated container runner (GitHub Actions `ubuntu-latest`).

#### What belongs strictly in CI:
* **Local verification parity:** Runs the identical check suite (`go mod verify`, `test -z "$(go fmt ./...)"`, `go vet ./...`, `python3 scripts/deslop.py --all --force`, and `go test -v ./...`) to guard against unconfigured contributor environments.
* **Cross-compilation build matrix:** Compiles target binaries across target architectures (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64` and `GOOS=linux GOARCH=arm64`). Contributor workstations often lack cross-compilers or should not spend local cycles building alternate architectures.
* **System integration and daemon tests:** Tests requiring elevated system privileges, systemd isolation, or container runtimes.
* **Security and vulnerability scanning (`govulncheck ./...`):** Periodic database lookups against the Go vulnerability database.
* **Release artifact packaging:** Automated tagging, binary signing, and release asset generation.

### Pipeline Stage Comparison

| Dimension | Pre-Commit | Pre-Push | Continuous Integration (CI) |
| :--- | :--- | :--- | :--- |
| **Frequency** | 10 to 50 times per day | 2 to 5 times per day | Per PR / pushed branch |
| **Target Latency** | < 500 ms | < 5 seconds | 1 to 5 minutes |
| **Target Scope** | Staged files only | Working tree / branch state | Fresh repository checkout |
| **Primary Tasks** | `gofmt` check, staged deslop | `go mod verify`, `go vet`, `go test` | Cross-compile matrix, release builds |
| **Execution Environment**| Local developer shell | Local developer shell | Ephemeral GitHub Actions runner |
| **Failure Consequence** | Blocks local commit creation | Blocks network push | Blocks pull request merge |

---

## 2. Contributor Experience and Ergonomics

### 2.1 The Friction of Heavy Pre-Commit Hooks

Heavy pre-commit hooks remain one of the most common causes of developer frustration in open-source projects. Two issues in particular create friction:

#### 1. Disruption of the Inner Loop
Software development relies on rapid iterations: writing code, checking diffs, creating intermediate commits, and squashing or rebasing later. When a `pre-commit` hook takes several seconds to run full package linting or test suites, developers experience cognitive friction. They quickly develop muscle memory for typing `git commit --no-verify`, which defeats the purpose of the hooks.

#### 2. The Dangers of Stashing and In-Hook File Modification
Many automated hook configurations try to format files automatically during `pre-commit` and run `git add` to restage them. This creates severe pitfalls:
* **Breaking partial staging (`git add -p`):** If a developer selectively stages five lines of a file while leaving experimental debug logging unstaged, running `git add` inside a pre-commit hook stages the entire file. The unstaged debug edits bleed into the commit unnoticed.
* **Stash corruption:** Tools that attempt automatic stashing (`git stash -q --keep-index`) can cause merge conflicts when popping unstaged changes back onto modified files. If the process is interrupted with a SIGINT (Ctrl+C), unstaged work can be trapped in the stash stack or lost.

#### The Golden Rule of Hook Ergonomics: Validate, Never Mutate
Hooks must be read-only and non-destructive. If files fail formatting or style checks, the hook must exit with a non-zero status, print the offending files, and provide the command to fix them. The developer retains control over what is staged.

### 2.2 Why `core.hooksPath` is the Zero-Dependency Standard for Go

Many web and Python ecosystems rely on heavy hook managers:
* **Husky:** Requires Node.js, `npm`, and `package.json`. In a pure Go repository, requiring Node.js adds an alien runtime dependency.
* **pre-commit (Python framework):** Requires Python, `pip`, and `virtualenv`. It downloads isolated Python environments on the fly, consuming disk space and adding 200 to 500 ms of Python startup overhead to every commit.

#### The Go Philosophy: Zero External Toolchain Dependencies
Go developers expect repositories to build and test using standard Go commands (`go build`, `go test`) and basic POSIX shell tools.

In Git 2.9 (released June 2016), Git introduced the `core.hooksPath` configuration setting. This allows a repository to point Git directly to a version-controlled hooks directory:

```bash
git config core.hooksPath .githooks
```

Advantages of `core.hooksPath`:
* **Zero dependencies:** Relies exclusively on Git and the Go toolchain already installed on the host.
* **Version controlled:** Hook scripts live in `.githooks/` and are tracked in Git, reviewed through pull requests, and kept in sync with the codebase.
* **Transparent and inspectable:** Plain POSIX shell scripts (`.githooks/pre-commit`, `.githooks/pre-push`) are easy to read, debug, and maintain.
* **Instant execution:** Direct shell execution has near-zero process startup latency compared to Python or Node.js wrappers.

### 2.3 Bypass Capabilities and Local CI Parity

#### Native Escape Hatches
Hooks must never hold a developer hostage during emergency fixes, branch exploratory rebases, or work-in-progress checkpoints.
* Git natively provides the `--no-verify` (or `-n`) flag for both `git commit` and `git push`:
  ```bash
  git commit --no-verify -m "wip: checkpoint before refactoring"
  git push --no-verify
  ```
* Hook scripts should also inspect environment variables (such as `SKIP_HOOKS=1` or `SKIP_DESLOP=1`) to allow automation scripts, rebases, or external tools to bypass checks cleanly.

#### Local CI Parity via `./scripts/check-ci.sh`
A frequent contributor complaint is pushing a change only to have CI fail on a check they did not know existed. Providing a dedicated runner script (`./scripts/check-ci.sh`) guarantees parity:
* Contributors can run `./scripts/check-ci.sh` locally at any time.
* The script executes the exact sequence run by GitHub Actions.
* The `pre-push` hook delegates directly to this script, ensuring that what passes locally passes CI.

---

## 3. Concrete Recommendation for Arkon

### 3.1 Script Structure for `.githooks/pre-commit`

This script validates staged Go files for formatting and runs the deslop checker against staged modifications. It executes in well under 200 milliseconds.

```bash
#!/usr/bin/env bash
set -e

# Pre-commit hook: Sub-second checks on staged changes.
# Bypassed via: git commit --no-verify OR SKIP_HOOKS=1 / SKIP_DESLOP=1

if [ "${SKIP_HOOKS:-0}" = "1" ] || [ "${SKIP_DESLOP:-0}" = "1" ]; then
    echo "[git-hook] Pre-commit checks skipped via environment flag."
    exit 0
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
DESLOP_SCRIPT="$REPO_ROOT/scripts/deslop.py"
RULES_FILE="$REPO_ROOT/.agents/rules/deslop.md"

# 1. Resolve gofmt binary (standard PATH or GOROOT fallback)
GOFMT_BIN="gofmt"
if ! command -v gofmt >/dev/null 2>&1; then
    GOROOT="$(go env GOROOT 2>/dev/null || true)"
    if [ -n "$GOROOT" ] && [ -x "$GOROOT/bin/gofmt" ]; then
        GOFMT_BIN="$GOROOT/bin/gofmt"
    fi
fi

# 2. Check formatting on staged Go files
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null | grep '\.go$' || true)
if [ -n "$STAGED_GO_FILES" ]; then
    UNFORMATTED=$("$GOFMT_BIN" -l $STAGED_GO_FILES 2>/dev/null || true)
    if [ -n "$UNFORMATTED" ]; then
        echo ""
        echo "[git-hook] ❌ Formatting required on staged Go files:"
        echo "$UNFORMATTED"
        echo ""
        echo "[git-hook] Fix by running: go fmt ./... (or $GOFMT_BIN -w <file>) and re-stage."
        echo "[git-hook] Emergency bypass: git commit --no-verify"
        exit 1
    fi
fi

# 3. Check anti-slop guidelines on staged changes
if [ -f "$DESLOP_SCRIPT" ]; then
    python3 "$DESLOP_SCRIPT" --staged --rules "$RULES_FILE"
    EXIT_CODE=$?
    if [ $EXIT_CODE -ne 0 ]; then
        echo ""
        echo "[git-hook] ❌ Commit blocked: AI slop detected in staged changes."
        echo "[git-hook] Please review .agents/rules/deslop.md and fix the issues above."
        echo "[git-hook] Emergency bypass: git commit --no-verify"
        exit 1
    fi
fi

exit 0
```

### 3.2 Script Structure for `.githooks/pre-push`

This script runs prior to `git push`. It intercepts pushed ref lines from standard input, ignores branch deletions, and invokes the local verification suite.

```bash
#!/usr/bin/env bash
set -e

# Pre-push hook: Runs the local CI verification suite prior to pushing upstream.
# Bypassed via: git push --no-verify OR SKIP_HOOKS=1 / SKIP_PRE_PUSH=1

if [ "${SKIP_HOOKS:-0}" = "1" ] || [ "${SKIP_PRE_PUSH:-0}" = "1" ]; then
    echo "[git-hook] Pre-push checks skipped via environment flag."
    exit 0
fi

ZERO_SHA="0000000000000000000000000000000000000000"
IS_DELETION=1

# Read push ref specs from stdin: <local ref> <local sha> <remote ref> <remote sha>
while read -r local_ref local_sha remote_ref remote_sha; do
    if [ "$local_sha" != "$ZERO_SHA" ] && [ "$local_ref" != "(delete)" ]; then
        IS_DELETION=0
        break
    fi
done

if [ "$IS_DELETION" -eq 1 ]; then
    # Ref deletion push; skip test execution.
    exit 0
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
CI_LOCAL_SCRIPT="$REPO_ROOT/scripts/check-ci.sh"

echo "[git-hook] Running local verification suite before push..."
if [ -f "$CI_LOCAL_SCRIPT" ]; then
    "$CI_LOCAL_SCRIPT"
else
    echo "[git-hook] Warning: scripts/check-ci.sh not found. Running fallback verification."
    go mod verify
    go vet ./...
    go test -v ./...
fi

exit 0
```

### 3.3 Parity Script: `scripts/check-ci.sh`

This script replicates the exact sequence defined in `.github/workflows/ci.yml`. It acts as the single source of truth for local pre-push checks and manual verification.

```bash
#!/usr/bin/env bash
set -euo pipefail

# scripts/check-ci.sh: Runs the exact verification suite executed by GitHub Actions CI.
# Usage: ./scripts/check-ci.sh

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "==> [1/5] Verifying module dependencies (go mod verify)..."
go mod verify

echo "==> [2/5] Verifying code formatting (go fmt ./...)..."
UNFORMATTED="$(go fmt ./...)"
if [ -n "$UNFORMATTED" ]; then
    echo "❌ Error: The following Go files require formatting:"
    echo "$UNFORMATTED"
    echo "Run 'go fmt ./...' to format these files."
    exit 1
fi

echo "==> [3/5] Running static analysis (go vet ./...)..."
go vet ./...

echo "==> [4/5] Checking anti-slop guidelines (scripts/deslop.py --all --force)..."
python3 scripts/deslop.py --all --force

echo "==> [5/5] Running test suite (go test -v ./...)..."
go test -v ./...

echo ""
echo "✅ All local verification checks passed successfully."
```

### 3.4 Contributor Onboarding and Setup

Activating hooks in Arkon requires one standard Git configuration command:

```bash
git config core.hooksPath .githooks
```

Permissions are committed directly to repository tracking:
```bash
chmod +x .githooks/pre-commit .githooks/pre-push scripts/check-ci.sh
```

---

## 4. Primary Source Citations

1. **Git Documentation:**
   * `githooks(5)`: Interface documentation for `pre-commit` and `pre-push` hooks, stdin format for pushed refs, and exit status semantics.
   * `git-config(1)`: Specification for `core.hooksPath` (Git 2.9 release notes, June 2016, Junio C Hamano).
   * `git-commit(1)` and `git-push(1)`: Documentation for the `--no-verify` flag.
2. **Go Toolchain Documentation:**
   * Go Command Reference (`cmd/go`): Specifications for `go fmt`, `go vet`, `go test`, and `go mod verify`.
   * Effective Go (Formatting with `gofmt`): Deterministic styling standards across Go codebases.
   * Go Code Review Comments: Guidelines on static verification and vet warnings.
3. **Software Engineering Standards:**
   * Fowler, Martin: Continuous Integration (fast local feedback loops and cleanroom automated builds).
   * Git Plumbing and Staging Semantics: Pitfalls of `git add` and index mutation within `pre-commit` contexts.
