#!/usr/bin/env bash
# Deploy Cloudflare Workers
set -euo pipefail

echo "Deploying Cloudflare Workers..."

# Deploy cron worker
echo "→ Deploying cron worker..."
cd cloudflare/workers/cron
npx wrangler deploy
cd ../../..

# Deploy API proxy worker
echo "→ Deploying API proxy worker..."
cd cloudflare/workers/api
npx wrangler deploy
cd ../../..

echo "All workers deployed."
