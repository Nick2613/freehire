#!/bin/sh
set -eu

# Let indexing finish before starting Node to reduce cold-start peak memory.
until curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; do
  sleep 2
done
exec node /app/build
