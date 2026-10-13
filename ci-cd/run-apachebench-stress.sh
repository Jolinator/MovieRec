#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

# The integration runner resets its Compose services when it finishes. Start a
# clean, complete API stack for the independent ApacheBench profiles.
docker compose up -d --build --remove-orphans \
    auth-postgres movies-postgres recommendations-postgres \
    auth movies recommendations gateway

BACKEND_URL="http://${BACKEND_HOST:-localhost}:${BACKEND_PORT:-8080}/movieAPI/health"
attempt=0
until curl --fail --silent "$BACKEND_URL" | python3 -c '
import json
import sys

try:
    health = json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit(1)
required = (
    "importedMoviesAvailable",
    "authDBAvailable",
    "moviesDBAvailable",
    "recommendationsDBAvailable",
)
sys.exit(not all(health.get(field) is True for field in required))
'; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 300 ]; then
        echo "ApacheBench stack did not become ready within 300 seconds: $BACKEND_URL" >&2
        exit 1
    fi
    sleep 1
done

bash scripts/apachebench-stress.sh
