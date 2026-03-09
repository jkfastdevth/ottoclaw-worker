#!/bin/sh
# entrypoint.sh — Start both siam-worker (Arm) and OttoClaw (Brain)
# Modes:
#   OTTOCLAW_ONESHOT_MESSAGE — one-shot job, exits after reply
#   OTTOCLAW_MODE=orchestrator — persistent OttoClaw orchestrator replacement
#   (default) — persistent worker agent
set -e

echo "🚀 Starting Siam-Synapse OttoClaw Worker (Brain & Arm)"
echo "  NODE_ID: ${NODE_ID:-unknown}"
echo "  MASTER_GRPC_URL: ${MASTER_GRPC_URL:-not set}"
echo "  MODE: ${OTTOCLAW_MODE:-worker}"

OTTOCLAW_HOME_DIR="${OTTOCLAW_HOME:-/root/.ottoclaw}"
export OTTOCLAW_HOME="$OTTOCLAW_HOME_DIR"

MODEL_NAME="${OTTOCLAW_MODEL_NAME:-default}"
MODEL_ID="${OTTOCLAW_MODEL_ID:-}"
if [ -z "$MODEL_ID" ]; then
  MODEL_ID="${LLM_PROVIDER:-}/"
fi
MODEL_ID="${MODEL_ID%/}"

API_BASE_OVERRIDE="${OTTOCLAW_API_BASE:-}"
API_KEY_OVERRIDE="${OTTOCLAW_API_KEY:-}"

CONFIG_PATH="${OTTOCLAW_CONFIG:-${OTTOCLAW_HOME_DIR}/config.json}"
export OTTOCLAW_CONFIG="$CONFIG_PATH"

# Default workspace — baked into image at /app/workspace (orchestrator prompts)
WORKSPACE_DIR="${OTTOCLAW_WORKSPACE:-/app/workspace}"
export OTTOCLAW_AGENTS_DEFAULTS_WORKSPACE="$WORKSPACE_DIR"

mkdir -p "$OTTOCLAW_HOME_DIR" "$WORKSPACE_DIR"

# ── Channels JSON fragment (SiamSync + Optional Telegram) ──────────────────
SIAM_SYNC_FRAG="\"siam_sync\": { \"enabled\": true, \"interval\": 5, \"master_url\": \"${MASTER_URL}\", \"api_key\": \"${MASTER_API_KEY}\" }"
TG_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TG_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
TG_FRAG=""
if [ -n "$TG_TOKEN" ]; then
  ALLOW_FRAGMENT=""
  if [ -n "$TG_ALLOW_FROM" ]; then
    ALLOW_FRAGMENT=", \"allow_from\": [$(echo "$TG_ALLOW_FROM" | sed 's/,/\",\"/g' | sed 's/^/\"/' | sed 's/$/\"/')]"
  fi
  ORCH_FRAG=""
  if [ "${TELEGRAM_ORCHESTRATION_ENABLED:-false}" = "true" ] && [ -n "${TELEGRAM_BRIDGE_CHAT_ID:-}" ]; then
    ORCH_FRAG=", \"orchestration_enabled\": true, \"bridge_chat_id\": \"${TELEGRAM_BRIDGE_CHAT_ID}\""
  fi
  TG_FRAG=", \"telegram\": { \"enabled\": true, \"token\": \"${TG_TOKEN}\"${ALLOW_FRAGMENT}${ORCH_FRAG}, \"typing\": {\"enabled\": true} }"
  echo "📱 Telegram channel: enabled"
fi
CHANNELS_JSON=", \"channels\": { ${SIAM_SYNC_FRAG}${TG_FRAG} }"

# ── Heartbeat JSON fragment (orchestrator mode: 6-min autonomous health loop) ─
HEARTBEAT_JSON=""
if [ "${OTTOCLAW_MODE:-}" = "orchestrator" ]; then
  HEARTBEAT_INTERVAL="${OTTOCLAW_HEARTBEAT_INTERVAL:-6}"
  HEARTBEAT_JSON=", \"heartbeat\": { \"enabled\": true, \"interval\": ${HEARTBEAT_INTERVAL} }"
  echo "❤️  Heartbeat: enabled (every ${HEARTBEAT_INTERVAL} min)"
fi

# ── Generate config.json if not already present ────────────────────────────
if [ ! -f "$CONFIG_PATH" ]; then
  if [ -n "$API_BASE_OVERRIDE" ] && [ -n "$API_KEY_OVERRIDE" ] && [ -n "$MODEL_ID" ]; then
    echo "🧩 Generating OttoClaw config (master proxy): $CONFIG_PATH"
    cat > "$CONFIG_PATH" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "${WORKSPACE_DIR}",
      "model": "${MODEL_NAME}",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "${MODEL_NAME}",
      "model": "${MODEL_ID}",
      "api_base": "${API_BASE_OVERRIDE}",
      "api_key": "${API_KEY_OVERRIDE}"
    }
  ]${CHANNELS_JSON}${HEARTBEAT_JSON}
}
EOF
  elif [ -n "$MODEL_ID" ] && [ -n "${LLM_API_KEY:-}" ]; then
    echo "🧩 Generating OttoClaw config: $CONFIG_PATH"
    cat > "$CONFIG_PATH" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "${WORKSPACE_DIR}",
      "model": "${MODEL_NAME}",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "${MODEL_NAME}",
      "model": "${MODEL_ID}",
      "api_key": "${LLM_API_KEY}"
    }
  ]${CHANNELS_JSON}${HEARTBEAT_JSON}
}
EOF
  fi
fi

# ── Orchestrator mode: copy workspace identity files if not yet present ────
if [ "${OTTOCLAW_MODE:-}" = "orchestrator" ]; then
  echo "🤖 Orchestrator mode: loading OttoClaw identity"
  for f in SOUL.md AGENTS.md USER.md IDENTITY.md HEARTBEAT.md; do
    SRC="/app/workspace/${f}"
    DST="${WORKSPACE_DIR}/${f}"
    if [ -f "$SRC" ] && [ ! -f "$DST" ]; then
      cp "$SRC" "$DST"
      echo "  📄 Loaded ${f}"
    fi
  done
fi

# ── 1. Register SIAM skills with Master (non-blocking) ─────────────────────
/app/skills/siam/register.sh &

# ── 2. Start gRPC worker Arm in background ────────────────────────────────
echo "💪 Starting siam-worker arm..."
/app/siam-worker &
WORKER_PID=$!

# ── 3. Start OttoClaw Brain ───────────────────────────────────────────────
if [ "${OTTOCLAW_MODE:-}" = "baremetal" ]; then
  echo "💤 Worker is an empty body. Foregoing immediate Brain launch."
  echo "💪 Executing siam-worker arm as main process..."
  exec /app/siam-worker
fi

echo "🧠 Starting OttoClaw brain..."

if [ -n "${OTTOCLAW_ONESHOT_MESSAGE:-}" ]; then
  echo "🎯 One-shot mode enabled"
  echo "  Model: ${MODEL_NAME}"
  echo "  Message: ${OTTOCLAW_ONESHOT_MESSAGE}"

  /app/ottoclaw agent --model "${MODEL_NAME}" -m "${OTTOCLAW_ONESHOT_MESSAGE}"

  echo "🛑 One-shot complete. Stopping siam-worker arm..."
  kill "$WORKER_PID" 2>/dev/null || true
  wait "$WORKER_PID" 2>/dev/null || true
  echo "✅ Exiting"
  exit 0
fi

# Persistent mode (orchestrator or worker) — keep running, serve channels
exec /app/ottoclaw gateway --debug
