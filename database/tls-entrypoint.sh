#!/bin/sh
set -eu

# The mounted key is owned by the host user. Copy it while this container's
# entrypoint is still running as root, then give it the permissions PostgreSQL
# requires before the official image drops privileges.
cp /run/postgres-tls/server.crt /tmp/server.crt
cp /run/postgres-tls/server.key /tmp/server.key
chown postgres:postgres /tmp/server.crt /tmp/server.key
chmod 600 /tmp/server.key

exec /usr/local/bin/docker-entrypoint.sh "$@"
