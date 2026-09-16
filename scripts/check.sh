#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

go_version=$(cat .go-version)
lint_version=$(cat .golangci-version)
export PATH="$PWD/.tools/go-$go_version/bin:$PWD/.tools/golangci-lint-$lint_version:$PATH"
export GOTOOLCHAIN=local
# Keep check caches local to this checkout, including sandboxed runs.
export GOCACHE="$PWD/.tools/cache/go-build"
export GOMODCACHE="$PWD/.tools/cache/mod"
export GOLANGCI_LINT_CACHE="$PWD/.tools/cache/golangci-lint"
if ! command -v go >/dev/null || ! command -v golangci-lint >/dev/null; then
    echo 'Missing check tools. Run: make setup-checks' >&2
    exit 1
fi
if [[ $(go env GOVERSION) != "go$go_version" ]] ||
   ! golangci-lint version | grep -Fq "version $lint_version "; then
    echo 'Check tool versions differ from CI. Run: make setup-checks' >&2
    exit 1
fi

base=${CHECK_BASE:-origin/master}
if ! git rev-parse --verify "$base^{commit}" >/dev/null 2>&1; then
    echo "Missing lint base $base. Run git fetch origin master or set CHECK_BASE." >&2
    exit 1
fi
echo "Checking with Go $go_version, golangci-lint $lint_version; lint base: $base"
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
golangci-lint config verify
# Same PR baseline locally and in CI, including pending working-tree edits.
golangci-lint run --new-from-merge-base="$base"
python3 -m unittest discover -s scripts -p 'test_*.py'
