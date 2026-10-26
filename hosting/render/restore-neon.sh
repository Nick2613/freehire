#!/bin/sh
set -eu
: "${NEON_DATABASE_URL:?Set NEON_DATABASE_URL to the direct SSL connection string for an empty Neon PostgreSQL 18 database}"
if [ "$#" -ne 1 ] || [ ! -s "$1" ]; then
  echo "Usage: NEON_DATABASE_URL=... $0 /absolute/path/to/backup.dump" >&2
  exit 1
fi
dump_dir=$(CDPATH='' cd -- "$(dirname -- "$1")" && pwd)
dump_path="$dump_dir/$(basename -- "$1")"
empty_database_sql="SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f')"
# Credentials stay in the container environment and are never printed.
docker run --rm -i \
  -e PGDATABASE="$NEON_DATABASE_URL" \
  -e EMPTY_DATABASE_SQL="$empty_database_sql" \
  -v "$dump_path:/restore/backup.dump:ro" \
  --entrypoint /bin/sh pgvector/pgvector:pg18 -eu -c '
    version=$(psql -X -d "$PGDATABASE" -v ON_ERROR_STOP=1 -Atc "SHOW server_version_num")
    if [ "$version" -lt 180000 ]; then
      echo "The backup requires PostgreSQL 18 or newer." >&2
      exit 1
    fi
    objects=$(psql -X -d "$PGDATABASE" -v ON_ERROR_STOP=1 -Atc "$EMPTY_DATABASE_SQL")
    if [ "$objects" -ne 0 ]; then
      echo "Refusing restore: the target database is not empty." >&2
      exit 1
    fi
    pg_restore --no-owner --no-privileges --no-comments --single-transaction --exit-on-error --dbname "$PGDATABASE" /restore/backup.dump
    psql -X -d "$PGDATABASE" -v ON_ERROR_STOP=1 -Atc "SELECT count(*) AS jobs FROM jobs; SELECT count(*) AS boards FROM boards;"
  '
