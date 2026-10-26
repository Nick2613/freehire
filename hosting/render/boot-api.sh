#!/bin/sh
set -eu

# The search index is disposable; the job records remain in Neon.
attempt=0
until redis-cli -h 127.0.0.1 ping >/dev/null 2>&1 && curl -fsS http://127.0.0.1:7700/health >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "Search or Redis did not start" >&2
    exit 1
  fi
  sleep 1
done

/app/migrate
/app/rebuild-search.sh
exec /app/hire
