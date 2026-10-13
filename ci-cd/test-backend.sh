#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

cd "$ROOT_DIR"
python3 testing/backend/DockerTests/TestRunner.py
./ci-cd/run-apachebench-stress.sh
