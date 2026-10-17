#!/bin/sh
# Backs up the database, restores the backup into a scratch database and compares row counts,
# so a restore is known to work before one is needed. Exits non-zero on any difference.
#
#   DATABASE_URL  the database to back up (default: the docker-compose one)
#   PG_BIN        folder holding pg_dump, pg_restore and psql of the server's major version
#
# The dump reads with row security on: the policies let every row through when no tenant is
# set, and a table owner can't read with it off, since the tables force it.
set -eu

url=${DATABASE_URL:-postgres://staykey:staykey@localhost:5432/staykey?sslmode=disable}
bin=${PG_BIN:+$PG_BIN/}
scratch="staykey_restore_check_$(date +%s)"
scratch_url=$(printf '%s' "$url" | sed -E "s#^(postgres(ql)?://[^/]+)/[^?]*#\1/$scratch#")
dump=$(mktemp)

cleanup() {
  rm -f "$dump"
  "${bin}psql" -q -d "$url" -c "DROP DATABASE IF EXISTS $scratch" || true
}
trap cleanup EXIT

echo "Backing up"
"${bin}pg_dump" --format=custom --no-owner --enable-row-security --file="$dump" "$url"

echo "Restoring into $scratch"
"${bin}psql" -q -d "$url" -c "CREATE DATABASE $scratch"
"${bin}pg_restore" --no-owner --exit-on-error --dbname="$scratch_url" "$dump"

status=0
for table in accounts properties bookings unit_nights activity subscriptions; do
  before=$("${bin}psql" -At -d "$url" -c "SELECT count(*) FROM $table")
  after=$("${bin}psql" -At -d "$scratch_url" -c "SELECT count(*) FROM $table")
  if [ "$before" = "$after" ]; then
    echo "  $table: $before rows"
  else
    echo "  $table: $before rows, restored $after"
    status=1
  fi
done

if [ "$status" -ne 0 ]; then
  echo "The restore doesn't match the database."
  exit 1
fi
echo "The backup restores cleanly."
