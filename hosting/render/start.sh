#!/bin/sh
set -eu

: "${DATABASE_URL:?Set DATABASE_URL to the Neon connection string}"
: "${JWT_SECRET:?Set a random JWT_SECRET of at least 32 characters}"
: "${MEILI_MASTER_KEY:?Set a random MEILI_MASTER_KEY of at least 16 characters}"

PUBLIC_ORIGIN=${FRONTEND_ORIGIN:-${RENDER_EXTERNAL_URL:-}}
if [ -z "$PUBLIC_ORIGIN" ] && [ -n "${RAILWAY_PUBLIC_DOMAIN:-}" ]; then
  PUBLIC_ORIGIN="https://$RAILWAY_PUBLIC_DOMAIN"
fi
case "$PUBLIC_ORIGIN" in
  https://*) ;;
  *) echo "Set FRONTEND_ORIGIN to the public HTTPS URL" >&2; exit 1 ;;
esac
export FRONTEND_ORIGIN="$PUBLIC_ORIGIN" ORIGIN="$PUBLIC_ORIGIN"
export PORT="${PORT:-10000}"
case "$PORT" in
  ''|*[!0-9]*) echo "PORT must be a number" >&2; exit 1 ;;
esac
if [ "$PORT" -lt 1024 ] || [ "$PORT" -gt 65535 ] || [ "$PORT" = 8080 ] || [ "$PORT" = 3000 ] || [ "$PORT" = 7700 ] || [ "$PORT" = 6379 ]; then
  echo "Choose an unused public PORT between 1024 and 65535" >&2
  exit 1
fi
sed "s/__PUBLIC_PORT__/$PORT/g" /app/nginx.template.conf > /tmp/nginx.conf
exec supervisord -n -c /app/supervisord.conf
