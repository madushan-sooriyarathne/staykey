#!/bin/sh
# One-time local setup: dependencies, env files, Postgres, migrations and the sample account.
set -e
cd "$(dirname "$0")/.."

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "Missing $1. $2"; exit 1; }
}
need bun "Install Bun 1.4 or newer: https://bun.sh"
need go "Install Go 1.27 or newer: https://go.dev/dl"
need node "Install Node.js 22 LTS (Expo and Next.js run on Node)."
need docker "Install Docker, or start your own Postgres 16+ and set DATABASE_URL."

echo "Installing dependencies"
bun install

if [ ! -f apps/mobile/.env ]; then
  cp apps/mobile/.env.example apps/mobile/.env
  echo "Created apps/mobile/.env. Set EXPO_PUBLIC_API_URL to your LAN address to use a phone."
fi

echo "Starting Postgres"
bun run db:up
i=0
until docker compose exec -T postgres pg_isready -U staykey >/dev/null 2>&1; do
  i=$((i + 1))
  [ "$i" -gt 30 ] && { echo "Postgres did not start in time."; exit 1; }
  sleep 1
done

echo "Migrating and seeding"
bun run db:migrate
bun run db:seed

echo
echo "Ready. Run: bun run dev"
echo "Sign in with +94770000001 (owner), +94772223344 (manager) or +94713338899 (caretaker)."
echo "Codes appear in the API log and on screen in development builds."
