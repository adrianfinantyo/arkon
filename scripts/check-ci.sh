#!/usr/bin/env bash
set -e

# Local CI parity script: mirrors GitHub Actions workflow steps.

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "==> Verifying Go module dependencies..."
go mod verify

echo "==> Verifying Go code formatting..."
UNFORMATTED="$(go fmt ./...)"
if [ -n "$UNFORMATTED" ]; then
    echo "Error: The following Go files are not formatted:"
    echo "$UNFORMATTED"
    echo "Run 'go fmt ./...' to format them."
    exit 1
fi

echo "==> Running go vet..."
go vet ./...

echo "==> Verifying anti-slop guidelines..."
python3 scripts/deslop.py --all --force

echo "==> Running Go test suite..."
go test -v ./...

echo "==> All CI parity checks passed."
