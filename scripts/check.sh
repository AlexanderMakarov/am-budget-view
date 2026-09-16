#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export GOTOOLCHAIN=local
if ! command -v go >/dev/null; then
    echo 'Go is not installed. Install Go 1.21 or newer with your OS package manager.' >&2
    exit 1
fi
if [[ ${SKIP_GOLANGCI_LINT:-0} != 1 ]] && ! command -v golangci-lint >/dev/null; then
    echo 'golangci-lint v2 is not installed. See https://golangci-lint.run/welcome/install/.' >&2
    exit 1
fi

base=${CHECK_BASE:-origin/master}
if ! git rev-parse --verify "$base^{commit}" >/dev/null 2>&1; then
    echo "Missing lint base $base. Run git fetch origin master or set CHECK_BASE." >&2
    exit 1
fi
echo "Checking with $(go version); lint base: $base"
git diff --check
# Match CI's existing formatting scope; legacy root files have a backlog.
unformatted=$(gofmt -l internal main.go)
if [[ -n "$unformatted" ]]; then
    echo "Run gofmt -w on these files:" >&2
    echo "$unformatted" >&2
    exit 1
fi
go mod download
go vet ./...
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
if [[ ${SKIP_GOLANGCI_LINT:-0} != 1 ]]; then
    golangci-lint version
    golangci-lint config verify
    # Same PR baseline locally and in CI, including pending working-tree edits.
    golangci-lint run --new-from-merge-base="$base"
fi
python3 -m unittest discover -s scripts -p 'test_*.py'
