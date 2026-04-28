#!/bin/sh
set -e

RUN_MIGRATIONS="${RUN_MIGRATIONS:-true}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/app/migrations}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_SSLMODE="${POSTGRES_SSLMODE:-disable}"

if [ "$RUN_MIGRATIONS" = "true" ]; then
  echo "running database migrations"
  goose -dir "$MIGRATIONS_DIR" postgres "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB sslmode=$POSTGRES_SSLMODE" up
fi

exec "$@"
