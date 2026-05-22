#!/usr/bin/env bash
# Run database migrations (for future use when migrating from SQLite to D1/Postgres)
set -euo pipefail

DB_URL="${DATABASE_URL:-file:./data/bot.db?cache=shared&_journal=WAL}"

echo "Running migrations against: ${DB_URL}"
echo "Note: SQLite migrations are auto-applied on startup. This script is for future D1/Postgres migrations."

# Create data directory if needed
mkdir -p data

echo "Migrations complete."
