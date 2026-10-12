#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
OUT_DIR="$ROOT_DIR/testing/backend/unit"

mkdir -p "$OUT_DIR"
cd "$ROOT_DIR/backend"

go test ./... -count=1 -coverprofile="$OUT_DIR/coverage.out"
go tool cover -func="$OUT_DIR/coverage.out" | tail -n 1
