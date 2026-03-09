#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — Native Binary Installer
# ═══════════════════════════════════════════════════════════════════════════════
# Installs ottoclaw (Brain) + siam-worker (Arm) as native binaries with
# a systemd service, allowing full host OS access for the AI agent.
#
# Usage:  sudo bash install.sh
#         sudo bash install.sh --uninstall
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Colors & Helpers ──────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${CYAN}${BOLD}══ $1 ══${RESET}\n"; }
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
error()  { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }
prompt() {
    local var_name="$1"
    local display="$2"
    local default="$3"
    local secret="${4:-false}"
    local value=""

    if [ "$secret" = "true" ]; then
        read -rsp "  ${CYAN}?${RESET}  ${display} [${default:+hidden}${default:+}]: " value
        echo ""
    else
        read -rp  "  ${CYAN}?${RESET}  ${display} [${default}]: " value
    fi
    echo "${value:-$default}"
}

# ── Uninstall Mode ────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--uninstall" ]]; then
    banner "Uninstalling OttoClaw Worker"
    systemctl stop  ottoclaw-worker siam-worker 2>/dev/null || true
    systemctl disable ottoclaw-worker siam-worker 2>/dev/null || true
    rm -f /etc/systemd/system/ottoclaw-worker.service
    rm -f /etc/systemd/system/siam-worker.service
    rm -f /usr/local/bin/ottoclaw
    rm -f /usr/local/bin/siam-worker
    rm -f /etc/ottoclaw/env
    systemctl daemon-reload
    info "OttoClaw Worker uninstalled."
    info "Note: workspace data in /var/lib/ottoclaw was NOT removed."
    exit 0
fi

# ── Root Check ────────────────────────────────────────────────────────────────
if [[ "$EUID" -ne 0 ]]; then
    error "Please run as root: sudo bash install.sh"
fi

# ── Detect Architecture ───────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  GO_ARCH="amd64" ;;
    aarch64) GO_ARCH="arm64" ;;
    armv7l)  GO_ARCH="arm"   ;;
    *)       error "Unsupported architecture: $ARCH" ;;
esac
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# ── Check Build Tools ─────────────────────────────────────────────────────────
banner "Checking Prerequisites"
if ! command -v go &>/dev/null; then
    warn "Go not found. Installing Go 1.21..."
    TMP_GO=$(mktemp -d)
    GO_VERSION="1.21.8"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.${OS}-${GO_ARCH}.tar.gz" -o "${TMP_GO}/go.tar.gz"
    tar -C /usr/local -xzf "${TMP_GO}/go.tar.gz"
    export PATH="/usr/local/go/bin:$PATH"
    info "Go ${GO_VERSION} installed."
fi
info "Go: $(go version)"

# ── Locate Source ─────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
info "Source root: ${REPO_ROOT}"

# ── Build Binaries ────────────────────────────────────────────────────────────
banner "Building OttoClaw Binaries"

echo -e "  Building ${BOLD}ottoclaw${RESET} (Brain)..."
pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
mkdir -p cmd/ottoclaw/internal/onboard
cp -r "${SCRIPT_DIR}/workspace" cmd/ottoclaw/internal/onboard/workspace 2>/dev/null || true
CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/ottoclaw ./cmd/ottoclaw
popd >/dev/null
info "ottoclaw → /usr/local/bin/ottoclaw"

echo -e "  Building ${BOLD}siam-worker${RESET} (Arm)..."
pushd "${REPO_ROOT}/worker" >/dev/null
CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/siam-worker .
popd >/dev/null
info "siam-worker → /usr/local/bin/siam-worker"

# ── Interactive Configuration ─────────────────────────────────────────────────
banner "Configuration Setup"
echo -e "${BOLD}Please provide the following configuration values.${RESET}"
echo -e "Press ${CYAN}Enter${RESET} to accept the default value shown in brackets.\n"

# --- Section 1: Agent Identity ---
echo -e "${BOLD}[1/5] Agent Identity${RESET}"
NODE_ID=$(        prompt "NODE_ID"    "Node ID (unique name for this machine)"           "worker-$(hostname)")
AGENT_NAME=$(     prompt "AGENT_NAME" "Agent Soul Name (e.g. nova-spire, zephyr-flux)"   "nova-spire")
OTTOCLAW_MODE=$(  prompt "MODE"       "Mode: worker / orchestrator / baremetal"           "worker")

echo ""

# --- Section 2: Master Connection ---
echo -e "${BOLD}[2/5] Master Connection${RESET}"
MASTER_URL=$(      prompt "MASTER_URL"      "Master HTTP URL (for REST API)"                  "http://master:8080")
MASTER_GRPC_URL=$( prompt "MASTER_GRPC_URL" "Master gRPC address (host:port)"                 "master:50051")
MASTER_API_URL=$(  prompt "MASTER_API_URL"  "Master API base URL (for agent API)"              "${MASTER_URL}")
MASTER_API_KEY=$(  prompt "MASTER_API_KEY"  "Master API Key"                                   "" "true")
NODE_SECRET=$(     prompt "NODE_SECRET"     "Node Secret (shared with Master)"                 "" "true")

echo ""

# --- Section 3: LLM Configuration ---
echo -e "${BOLD}[3/5] LLM Configuration${RESET}"
echo -e "  ${YELLOW}Tip:${RESET} Use Master Proxy to keep API keys central (recommended)."
echo -e "  → Set API Base to: ${CYAN}${MASTER_URL}/api/agent/v1/llm/proxy${RESET}"
echo -e "  → Set API Key to your Master API Key above"
echo ""
LLM_MODE=$(prompt "LLM_MODE" "LLM Mode: proxy (use Master) / direct (own key)" "proxy")

if [[ "$LLM_MODE" == "proxy" ]]; then
    OTTOCLAW_API_BASE="${MASTER_URL}/api/agent/v1/llm/proxy"
    OTTOCLAW_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_MODEL_ID="default"
    OTTOCLAW_MODEL_NAME="default"
    info "Using Master LLM Proxy → ${OTTOCLAW_API_BASE}"
else
    echo -e "  ${BOLD}Direct LLM Provider:${RESET}"
    LLM_PROVIDER=$(   prompt "PROVIDER"    "Provider (groq/openai/anthropic/ollama/...)"    "groq")
    LLM_API_KEY=$(    prompt "LLM_API_KEY" "LLM API Key"                                    "" "true")
    LLM_MODEL=$(      prompt "MODEL"       "Model ID (e.g. groq/llama-3.3-70b-versatile)"   "groq/llama-3.3-70b-versatile")
    LLM_MODEL_NAME=$( prompt "MODEL_NAME"  "Model display name (e.g. llama-3.3)"             "llama-3.3")

    OTTOCLAW_API_BASE=$(prompt "API_BASE"  "Provider API base URL"                           "https://api.groq.com/openai/v1")
    OTTOCLAW_API_KEY="${LLM_API_KEY}"
    OTTOCLAW_MODEL_ID="${LLM_MODEL}"
    OTTOCLAW_MODEL_NAME="${LLM_MODEL_NAME}"
fi

echo ""

# --- Section 4: Telegram (Optional) ---
echo -e "${BOLD}[4/5] Telegram Channel (Optional)${RESET}"
TELEGRAM_BOT_TOKEN=$(prompt "TELEGRAM_BOT_TOKEN" "Telegram Bot Token (leave blank to skip)"  "")
TELEGRAM_ALLOW_FROM=""
if [[ -n "$TELEGRAM_BOT_TOKEN" ]]; then
    TELEGRAM_ALLOW_FROM=$(prompt "TELEGRAM_ALLOW_FROM" "Allowed Telegram IDs (comma-separated)" "")
fi

echo ""

# --- Section 5: Paths & Workspace ---
echo -e "${BOLD}[5/5] Paths & Workspace${RESET}"
OTTOCLAW_HOME=$(    prompt "OTTOCLAW_HOME"    "OttoClaw home dir (config, cache)"               "/var/lib/ottoclaw")
OTTOCLAW_WORKSPACE=$(prompt "OTTOCLAW_WORKSPACE" "Agent workspace dir (markdown files, tools)"  "${OTTOCLAW_HOME}/workspace")
RUN_AS_USER=$(       prompt "RUN_AS_USER"     "Run service as user (leave blank = root)"          "")

echo ""

# ── Create Directories & User ─────────────────────────────────────────────────
banner "Setting Up System"
mkdir -p "${OTTOCLAW_HOME}" "${OTTOCLAW_WORKSPACE}/v2" /etc/ottoclaw

# Copy workspace files from source
if [[ -d "${SCRIPT_DIR}/workspace" ]]; then
    cp -rn "${SCRIPT_DIR}/workspace/." "${OTTOCLAW_WORKSPACE}/" 2>/dev/null || true
    info "Workspace files copied to ${OTTOCLAW_WORKSPACE}"
fi

# Copy SIAM skills
if [[ -d "${SCRIPT_DIR}/skills" ]]; then
    mkdir -p "${OTTOCLAW_HOME}/workspace/skills"
    cp -rn "${SCRIPT_DIR}/skills/." "${OTTOCLAW_HOME}/workspace/skills/" 2>/dev/null || true
    info "Skills copied to ${OTTOCLAW_HOME}/workspace/skills"
fi

# Assign ownership if running as specific user
SERVICE_USER="root"
if [[ -n "$RUN_AS_USER" ]]; then
    SERVICE_USER="$RUN_AS_USER"
    chown -R "${RUN_AS_USER}:${RUN_AS_USER}" "${OTTOCLAW_HOME}" /etc/ottoclaw
fi

# ── Write Environment File ────────────────────────────────────────────────────
banner "Writing Environment Configuration"
cat > /etc/ottoclaw/env << EOF
# ═══════════════════════════════════════════════════════════════
# OttoClaw Worker — Environment Configuration
# Generated by install.sh on $(date)
# Edit this file to update configuration, then:
#   sudo systemctl restart ottoclaw-worker siam-worker
# ═══════════════════════════════════════════════════════════════

# ── Agent Identity ───────────────────────────────────────────
NODE_ID=${NODE_ID}
AGENT_NAME=${AGENT_NAME}
OTTOCLAW_MODE=${OTTOCLAW_MODE}

# ── Master Connection ────────────────────────────────────────
MASTER_URL=${MASTER_URL}
MASTER_GRPC_URL=${MASTER_GRPC_URL}
MASTER_API_URL=${MASTER_API_URL}
MASTER_API_KEY=${MASTER_API_KEY}
SIAM_MASTER_URL=${MASTER_URL}
SIAM_API_KEY=${MASTER_API_KEY}
NODE_SECRET=${NODE_SECRET}

# ── LLM Configuration ────────────────────────────────────────
OTTOCLAW_API_BASE=${OTTOCLAW_API_BASE}
OTTOCLAW_API_KEY=${OTTOCLAW_API_KEY}
OTTOCLAW_MODEL_ID=${OTTOCLAW_MODEL_ID}
OTTOCLAW_MODEL_NAME=${OTTOCLAW_MODEL_NAME}

# ── Telegram Channel ─────────────────────────────────────────
TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
TELEGRAM_ALLOW_FROM=${TELEGRAM_ALLOW_FROM}

# ── Paths ────────────────────────────────────────────────────
OTTOCLAW_HOME=${OTTOCLAW_HOME}
OTTOCLAW_WORKSPACE=${OTTOCLAW_WORKSPACE}/v2
EOF

chmod 600 /etc/ottoclaw/env
info "Environment saved → /etc/ottoclaw/env (mode 600)"

# ── Create systemd Service: siam-worker (Arm) ─────────────────────────────────
banner "Creating systemd Services"
cat > /etc/systemd/system/siam-worker.service << EOF
[Unit]
Description=Siam-Synapse gRPC Arm (siam-worker)
Documentation=https://github.com/jkfastdevth/Siam-Synapse
After=network.target
Wants=network.target

[Service]
Type=simple
User=${SERVICE_USER}
EnvironmentFile=/etc/ottoclaw/env
ExecStart=/usr/local/bin/siam-worker
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=siam-worker

# Allow full host access
PrivateTmp=no
ProtectSystem=no
ProtectHome=no
NoNewPrivileges=no

[Install]
WantedBy=multi-user.target
EOF
info "siam-worker.service created"

# ── Create systemd Service: ottoclaw-worker (Brain) ───────────────────────────
cat > /etc/systemd/system/ottoclaw-worker.service << EOF
[Unit]
Description=Siam-Synapse OttoClaw Brain (ottoclaw-worker)
Documentation=https://github.com/jkfastdevth/Siam-Synapse
After=network.target siam-worker.service
Requires=siam-worker.service

[Service]
Type=simple
User=${SERVICE_USER}
EnvironmentFile=/etc/ottoclaw/env
WorkingDirectory=${OTTOCLAW_HOME}

# ── Config Generation (runs before ExecStart) ────────────────
ExecStartPre=/usr/local/bin/ottoclaw-setup

# ── Launch Brain ─────────────────────────────────────────────
ExecStart=/usr/local/bin/ottoclaw gateway --debug
Restart=on-failure
RestartSec=15s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ottoclaw-brain

# Allow full host access (native mode - intentional)
PrivateTmp=no
ProtectSystem=no
ProtectHome=no
NoNewPrivileges=no

[Install]
WantedBy=multi-user.target
EOF
info "ottoclaw-worker.service created"

# ── Create Setup Helper Script ─────────────────────────────────────────────────
cat > /usr/local/bin/ottoclaw-setup << 'SETUPEOF'
#!/usr/bin/env bash
# Generates OttoClaw config.json from environment before service start
set -euo pipefail

OTTOCLAW_HOME_DIR="${OTTOCLAW_HOME:-/var/lib/ottoclaw}"
CONFIG_PATH="${OTTOCLAW_CONFIG:-${OTTOCLAW_HOME_DIR}/config.json}"
WORKSPACE_DIR="${OTTOCLAW_WORKSPACE:-${OTTOCLAW_HOME_DIR}/workspace}"

export OTTOCLAW_HOME="$OTTOCLAW_HOME_DIR"
export OTTOCLAW_CONFIG="$CONFIG_PATH"
export OTTOCLAW_AGENTS_DEFAULTS_WORKSPACE="$WORKSPACE_DIR"

mkdir -p "${OTTOCLAW_HOME_DIR}" "${WORKSPACE_DIR}"

MODEL_NAME="${OTTOCLAW_MODEL_NAME:-default}"
MODEL_ID="${OTTOCLAW_MODEL_ID:-default}"

TG_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TG_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
TG_JSON=""
if [ -n "$TG_TOKEN" ]; then
    ALLOW_FRAGMENT=""
    if [ -n "$TG_ALLOW_FROM" ]; then
        ALLOW_FRAGMENT="\"allow_from\": [$(echo "$TG_ALLOW_FROM" | sed 's/,/\",\"/g' | sed 's/^/\"/' | sed 's/$/\"/')], "
    fi
    TG_JSON=", \"channels\": { \"telegram\": { \"enabled\": true, \"token\": \"${TG_TOKEN}\", ${ALLOW_FRAGMENT}\"typing\": {\"enabled\": true} } }"
fi

HEARTBEAT_JSON=""
if [ "${OTTOCLAW_MODE:-}" = "orchestrator" ]; then
    HEARTBEAT_JSON=", \"heartbeat\": { \"enabled\": true, \"interval\": 6 }"
fi

cat > "${CONFIG_PATH}" << EOF
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
      "api_base": "${OTTOCLAW_API_BASE:-}",
      "api_key": "${OTTOCLAW_API_KEY:-}"
    }
  ]${TG_JSON}${HEARTBEAT_JSON}
}
EOF

echo "✓ OttoClaw config generated: ${CONFIG_PATH}"
SETUPEOF
chmod +x /usr/local/bin/ottoclaw-setup
info "ottoclaw-setup helper → /usr/local/bin/ottoclaw-setup"

# ── Register SIAM Skills ───────────────────────────────────────────────────────
if [[ -f "${SCRIPT_DIR}/skills/siam/register.sh" ]]; then
    cp "${SCRIPT_DIR}/skills/siam/register.sh" /usr/local/bin/siam-register.sh
    chmod +x /usr/local/bin/siam-register.sh
    info "SIAM skills register script → /usr/local/bin/siam-register.sh"
fi

# ── Enable & Start Services ───────────────────────────────────────────────────
banner "Enabling Services"
systemctl daemon-reload
systemctl enable siam-worker ottoclaw-worker
systemctl restart siam-worker
sleep 2
systemctl restart ottoclaw-worker

# ── Print Final Status ────────────────────────────────────────────────────────
banner "Installation Complete!"
echo -e "  ${GREEN}✓${RESET}  ${BOLD}ottoclaw-worker${RESET} (Brain) running as native binary"
echo -e "  ${GREEN}✓${RESET}  ${BOLD}siam-worker${RESET} (Arm) running as native binary"
echo ""
echo -e "${BOLD}📋 Management Commands:${RESET}"
echo -e "  View Brain logs:  ${CYAN}journalctl -u ottoclaw-worker -f${RESET}"
echo -e "  View Arm logs:    ${CYAN}journalctl -u siam-worker -f${RESET}"
echo -e "  Restart both:     ${CYAN}systemctl restart siam-worker ottoclaw-worker${RESET}"
echo -e "  Stop both:        ${CYAN}systemctl stop ottoclaw-worker siam-worker${RESET}"
echo -e "  Edit config:      ${CYAN}nano /etc/ottoclaw/env${RESET}"
echo -e "  Uninstall:        ${CYAN}sudo bash ${SCRIPT_DIR}/install.sh --uninstall${RESET}"
echo ""
echo -e "${BOLD}📁 Paths:${RESET}"
echo -e "  Config/Env:    ${CYAN}/etc/ottoclaw/env${RESET}"
echo -e "  Ottoclaw Home: ${CYAN}${OTTOCLAW_HOME}${RESET}"
echo -e "  Workspace:     ${CYAN}${OTTOCLAW_WORKSPACE}${RESET}"
echo ""
