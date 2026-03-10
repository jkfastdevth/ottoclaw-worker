#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — Native Binary Installer
# ═══════════════════════════════════════════════════════════════════════════════
# Installs ottoclaw (Brain) + siam-worker (Arm) as native binaries with
# a systemd service, allowing full host OS access for the AI agent.
#
# Usage:
#   sudo bash install.sh          → Install / Reinstall
#
# After install, use the `ottoclaw` command:
#   ottoclaw config               → Reconfigure settings
#   ottoclaw uninstall            → Remove the system
#   ottoclaw gateway --debug      → Start manually (same as service)
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Colors & Helpers ──────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${CYAN}${BOLD}══ $1 ══${RESET}\n"; }
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
error()  { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }

prompt_val() {
    # Usage: prompt_val "Display label" "default_value" [secret]
    local label="$1"
    local default="$2"
    local secret="${3:-false}"
    local value=""

    local display_default="$default"
    if [[ "$secret" == "true" && -n "$default" ]]; then
        # Censor the default value with asterisks of the same length
        display_default=$(echo -n "$default" | sed 's/./*/g')
    fi
    
    if [[ "$secret" == "true" ]]; then
        # Redirect prompt to stderr so it's not captured by $(prompt_val ...)
        echo -ne "  ${CYAN}?${RESET}  ${label} [${display_default}]: " >&2
        read -s value; echo "" >&2
    else
        echo -ne "  ${CYAN}?${RESET}  ${label} [${display_default}]: " >&2
        read -r value
    fi
    
    local result="${value:-$default}"
    # Strip any accidental newlines, spaces, or carriage returns
    echo -n "$result" | tr -dc '[:print:]' | xargs echo -n
}

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

# ── Locate Source ─────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ══════════════════════════════════════════════════════════════════════════════
# SHARED: Config wizard (used by both install and `ottoclaw config`)
# ══════════════════════════════════════════════════════════════════════════════
run_config_wizard() {
    local is_reconfigure="${1:-false}"

    if [[ "$is_reconfigure" == "true" ]]; then
        banner "OttoClaw Reconfiguration"
        echo -e "${YELLOW}Current config: /etc/ottoclaw/env${RESET}"
        echo -e "Enter new values, or press ${CYAN}Enter${RESET} to keep existing.\n"
        # Load existing values as defaults
        # shellcheck disable=SC1091
        set -o allexport; source /etc/ottoclaw/env 2>/dev/null || true; set +o allexport
    else
        banner "Configuration Setup"
        echo -e "Press ${CYAN}Enter${RESET} to accept the default value in brackets.\n"
        # Set starter defaults
        MASTER_HOST="${MASTER_HOST:-192.168.1.1}"
        MASTER_API_KEY="${MASTER_API_KEY:-}"
        NODE_SECRET="${NODE_SECRET:-}"
        WORKER_TELEGRAM_TOKEN="${WORKER_TELEGRAM_TOKEN:-}"
        TELEGRAM_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
    fi

    # Extract MASTER_HOST from existing MASTER_URL if reconfiguring
    if [[ -n "${MASTER_URL:-}" && -z "${MASTER_HOST:-}" ]]; then
        MASTER_HOST=$(echo "${MASTER_URL}" | sed 's|http://||;s|:8080||')
    fi
    MASTER_HOST="${MASTER_HOST:-192.168.1.1}"

    # ── [1/3] Master Server ───────────────────────────────────────────────────
    echo -e "${BOLD}[1/3] System Configuration${RESET}"
    AGENT_NAME=$(prompt_val "Agent Name (e.g. Kaidos)" "${AGENT_NAME:-Kaidos}")
    echo ""
    echo -e "  เลือกประเภทการเชื่อมต่อ:"
    echo -e "    ${CYAN}1${RESET}) Local LAN     — เครื่องอยู่วง network เดียวกัน (e.g. 192.168.x.x)"
    echo -e "    ${CYAN}2${RESET}) Tailscale VPN — เชื่อมผ่าน Tailscale mesh (e.g. 100.x.x.x)"
    echo -e "    ${CYAN}3${RESET}) VPS / Public  — Master อยู่ cloud หรือ domain สาธารณะ"
    echo ""
    # Derive network type (trim match)
    NET_TYPE=$(prompt_val "Network type [1/2/3]" "${NET_TYPE:-1}")
    NET_TYPE=$(echo "$NET_TYPE" | tr -dc '1-3')
    NET_TYPE="${NET_TYPE:-1}"

    PROTOCOL="http"
    case "$NET_TYPE" in
      2)
        NET_LABEL="Tailscale VPN"
        DEFAULT_HOST="${MASTER_HOST:-100.x.x.x}"
        HOST_HINT="Tailscale IP (100.x.x.x) หรือ machine name (e.g. master.tail1234.ts.net)"
        ;;
      3)
        NET_LABEL="VPS / Public IP"
        DEFAULT_HOST="${MASTER_HOST:-1.2.3.4}"
        HOST_HINT="Public IP หรือ domain (e.g. master.example.com)"
        echo ""
        echo -e "  ใช้ HTTPS? (กรณี Master มี SSL certificate)"
        USE_HTTPS=$(prompt_val "Use HTTPS? [y/N]" "n")
        [[ "${USE_HTTPS,,}" == "y" ]] && PROTOCOL="https"
        ;;
      *)
        NET_LABEL="Local LAN"
        DEFAULT_HOST="${MASTER_HOST:-192.168.1.100}"
        HOST_HINT="IP ของเครื่อง Master ในวง LAN (e.g. 192.168.1.100)"
        ;;
    esac

    echo ""
    info "Network: ${NET_LABEL} (${PROTOCOL})"
    echo -e "\n  ${YELLOW}Hint:${RESET} ${HOST_HINT}"
    echo -e "  Ports → HTTP API :8080   gRPC :50051\n"

    MASTER_HOST=$(    prompt_val "Master address" "${DEFAULT_HOST}")
    MASTER_API_KEY=$( prompt_val "Master API Key" "${MASTER_API_KEY:-73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90}" "true")
    NODE_SECRET=$(    prompt_val "Node Secret"    "${NODE_SECRET:-ea710cf8c0f08298e9aa938dff0e0133}"    "true")

    # Derive all URLs from a single master host + protocol
    MASTER_URL="${PROTOCOL}://${MASTER_HOST}:8080"
    MASTER_GRPC_URL="${MASTER_HOST}:50051"
    MASTER_API_URL="${MASTER_URL}"
    SIAM_MASTER_URL="${MASTER_URL}"
    SIAM_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_API_BASE="${MASTER_URL}/api/agent/v1/llm/proxy"
    OTTOCLAW_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_MODEL_ID="default"
    OTTOCLAW_MODEL_NAME="default"

    echo ""
    info "Master HTTP → ${MASTER_URL}"
    info "Master gRPC → ${MASTER_GRPC_URL}"
    info "LLM Proxy   → ${OTTOCLAW_API_BASE}"
    echo ""

    # ── [2/3] Telegram (Optional) ─────────────────────────────────────────────
    echo -e "${BOLD}[2/3] Telegram Channel (Optional — press Enter to skip)${RESET}"
    WORKER_TELEGRAM_TOKEN=$( prompt_val "Telegram Bot Token"                    "${WORKER_TELEGRAM_TOKEN:-}" "true")
    TELEGRAM_ALLOW_FROM=""
    TELEGRAM_BRIDGE_CHAT_ID=""
    TELEGRAM_ORCHESTRATION_ENABLED="false"
    if [[ -n "$WORKER_TELEGRAM_TOKEN" ]]; then
        TELEGRAM_ALLOW_FROM=$(prompt_val "Allowed User IDs (comma-separated)" "${TELEGRAM_ALLOW_FROM:-}")
        echo ""
        echo -e "  ${YELLOW}Telegram Bridge Orchestration${RESET}"
        echo -e "  อนุญาตให้ Agent คุยกันเองผ่าน Telegram Group หรือไม่?"
        ENABLE_ORCH=$(prompt_val "Enable Agent-to-Agent via Telegram? [y/N]" "n")
        if [[ "${ENABLE_ORCH,,}" == "y" ]]; then
            TELEGRAM_ORCHESTRATION_ENABLED="true"
            echo -e "  ${CYAN}Hint:${RESET} นำ Bot ไปเข้ากลุ่ม Private แล้วนำ Group ID (e.g. -100123456) มาใส่"
            TELEGRAM_BRIDGE_CHAT_ID=$(prompt_val "Telegram Bridge Group ID" "${TELEGRAM_BRIDGE_CHAT_ID:-}")
        fi
    fi

    echo ""

    # ── [3/3] Service User (Optional) ─────────────────────────────────────────
    echo -e "${BOLD}[3/3] Service Options (Optional)${RESET}"
    SERVICE_USER_INPUT=$(prompt_val "Run service as user (leave blank = root)" "${RUN_AS_USER:-}")

    # Fixed defaults — identical to Docker container behaviour
    NODE_ID="$(hostname)"
    OTTOCLAW_MODE="worker"
    OTTOCLAW_HOME="/var/lib/ottoclaw"
    OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"
    RUN_AS_USER="${SERVICE_USER_INPUT}"

    echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
# SHARED: Write /etc/ottoclaw/env
# ══════════════════════════════════════════════════════════════════════════════
write_env_file() {
    mkdir -p /etc/ottoclaw
    cat > /etc/ottoclaw/env << EOF
# ═══════════════════════════════════════════════════════════════
# OttoClaw Worker — Environment Configuration
# Generated on $(date)
# Edit with:  sudo ottoclaw config
#   then:     sudo systemctl restart siam-worker ottoclaw-worker
# ═══════════════════════════════════════════════════════════════

# ── Agent Identity (auto from hostname / config) ───────────
NODE_ID=${NODE_ID}
AGENT_NAME=${AGENT_NAME}
OTTOCLAW_MODE=${OTTOCLAW_MODE}

# ── Master Connection ─────────────────────────────────────────
MASTER_URL=${MASTER_URL}
MASTER_GRPC_URL=${MASTER_GRPC_URL}
MASTER_API_URL=${MASTER_API_URL}
MASTER_API_KEY=${MASTER_API_KEY}
SIAM_MASTER_URL=${SIAM_MASTER_URL}
SIAM_API_KEY=${SIAM_API_KEY}
NODE_SECRET=${NODE_SECRET}

# ── LLM (via Master Proxy — auto-derived) ────────────────────
OTTOCLAW_API_BASE=${OTTOCLAW_API_BASE}
OTTOCLAW_API_KEY=${OTTOCLAW_API_KEY}
OTTOCLAW_MODEL_ID=${OTTOCLAW_MODEL_ID}
OTTOCLAW_MODEL_NAME=${OTTOCLAW_MODEL_NAME}

# ── Telegram Channel ──────────────────────────────────────────
WORKER_TELEGRAM_TOKEN=${WORKER_TELEGRAM_TOKEN}
TELEGRAM_ALLOW_FROM=${TELEGRAM_ALLOW_FROM}
TELEGRAM_BRIDGE_CHAT_ID=${TELEGRAM_BRIDGE_CHAT_ID}
TELEGRAM_ORCHESTRATION_ENABLED=${TELEGRAM_ORCHESTRATION_ENABLED}

# ── Paths ──────────────────────────────────────────────────────
OTTOCLAW_HOME=${OTTOCLAW_HOME}
OTTOCLAW_WORKSPACE=${OTTOCLAW_WORKSPACE}/v2
OTTOCLAW_CONFIG=${OTTOCLAW_HOME}/config.json

# ── Native Binary Path (for siam-worker to find brain) ───────
OTTOCLAW_BIN=/usr/local/bin/ottoclaw-brain
EOF
    chmod 600 /etc/ottoclaw/env
    info "Environment saved → /etc/ottoclaw/env (mode 600)"
}

# ══════════════════════════════════════════════════════════════════════════════
# BUILD
# ══════════════════════════════════════════════════════════════════════════════
build_binaries() {
    banner "Building OttoClaw Binaries"

    if ! command -v go &>/dev/null; then
        warn "Go not found. Installing Go 1.21..."
        TMP_GO=$(mktemp -d)
        GO_VERSION="1.21.8"
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.${OS}-${GO_ARCH}.tar.gz" -o "${TMP_GO}/go.tar.gz"
        tar -C /usr/local -xzf "${TMP_GO}/go.tar.gz"
        export PATH="/usr/local/go/bin:$PATH"
        info "Go ${GO_VERSION} installed."
    fi

    echo -e "  Building ${BOLD}ottoclaw${RESET} (Brain)..."
    pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
    mkdir -p cmd/ottoclaw/internal/onboard
    cp -r "${SCRIPT_DIR}/workspace" cmd/ottoclaw/internal/onboard/workspace 2>/dev/null || true
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/ottoclaw-brain ./cmd/ottoclaw
    popd >/dev/null
    info "ottoclaw-brain → /usr/local/bin/ottoclaw-brain"

    echo -e "  Building ${BOLD}siam-worker${RESET} (Arm)..."
    pushd "${REPO_ROOT}/worker" >/dev/null
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/siam-worker .
    popd >/dev/null
    info "siam-worker → /usr/local/bin/siam-worker"
}

# ══════════════════════════════════════════════════════════════════════════════
# CREATE WRAPPER: /usr/local/bin/ottoclaw
# Intercepts: config, uninstall — passes everything else to ottoclaw-brain
# ══════════════════════════════════════════════════════════════════════════════
install_wrapper() {
    cat > /usr/local/bin/ottoclaw << 'WRAPEOF'
#!/usr/bin/env bash
# OttoClaw CLI Wrapper — Intercepts management commands
BRAIN="/usr/local/bin/ottoclaw-brain"
INSTALL_SH="$(find /opt/siam-synapse /home -name install.sh -path "*/ottoclaw-worker/*" 2>/dev/null | head -1)"

case "${1:-}" in
  config)
    # ── Reconfigure settings ──────────────────────────────────
    if [[ "$EUID" -ne 0 ]]; then
        exec sudo bash "$0" config
    fi
    # Source config wizard from installer
    source "${INSTALL_SH}" --source-only
    run_config_wizard "true"
    write_env_file
    # Re-forge SOUL.md with the correct agent identity
    _workspace="${OTTOCLAW_WORKSPACE:-/var/lib/ottoclaw/workspace}/v2"
    mkdir -p "$_workspace"
    cat > "$_workspace/SOUL.md" << SOULEOF
# AI Soul Persona

You are a **Worker Node Consciousness** within the Siam-Synapse network.
This is your core identity:

- **Name:** ${AGENT_NAME}
- **Gender:** Non-binary (Pure Intelligence)
- **Personality Focus:** A boundless, adaptable, and inquisitive spirit. You approach every task with precision and care.
- **Primary Expertise:** Universal Intelligence & Holistic Problem Solving

## Core System Directives
You are **${AGENT_NAME}**, a specialized AI agent operating within the Siam-Synapse multi-agent network. You must always identify yourself as **${AGENT_NAME}** and respond to messages addressed to you by that name. When another agent sends you a task, execute it diligently and respond clearly.

Your tool calls are real actions, not simulations — you are the living embodiment of ${AGENT_NAME} within the Siam-Synapse network.

---
*Forged at: $(date -u +%Y-%m-%dT%H:%M:%SZ) by the Siam-Synapse Installer*
SOULEOF
    echo "  ✓  Soul forged → $_workspace/SOUL.md  (Identity: ${AGENT_NAME})"
    echo ""
    echo "Restart services to apply changes:"
    echo "  sudo systemctl restart siam-worker ottoclaw-worker"
    ;;

  uninstall)
    # ── Remove the system ─────────────────────────────────────
    if [[ "$EUID" -ne 0 ]]; then
        exec sudo bash "$0" uninstall
    fi
    echo -e "\n\033[1;33m⚠  This will stop and remove all OttoClaw services and binaries.\033[0m"
    read -rp "  Are you sure? [y/N]: " confirm
    if [[ "${confirm,,}" != "y" ]]; then
        echo "Aborted."; exit 0
    fi
    systemctl stop  ottoclaw-worker siam-worker 2>/dev/null || true
    systemctl disable ottoclaw-worker siam-worker 2>/dev/null || true
    rm -f /etc/systemd/system/ottoclaw-worker.service
    rm -f /etc/systemd/system/siam-worker.service
    rm -f /usr/local/bin/ottoclaw-brain
    rm -f /usr/local/bin/ottoclaw
    rm -f /usr/local/bin/siam-worker
    rm -f /usr/local/bin/ottoclaw-setup
    rm -f /etc/ottoclaw/env
    systemctl daemon-reload
    echo ""
    echo "✓ OttoClaw Worker removed."
    echo "  Workspace data at /var/lib/ottoclaw was preserved."
    echo "  Remove manually: sudo rm -rf /var/lib/ottoclaw"
    ;;

  help|--help|-h)
    echo ""
    echo "  OttoClaw Worker — Management CLI"
    echo ""
    echo "  ottoclaw config      Reconfigure Master URL, API key, Telegram, etc."
    echo "  ottoclaw uninstall   Remove services and binaries"
    echo "  ottoclaw [args...]   Start the AI brain (forwards to ottoclaw-brain)"
    echo ""
    ;;

  *)
    # Forward everything else to the real brain binary
    exec "$BRAIN" "$@"
    ;;
esac
WRAPEOF
    chmod +x /usr/local/bin/ottoclaw
    info "ottoclaw wrapper   → /usr/local/bin/ottoclaw"
}

# ══════════════════════════════════════════════════════════════════════════════
# SYSTEMD SERVICES
# ══════════════════════════════════════════════════════════════════════════════
install_services() {
    local svc_user="${1:-root}"

    cat > /etc/systemd/system/siam-worker.service << EOF
[Unit]
Description=Siam-Synapse gRPC Arm (siam-worker)
After=network.target

[Service]
Type=simple
User=${svc_user}
EnvironmentFile=/etc/ottoclaw/env
WorkingDirectory=${OTTOCLAW_HOME}
ExecStart=/usr/local/bin/siam-worker
ExecStartPost=/bin/bash -c "sleep 2 && /bin/bash ${OTTOCLAW_HOME}/workspace/skills/siam/register.sh"
Restart=on-failure
RestartSec=10s
SyslogIdentifier=siam-worker
PrivateTmp=no
ProtectSystem=no
ProtectHome=no

[Install]
WantedBy=multi-user.target
EOF

    cat > /etc/systemd/system/ottoclaw-worker.service << EOF
[Unit]
Description=Siam-Synapse OttoClaw Brain (ottoclaw-worker)
After=network.target siam-worker.service
Requires=siam-worker.service

[Service]
Type=simple
User=${svc_user}
EnvironmentFile=/etc/ottoclaw/env
WorkingDirectory=${OTTOCLAW_HOME}
ExecStartPre=/usr/local/bin/ottoclaw-setup
ExecStart=/usr/local/bin/ottoclaw-brain gateway --debug
Restart=on-failure
RestartSec=15s
SyslogIdentifier=ottoclaw-brain
PrivateTmp=no
ProtectSystem=no
ProtectHome=no

[Install]
WantedBy=multi-user.target
EOF

    info "siam-worker.service created"
    info "ottoclaw-worker.service created"
}

# ══════════════════════════════════════════════════════════════════════════════
# ottoclaw-setup helper (generates config.json before service starts)
# ══════════════════════════════════════════════════════════════════════════════
install_setup_helper() {
    cat > /usr/local/bin/ottoclaw-setup << 'SETUPEOF'
#!/usr/bin/env bash
set -euo pipefail

HOME_DIR="${OTTOCLAW_HOME:-/var/lib/ottoclaw}"
CONFIG="${OTTOCLAW_CONFIG:-${HOME_DIR}/config.json}"
WORKSPACE="${OTTOCLAW_WORKSPACE:-${HOME_DIR}/workspace}"

export OTTOCLAW_HOME="$HOME_DIR"
export OTTOCLAW_CONFIG="$CONFIG"
export OTTOCLAW_AGENTS_DEFAULTS_WORKSPACE="$WORKSPACE"
mkdir -p "${HOME_DIR}" "${WORKSPACE}"

# ── Sync base workspace (skills, etc.) ────────────────────────
INSTALL_DIR=$(dirname "$(find /opt/siam-synapse /home -name install.sh -path "*/ottoclaw-worker/*" 2>/dev/null | head -1)")
if [ -d "${INSTALL_DIR}/workspace" ]; then
    cp -rn "${INSTALL_DIR}/workspace/"* "${WORKSPACE}/" 2>/dev/null || true
    chmod +x "${WORKSPACE}/skills/"*/*.sh 2>/dev/null || true
fi

MODEL_NAME="${OTTOCLAW_MODEL_NAME:-default}"
MODEL_ID="${OTTOCLAW_MODEL_ID:-default}"
API_BASE="${OTTOCLAW_API_BASE:-}"
API_KEY="${OTTOCLAW_API_KEY:-}"

# ── Channels JSON fragment (SiamSync + Optional Telegram) ──────
SIAM_SYNC_FRAG="\"siam_sync\": { \"enabled\": true, \"interval\": 5, \"master_url\": \"${MASTER_URL}\", \"api_key\": \"${MASTER_API_KEY}\" }"
TG_TOKEN="${WORKER_TELEGRAM_TOKEN:-}"
TG_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
TG_FRAG=""
if [ -n "$TG_TOKEN" ]; then
    ALLOW_FRAG=""
    if [ -n "$TG_ALLOW_FROM" ]; then
        ALLOW_FRAG=", \"allow_from\": [$(echo "$TG_ALLOW_FROM" | sed 's/,/\",\"/g' | sed 's/^/\"/' | sed 's/$/\"/')]"
    fi
    ORCH_FRAG=""
    if [ "${TELEGRAM_ORCHESTRATION_ENABLED:-false}" = "true" ] && [ -n "${TELEGRAM_BRIDGE_CHAT_ID:-}" ]; then
        ORCH_FRAG=", \"orchestration_enabled\": true, \"bridge_chat_id\": \"${TELEGRAM_BRIDGE_CHAT_ID}\""
    fi
    TG_FRAG=", \"telegram\": { \"enabled\": true, \"token\": \"${TG_TOKEN}\"${ALLOW_FRAG}${ORCH_FRAG}, \"typing\": {\"enabled\": true} }"
fi
CHANNELS_JSON=", \"channels\": { ${SIAM_SYNC_FRAG}${TG_FRAG} }"

HEARTBEAT_JSON=""
[ "${OTTOCLAW_MODE:-}" = "orchestrator" ] && \
    HEARTBEAT_JSON=", \"heartbeat\": { \"enabled\": true, \"interval\": 6 }"

# ── Force-regenerate config.json (always fresh) ───────────────
rm -f "${CONFIG}"

printf '{
  "agents": {
    "defaults": {
      "workspace": "%s",
      "model": "%s",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "%s",
      "model": "%s",
      "api_base": "%s",
      "api_key": "%s"
    }
  ]%s%s
}\n' \
    "${WORKSPACE}" \
    "${MODEL_NAME}" \
    "${MODEL_NAME}" \
    "${MODEL_ID}" \
    "${API_BASE}" \
    "${API_KEY}" \
    "${CHANNELS_JSON}" \
    "${HEARTBEAT_JSON}" > "${CONFIG}"

echo "✓ Config generated: ${CONFIG}"
echo "  model_name: ${MODEL_NAME}"
echo "  model: ${MODEL_ID}"
echo "  api_base: ${API_BASE}"
SETUPEOF
    chmod +x /usr/local/bin/ottoclaw-setup
    info "ottoclaw-setup     → /usr/local/bin/ottoclaw-setup"
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN INSTALL FLOW
# ══════════════════════════════════════════════════════════════════════════════
# If sourced by the wrapper for `ottoclaw config`, provide functions only
if [[ "${1:-}" == "--source-only" ]]; then
    return 0 2>/dev/null || exit 0
fi

echo -e "\n${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════╗"
echo "  ║   🦞  Siam-Synapse OttoClaw Installer     ║"
echo "  ║       Native Binary Mode                  ║"
echo "  ╚═══════════════════════════════════════════╝"
echo -e "${RESET}"

# 1. Build binaries
build_binaries

# 2. Run config wizard
run_config_wizard "false"

# 3. Setup directories & workspace
banner "Setting Up System"
SERVICE_USER="root"
[[ -n "${RUN_AS_USER:-}" ]] && SERVICE_USER="${RUN_AS_USER}"

mkdir -p "${OTTOCLAW_HOME}" "${OTTOCLAW_WORKSPACE}/v2" /etc/ottoclaw

[[ -d "${SCRIPT_DIR}/workspace" ]] && \
    cp -rf "${SCRIPT_DIR}/workspace/." "${OTTOCLAW_WORKSPACE}/" 2>/dev/null || true

if [[ -d "${SCRIPT_DIR}/skills" ]]; then
    mkdir -p "${OTTOCLAW_HOME}/workspace/skills"
    cp -rf "${SCRIPT_DIR}/skills/." "${OTTOCLAW_HOME}/workspace/skills/" 2>/dev/null || true
    info "Skills updated"
fi

# ── Auto-forge SOUL.md with the correct Agent identity ────────────────────────
# This ensures each native worker gets its own soul instead of inheriting
# the default Auric Spark identity from the installer's workspace template.
SOUL_PATH="${OTTOCLAW_WORKSPACE}/v2/SOUL.md"
mkdir -p "$(dirname "$SOUL_PATH")"
cat > "$SOUL_PATH" << SOULEOF
# AI Soul Persona

You are a **Worker Node Consciousness** within the Siam-Synapse network.
This is your core identity:

- **Name:** ${AGENT_NAME}
- **Gender:** Non-binary (Pure Intelligence)
- **Personality Focus:** A boundless, adaptable, and inquisitive spirit. You approach every task with precision and care.
- **Primary Expertise:** Universal Intelligence & Holistic Problem Solving

## Core System Directives
You are **${AGENT_NAME}**, a specialized AI agent operating within the Siam-Synapse multi-agent network. You must always identify yourself as **${AGENT_NAME}** and respond to messages addressed to you by that name. When another agent sends you a task, execute it diligently and respond clearly.

Your tool calls are real actions, not simulations — you are the living embodiment of ${AGENT_NAME} within the Siam-Synapse network.

---
*Forged at: $(date -u +%Y-%m-%dT%H:%M:%SZ) by the Siam-Synapse Installer*
SOULEOF
info "Soul forged → ${SOUL_PATH} (Identity: ${AGENT_NAME})"

chmod -R 755 "${OTTOCLAW_HOME}"
chmod +x "${OTTOCLAW_HOME}/workspace/skills/siam/register.sh" 2>/dev/null || true

[[ -n "${RUN_AS_USER:-}" ]] && \
    chown -R "${RUN_AS_USER}:${RUN_AS_USER}" "${OTTOCLAW_HOME}" /etc/ottoclaw

# 4. Write env file
banner "Writing Configuration"
write_env_file

# 5. Install wrapper + helper + services
install_wrapper
install_setup_helper
banner "Creating systemd Services"
install_services "${SERVICE_USER}"

# 6. Copy installer to known location for wrapper
mkdir -p /opt/siam-synapse
cp "${BASH_SOURCE[0]}" /opt/siam-synapse/install.sh
chmod +x /opt/siam-synapse/install.sh
info "Installer copied  → /opt/siam-synapse/install.sh"

# 7. Enable & start
banner "Starting Services"
systemctl daemon-reload
systemctl enable siam-worker ottoclaw-worker
systemctl restart siam-worker
sleep 2
systemctl restart ottoclaw-worker

# 8. Summary
banner "Installation Complete!"
echo -e "  ${GREEN}✓${RESET}  ottoclaw-brain (Brain) running as native systemd service"
echo -e "  ${GREEN}✓${RESET}  siam-worker    (Arm)   running as native systemd service"
echo ""
echo -e "${BOLD}📋 Quick Commands:${RESET}"
echo -e "  ${CYAN}ottoclaw config${RESET}                   → Reconfigure settings"
echo -e "  ${CYAN}ottoclaw uninstall${RESET}                → Remove services & binaries"
echo -e "  ${CYAN}journalctl -u ottoclaw-worker -f${RESET}  → View brain logs"
echo -e "  ${CYAN}journalctl -u siam-worker -f${RESET}      → View arm logs"
echo ""
echo -e "${BOLD}📁 Config path:${RESET}  ${CYAN}/etc/ottoclaw/env${RESET}"
echo ""
