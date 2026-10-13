#!/usr/bin/env bash
set -euo pipefail

: "${AB_URL:=http://${AB_HOST:-${BACKEND_HOST:-localhost}}:${BACKEND_PORT:-8080}/movieAPI/health}"
: "${AB_REQUESTS:=1000}"
: "${AB_CONCURRENCY:=10}"
: "${AB_TIMEOUT:=30}"
: "${AB_KEEPALIVE:=1}"

die() {
	printf 'apachebench: %s\n' "$1" >&2
	exit 2
}

require_positive_integer() {
	local name="$1"
	local value="$2"
	[[ "$value" =~ ^[1-9][0-9]*$ ]] || die "$name must be a positive integer (got '$value')"
}

command -v ab >/dev/null 2>&1 || die "ab is required; install apache2-utils"
command -v curl >/dev/null 2>&1 || die "curl is required for the readiness check"
require_positive_integer AB_REQUESTS "$AB_REQUESTS"
require_positive_integer AB_CONCURRENCY "$AB_CONCURRENCY"
require_positive_integer AB_TIMEOUT "$AB_TIMEOUT"
[[ "$AB_KEEPALIVE" == 0 || "$AB_KEEPALIVE" == 1 ]] ||
	die "AB_KEEPALIVE must be 0 or 1 (got '$AB_KEEPALIVE')"

curl --fail --silent --show-error --max-time "$AB_TIMEOUT" "$AB_URL" >/dev/null ||
	die "readiness check failed for $AB_URL"

printf 'Running ApacheBench against %s (%s requests, %s concurrent clients)\n' \
	"$AB_URL" "$AB_REQUESTS" "$AB_CONCURRENCY"

result_file="$(mktemp)"
trap 'rm -f "$result_file"' EXIT

ab_args=(
	-n "$AB_REQUESTS" \
	-c "$AB_CONCURRENCY" \
	-s "$AB_TIMEOUT" \
)
if [[ "$AB_KEEPALIVE" == 1 ]]; then
	ab_args+=(-k)
fi

ab "${ab_args[@]}" "$AB_URL" | tee "$result_file"

failed_requests="$(awk '/^Failed requests:/ { print $3; exit }' "$result_file")"
[[ "$failed_requests" == 0 ]] ||
	die "ApacheBench reported ${failed_requests:-unknown} failed requests"
