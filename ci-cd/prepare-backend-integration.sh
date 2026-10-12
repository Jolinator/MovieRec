#!/usr/bin/env sh
set -eu

: "${TMDB_READ_ACCESS_TOKEN:?TMDB_READ_ACCESS_TOKEN must be configured as a masked CI/CD variable}"

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
SECRETS_DIR="$ROOT_DIR/secrets"

mkdir -p "$SECRETS_DIR/tls"

cp "$ROOT_DIR/secrets-example/ports.env.example" "$SECRETS_DIR/ports.env"
cp "$ROOT_DIR/secrets-example/auth.env.example" "$SECRETS_DIR/auth.env"
cp "$ROOT_DIR/secrets-example/movies.env.example" "$SECRETS_DIR/movies.env"
cp "$ROOT_DIR/secrets-example/recommendations.env.example" "$SECRETS_DIR/recommendations.env"
cp "$ROOT_DIR/secrets-example/jwt-private.pem.example" "$SECRETS_DIR/jwt-private.pem"
cp "$ROOT_DIR/secrets-example/jwt-public.pem.example" "$SECRETS_DIR/jwt-public.pem"
cp "$ROOT_DIR/secrets-example/tls/server.crt.example" "$SECRETS_DIR/tls/server.crt"
cp "$ROOT_DIR/secrets-example/tls/server.key.example" "$SECRETS_DIR/tls/server.key"

sed -i 's/REPLACE_WITH_A_STRONG_PASSWORD/ci-only-password/g' \
  "$SECRETS_DIR/auth.env" \
  "$SECRETS_DIR/movies.env" \
  "$SECRETS_DIR/recommendations.env"

{
    printf 'TMDB_READ_ACCESS_TOKEN=%s\n' "$TMDB_READ_ACCESS_TOKEN"
    printf 'TMDB_IMPORT_LIMIT=100\n'
} > "$SECRETS_DIR/tmdb.env"
