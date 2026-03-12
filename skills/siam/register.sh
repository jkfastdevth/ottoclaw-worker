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

# ── 1. Register Node Metadata ────────────────────────────────────────────────
# Gather basic specs to help Master identify this vessel
OS_INFO=$(uname -sr)
SYSTEM_SPEC="$(nproc) Cores, $(free -h | awk '/^Mem:/ {print $2}') RAM"
IP_ADDR=$(hostname -I | awk '{print $1}')

echo "📞 Registering Vessel [${NODE_ID}] at ${MASTER_URL}..."
curl -s -X POST "${MASTER_URL}/api/agent/v1/nodes/register" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d "{
    \"id\": \"${NODE_ID}\",
    \"ip\": \"${IP_ADDR}\",
    \"os_info\": \"${OS_INFO}\",
    \"system_spec\": \"${SYSTEM_SPEC}\",
    \"vessel_type\": \"Native Worker\"
  }"

# ── 2. Register Skills (Individual calls) ────────────────────────────────────
# Master endpoint: POST /api/agent/v1/agents/:id/skills
# Expects: { skill_name, agent_image, description }
echo "📚 Registering skills with Master..."

SKILLS="shell python system-info web-scrape"
for skill in $SKILLS; do
  curl -s -X POST "${MASTER_URL}/api/agent/v1/agents/${NODE_ID}/skills" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: ${API_KEY}" \
    -d "{
      \"skill_name\": \"${skill}\",
      \"agent_image\": \"siam-synapse-ottoclaw-worker:latest\",
      \"description\": \"Running natively on ${NODE_ID}\"
    }" > /dev/null && echo "  ✅ Skill: ${skill}"
done

echo "🎉 Awakening complete for ${NODE_ID}"
