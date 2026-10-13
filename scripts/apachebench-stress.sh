#!/usr/bin/env bash
set -euo pipefail

base_url="http://${AB_HOST:-${BACKEND_HOST:-localhost}}:${BACKEND_PORT:-8080}"

run_profile() {
	local name="$1"
	local url="$2"
	local requests="$3"
	local concurrency="$4"

	printf '\n=== ApacheBench profile: %s ===\n' "$name"
	AB_URL="$url" \
	AB_REQUESTS="$requests" \
	AB_CONCURRENCY="$concurrency" \
	bash scripts/apachebench.sh
}

run_profile \
	"health-concurrency" \
	"${AB_HEALTH_URL:-${base_url}/movieAPI/health}" \
	"${AB_HEALTH_REQUESTS:-10000}" \
	"${AB_HEALTH_CONCURRENCY:-100}"

run_profile \
	"movies-sustained" \
	"${AB_MOVIES_URL:-${base_url}/movieAPI/movies}" \
	"${AB_MOVIES_REQUESTS:-5000}" \
	"${AB_MOVIES_CONCURRENCY:-50}"

run_profile \
	"movies-large-page" \
	"${AB_MOVIES_LARGE_PAGE_URL:-${base_url}/movieAPI/movies?limit=100}" \
	"${AB_MOVIES_LARGE_PAGE_REQUESTS:-2000}" \
	"${AB_MOVIES_LARGE_PAGE_CONCURRENCY:-25}"

run_profile \
	"genres-concurrency" \
	"${AB_GENRES_URL:-${base_url}/movieAPI/genres}" \
	"${AB_GENRES_REQUESTS:-5000}" \
	"${AB_GENRES_CONCURRENCY:-75}"
