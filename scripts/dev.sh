#!/usr/bin/env bash
# Development server with live reload (requires air: go install github.com/air-verse/air@latest)
set -euo pipefail

echo "Starting x-finance-bot in development mode..."

if command -v air &> /dev/null; then
    air -c .air.toml 2>/dev/null || air
else
    echo "air not found, running directly..."
    go run ./cmd/bot
fi
