#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/../frontend"
npm ci --no-audit --no-fund
npm run build
