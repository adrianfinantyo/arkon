# 0001: Git Hooks and Local CI Parity

## Status
accepted

## Context
Developers and autonomous agents frequently push changes that fail GitHub Actions CI due to simple oversights: unformatted Go files (`go fmt`), lint issues (`go vet`), failing unit tests, or anti-slop violations. Discovering these issues remotely after push increases feedback latency and creates avoidable commit churn.

Running full test suites inside a `pre-commit` hook introduces noticeable latency on every commit. When pre-commit hooks take multiple seconds, developers often bypass them with `--no-verify`.

External hook managers such as python-based `pre-commit` introduce additional dependencies and environment configuration requirements. Arkon targets a zero-dependency, single-binary architecture, so developer tooling should reflect the same simplicity.

## Decision
We implement a zero-external-dependency Git hook strategy tracked directly in the repository:

1. **Tracked hooks directory (`.githooks/`):**
   Developers activate repository hooks using native Git functionality: `git config core.hooksPath .githooks`. This keeps hooks version-controlled without requiring third-party package managers.

2. **Split verification hierarchy:**
   - **Fast pre-commit (`.githooks/pre-commit`):** Executes only formatting (`test -z "$(go fmt ./...)"`) and repository-wide anti-slop checks (`python3 scripts/deslop.py --all --force`). This runs in under 200 milliseconds, keeping commit actions immediate.
   - **Comprehensive pre-push (`.githooks/pre-push`):** Executes the full repository health check script (`scripts/check-ci.sh`) before any branch push to a remote repository.
   - **Standardized CI parity script (`scripts/check-ci.sh`):** Replicates the GitHub Actions test workflow in exact sequence:
     - Dependency verification: `go mod verify`
     - Code formatting: `test -z "$(go fmt ./...)"`
     - Static analysis: `go vet ./...`
     - Anti-slop verification: `python3 scripts/deslop.py --all --force`
     - Full test suite: `go test -v ./...`

3. **Explicit bypass mechanisms:**
   Both hooks can be bypassed in emergencies through Git's native flag (`--no-verify`) or environment variables (`SKIP_PRE_COMMIT=1`, `SKIP_PRE_PUSH=1`).

## Consequences
- **Positive:**
  - Zero external dependencies beyond the standard Go toolchain and Python 3.
  - Commits that pass local pre-push checks match GitHub Actions CI requirements, preventing broken builds on remote branches.
  - Everyday commits remain fast because heavyweight test suites run only at push time.
  - Operators and contributors can run `./scripts/check-ci.sh` locally at any point to verify workspace health.
- **Negative:**
  - Pre-push hooks run the full test suite, adding a short delay (a few seconds) when pushing to remotes.
  - Contributors must run `git config core.hooksPath .githooks` once after cloning, because Git does not enable non-default hooks paths automatically.
