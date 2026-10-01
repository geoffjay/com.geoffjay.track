#!/bin/sh
# Prints a fresh API bearer token and inserts its SHA-256 hash into the
# track SQLite database (the same scheme TokenStore.Create uses). The raw
# token is echoed exactly once.
#
# Usage: scripts/create-test-token.sh [db_path] [username]
#   db_path   default /tmp/track-e2e.db
#   username  default geoff

set -eu

DB="${1:-/tmp/track-e2e.db}"
USER="${2:-geoff}"

command -v sqlite3 >/dev/null || { echo "sqlite3 required" >&2; exit 1; }

TOKEN="e2e-test-token-$(date +%s)"
HASH=$(printf '%s' "$TOKEN" | shasum -a 256 | cut -d' ' -f1)

sqlite3 "$DB" "INSERT INTO api_tokens (user_id, name, token_hash, created_at)
VALUES ((SELECT id FROM users WHERE username = '$USER'), 'e2e', '$HASH',
        strftime('%Y-%m-%dT%H:%M:%fZ','now'));"

echo "$TOKEN"