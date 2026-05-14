#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "== go vet =="
go vet ./...
echo "== go test =="
go test ./... -count=1 "$@"
