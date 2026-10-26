#!/bin/sh
set -eu
umask 077
REPO_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$REPO_DIR"
docker info >/dev/null
mkdir -p .cache/cloud-export
stamp=$(date -u +%Y%m%dT%H%M%SZ)
destination="$REPO_DIR/.cache/cloud-export/freehire-$stamp.dump"
temporary="$destination.tmp"
trap 'rm -f "$temporary"' EXIT HUP INT TERM
docker compose exec -T db pg_dump -U hire -d hire --format=custom --no-owner --no-acl > "$temporary"
test -s "$temporary"
mv "$temporary" "$destination"
printf 'Private database backup saved to %s\n' "$destination"
docker compose exec -T db psql -U hire -d hire -Atc \
  "SELECT json_build_object('jobs', (SELECT count(*) FROM jobs), 'boards', (SELECT count(*) FROM boards), 'database_bytes', pg_database_size(current_database()));"
