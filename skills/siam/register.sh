#!/bin/sh
# Register SIAM skills with the Master API at startup
# Waits until Master is reachable before registering.

# Use SIAM_MASTER_URL or MASTER_URL before falling back to local host
MASTER_URL="${SIAM_MASTER_URL:-${MASTER_URL:-http://siam-synapse-master:8080}}"
API_KEY="${MASTER_API_KEY:-}"

echo "🔌 Waiting for Master at ${MASTER_URL}..."
until curl -sf "${MASTER_URL}/api/public/v1/health" > /dev/null; do
  sleep 3
done

echo "📚 Registering skills with Master Skill Registry..."
curl -s -X POST "${MASTER_URL}/api/agent/v1/agents/${NODE_ID}/skills" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "skills": ["shell", "python", "system-info", "web-scrape"],
    "image": "siam-synapse-ottoclaw-worker:latest",
    "description": "OttoClaw Brain + Siam Worker Arm"
  }' && echo "✅ Skills registered for ${NODE_ID}"
