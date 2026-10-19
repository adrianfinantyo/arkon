# 17: Local CI Parity & Pre-Push Hook Automation

**What to build:** Implement local CI parity checks through `scripts/check-ci.sh`, wire `.githooks/pre-push` to prevent broken pushes, maintain fast `.githooks/pre-commit`, and record ADR on git hooks behavior.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Create `scripts/check-ci.sh` running full CI checks (`go mod verify`, `test -z "$(go fmt ./...)"`, `go vet ./...`, `python3 scripts/deslop.py --all --force`, `go test -v ./...`).
- [ ] Create executable `.githooks/pre-push` that invokes `scripts/check-ci.sh` before allowing any push to remote.
- [ ] Update `.githooks/pre-commit` to maintain sub-second execution (fast formatting and deslop verification).
- [ ] Record ADR `docs/adr/0001-git-hooks-and-ci-parity.md` explaining the hook hierarchy and zero-external-dependency model.
- [ ] Ensure scripts are executable (`chmod +x`) and work smoothly with `git config core.hooksPath .githooks`.
