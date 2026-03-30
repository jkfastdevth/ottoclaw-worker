#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — PRoot Linux Installer (No Systemd)
# ═══════════════════════════════════════════════════════════════════════════════
# สำหรับการใช้งานบน PRoot Distro (เช่น Ubuntu ใน Termux)
# รองรับการติดตั้ง Ollama และรันแบบ Background Process โดยไม่ต้องใช้ systemd
# ═══════════════════════════════════════════════════════════════════════════════
set -uo pipefail

# ── Colors & Helpers ──────────────────────────────────────────────────────────

# ── OTA Update Interceptor ────────────────────────────────────────────────────
if [[ "${1:-}" == "update" ]]; then
    if command -v ottoclaw >/dev/null 2>&1; then
        echo "🔄 Starting OTA Update via native wrapper..."
        exec ottoclaw update
    else
        echo "❌ Cannot perform OTA update: 'ottoclaw' command not found."
        exit 1
    fi
fi
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${CYAN}${BOLD}══ $1 ══${RESET}\n"; }
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
err()    { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }

# ── Locate Source ─────────────────────────────────────────────────────────────
# Get the absolute path of the script directory
SCRIPT_DIR="$(pwd)"
if [[ -n "${BASH_SOURCE+x}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd 2>/dev/null || echo "$SCRIPT_DIR")"
fi
REPO="jkfastdevth/ottoclaw-worker"

# ── Resource Check ────────────────────────────────────────────────────────────
check_resources() {
    local free_mem=$(free -m | grep "Mem:" | awk '{print $7}')
    if [[ $free_mem -lt 1000 ]]; then
        warn "แรมเหลือน้อย ($free_mem MB) แนะนำให้ใช้ Model ขนาดเล็ก เช่น moondream"
    fi
}

prompt_val() {
    local label="$1" default="$2" secret="${3:-false}" value=""
    local display_default="$default"
    if [[ "$secret" == "true" && -n "$default" ]]; then
        display_default=$(echo -n "$default" | sed 's/./*/g')
    fi
    echo -ne "  ${CYAN}?${RESET}  ${label} [${display_default}]: " >&2
    if [[ "$secret" == "true" && "${HIDE_SECRETS:-true}" == "true" ]]; then
        read -s value < /dev/tty; echo "" >&2
    else
        read -r value < /dev/tty
    fi
    local result="${value:-$default}"
    echo -n "$result" | tr -dc '[:print:]' | xargs echo -n
}

get_tailscale_ip() {
    local ts_ip=$(ip addr show 2>/dev/null | grep -oE "\b100\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    [[ -z "$ts_ip" ]] && ts_ip=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep "^100\." | head -n 1)
    echo -n "$ts_ip"
}

get_local_ip() {
    local local_ip=$(ip addr show 2>/dev/null | grep -oE "\b(192\.168|10)\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    [[ -z "$local_ip" ]] && local_ip=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E "^(192\.168|10)\." | head -n 1)
    echo -n "$local_ip"
}

# ── STEP 1: Dependencies & Ollama ─────────────────────────────────────────────
install_deps() {
    banner "Installing PRoot Dependencies"
    apt-get update -y -q
    apt-get install -y -q curl git golang ffmpeg python3-pip python3-av procps psmisc sudo xargs 2>/dev/null || true
    
    if ! command -v ollama &>/dev/null; then
        info "Installing Ollama (Binary mode for PRoot)..."
        local _arch
        case "$(uname -m)" in
            x86_64)  _arch="amd64" ;;
            aarch64) _arch="arm64" ;;
            *)       warn "Unknown arch $(uname -m), defaulting to arm64"; _arch="arm64" ;;
        esac
        local _url="https://github.com/ollama/ollama/releases/latest/download/ollama-linux-${_arch}"
        info "Downloading from: ${_url}"
        if ! curl -fsSL "$_url" -o /usr/local/bin/ollama; then
            warn "Ollama download failed — please install manually: curl -fsSL ${_url} -o /usr/local/bin/ollama"
        else
            chmod +x /usr/local/bin/ollama
            info "Ollama installed to /usr/local/bin/ollama"
        fi
    fi
}

pull_vision_model() {
    banner "Pulling Vision Model (AI)"
    info "Starting temporary Ollama server to pull models..."
    mkdir -p /var/lib/ottoclaw/logs
    nohup ollama serve > /var/lib/ottoclaw/logs/ollama_install.log 2>&1 &
    local ollama_pid=$!
    
    local retry=0
    while ! ollama list >/dev/null 2>&1 && [ $retry -lt 15 ]; do
        sleep 2
        retry=$((retry+1))
    done

    if ollama list >/dev/null 2>&1; then
        info "Pulling moondream (Vision model, ~1.6GB)..."
        ollama pull moondream
        info "Vision model ready."
    else
        warn "Ollama server failed to start in PRoot — ไม่ต้องกังวล ระบบหลักยังทำงานได้ปกติ"
        warn "รันด้วยตัวเองทีหลัง: ollama serve & && ollama pull moondream"
    fi
    
    pkill -f "ollama serve" 2>/dev/null; kill "$ollama_pid" 2>/dev/null; true
}

# ── STEP 2: Full Config Wizard ────────────────────────────────────────────────
run_config_wizard() {
    banner "Configuration Setup"
    echo -e "Press ${CYAN}Enter${RESET} to accept the default value in brackets.\n"

    MASTER_HOST="${MASTER_HOST:-192.168.1.100}"
    MASTER_API_KEY="${MASTER_API_KEY:-}"
    NODE_SECRET="${NODE_SECRET:-}"
    ORCHESTRATOR_TELEGRAM_TOKEN="${ORCHESTRATOR_TELEGRAM_TOKEN:-}"
    TELEGRAM_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
    GOOGLE_EMAIL="${GOOGLE_EMAIL:-}"
    GOOGLE_APP_PASSWORD="${GOOGLE_APP_PASSWORD:-}"

    echo -e "  🛡️  Input Security:"
    HIDE_SECRETS=$(prompt_val "Hide secret keys during input? [Y/n]" "y")
    if [[ "${HIDE_SECRETS,,}" == "n" || "${HIDE_SECRETS,,}" == "no" ]]; then
        HIDE_SECRETS="false"
        info "Keys will be visible as you type/paste them for verification."
    else
        HIDE_SECRETS="true"
    fi
    echo ""

    # ── [1/3] System Config ───────────────────────────────────────────────────
    echo -e "${BOLD}[1/3] System Configuration${RESET}"
    AGENT_NAME=$(prompt_val "AGENT_NAME" "${AGENT_NAME:-Kaidos}")
    ORCHESTRATOR_NICKNAMES=$(prompt_val "ORCHESTRATOR_NICKNAMES" "${ORCHESTRATOR_NICKNAMES:-${AGENT_NAME}}")
    echo ""
    echo -e "  เลือกประเภทการเชื่อมต่อ:"
    echo -e "    ${CYAN}1${RESET}) Local LAN     — เครื่องอยู่วง network เดียวกัน (e.g. 192.168.x.x)"
    echo -e "    ${CYAN}2${RESET}) Tailscale VPN — เชื่อมผ่าน Tailscale mesh (e.g. 100.x.x.x)"
    echo -e "    ${CYAN}3${RESET}) VPS / Public  — Master อยู่ cloud หรือ domain สาธารณะ"
    echo ""
    NET_TYPE=$(prompt_val "Network type [1/2/3]" "${NET_TYPE:-1}")
    NET_TYPE=$(echo "$NET_TYPE" | tr -dc '1-3')
    NET_TYPE="${NET_TYPE:-1}"

    PROTOCOL="http"
    case "$NET_TYPE" in
      2)
        NET_LABEL="Tailscale VPN"
        TS_IP=$(get_tailscale_ip)
        if [[ -n "$TS_IP" ]]; then
            info "Detected Tailscale IP: ${TS_IP}"
            DEFAULT_HOST="${TS_IP}"
        else
            DEFAULT_HOST="${MASTER_HOST:-100.x.x.x}"
        fi
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
        LAN_IP=$(get_local_ip)
        if [[ -n "$LAN_IP" ]]; then
            info "Detected Local IP: ${LAN_IP}"
            DEFAULT_HOST="${LAN_IP}"
        else
            DEFAULT_HOST="${MASTER_HOST:-192.168.1.100}"
        fi
        HOST_HINT="IP ของเครื่อง Master ในวง LAN (e.g. 192.168.1.100)"
        ;;
    esac

    echo ""
    info "Network: ${NET_LABEL} (${PROTOCOL})"
    echo -e "\n  ${YELLOW}Hint:${RESET} ${HOST_HINT}"
    echo -e "  Ports → HTTP API :8080   gRPC :50051\n"

    MASTER_HOST=$(prompt_val "MASTER_HOST" "${DEFAULT_HOST}")
    MASTER_API_KEY=$(prompt_val "MASTER_API_KEY" "${MASTER_API_KEY:-73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90}" "true")
    NODE_SECRET=$(prompt_val "NODE_SECRET" "${NODE_SECRET:-ea710cf8c0f08298e9aa938dff0e0133}" "true")

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
    ORCHESTRATOR_TELEGRAM_TOKEN=$(prompt_val "Telegram Bot Token" "${ORCHESTRATOR_TELEGRAM_TOKEN:-}" "true")
    TELEGRAM_ALLOW_FROM=""
    TELEGRAM_BRIDGE_CHAT_ID=""
    TELEGRAM_ORCHESTRATION_ENABLED="false"
    if [[ -n "$ORCHESTRATOR_TELEGRAM_TOKEN" ]]; then
        TELEGRAM_ALLOW_FROM=$(prompt_val "Allowed User IDs (comma-separated)" "${TELEGRAM_ALLOW_FROM:-}")
        echo ""
        echo -e "  ${YELLOW}Telegram Bridge Orchestration${RESET}"
        echo -e "  อนุญาตให้ Agent คุยกันเองผ่าน Telegram Group หรือไม่?"
        ENABLE_ORCH=$(prompt_val "Enable Agent-to-Agent via Telegram? [y/N]" "n")
        if [[ "${ENABLE_ORCH,,}" == "y" ]]; then
            TELEGRAM_ORCHESTRATION_ENABLED="true"
            echo -e "  ${CYAN}Hint:${RESET} นำ Bot ไปเข้ากลุ่ม Private แล้วนำ Group ID (e.g. -100123456) มาใส่"
            TELEGRAM_BRIDGE_CHAT_ID=$(prompt_val "Telegram Bridge Group ID" "${TELEGRAM_BRIDGE_CHAT_ID:-}")
            echo -e "  ${CYAN}Nicknames:${RESET} ชื่อที่ใช้เรียก Agent ใน Telegram (คั่นด้วยคอมมา)"
            ORCHESTRATOR_NICKNAMES=$(prompt_val "Orchestrator Nicknames" "${ORCHESTRATOR_NICKNAMES:-}")
        fi
    fi
    echo ""

    # ── [3/3] Google Skill Access (Optional) ─────────────────────────────────
    echo -e "${BOLD}[3/3] Google Skill Access (Optional — leave blank for General Worker)${RESET}"
    GOOGLE_EMAIL=$(prompt_val "GOOGLE_EMAIL" "${GOOGLE_EMAIL:-}")
    GOOGLE_APP_PASSWORD=$(prompt_val "GOOGLE_APP_PASSWORD" "${GOOGLE_APP_PASSWORD:-}" "true")
    echo ""

    NODE_ID="$(hostname)-proot"
    OTTOCLAW_MODE="worker"
    OTTOCLAW_HOME="/var/lib/ottoclaw"
    OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"

    mkdir -p /etc/ottoclaw
    cat > /etc/ottoclaw/env << EOF
# ═══════════════════════════════════════════════════════════════
# OttoClaw Worker — Environment Configuration (PRoot)
# Generated on $(date)
# ═══════════════════════════════════════════════════════════════

# ── Agent Identity ─────────────────────────────────────────────
NODE_ID="${NODE_ID}"
AGENT_NAME="${AGENT_NAME}"
ORCHESTRATOR_NICKNAMES="${ORCHESTRATOR_NICKNAMES}"
OTTOCLAW_MODE="${OTTOCLAW_MODE}"

# ── Master Connection ─────────────────────────────────────────
MASTER_URL="${MASTER_URL}"
MASTER_GRPC_URL="${MASTER_GRPC_URL}"
MASTER_API_URL="${MASTER_API_URL}"
MASTER_API_KEY="${MASTER_API_KEY}"
SIAM_MASTER_URL="${SIAM_MASTER_URL}"
SIAM_API_KEY="${SIAM_API_KEY}"
NODE_SECRET="${NODE_SECRET}"

# ── LLM (via Master Proxy — auto-derived) ────────────────────
OTTOCLAW_API_BASE="${OTTOCLAW_API_BASE}"
OTTOCLAW_API_KEY="${OTTOCLAW_API_KEY}"
OTTOCLAW_MODEL_ID="${OTTOCLAW_MODEL_ID}"
OTTOCLAW_MODEL_NAME="${OTTOCLAW_MODEL_NAME}"

# ── Telegram Channel ──────────────────────────────────────────
ORCHESTRATOR_TELEGRAM_TOKEN="${ORCHESTRATOR_TELEGRAM_TOKEN}"
TELEGRAM_BOT_TOKEN="${ORCHESTRATOR_TELEGRAM_TOKEN}"
TELEGRAM_ALLOW_FROM="${TELEGRAM_ALLOW_FROM}"
TELEGRAM_BRIDGE_CHAT_ID="${TELEGRAM_BRIDGE_CHAT_ID}"
TELEGRAM_ORCHESTRATION_ENABLED="${TELEGRAM_ORCHESTRATION_ENABLED}"

# ── Google Skill Access (Optional) ──────────────────────────
GOOGLE_EMAIL="${GOOGLE_EMAIL}"
GOOGLE_APP_PASSWORD="${GOOGLE_APP_PASSWORD}"

# ── Paths ──────────────────────────────────────────────────────
OTTOCLAW_HOME="${OTTOCLAW_HOME}"
OTTOCLAW_WORKSPACE="${OTTOCLAW_WORKSPACE}/v2"
OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"
OTTOCLAW_BIN="/usr/local/bin/ottoclaw-brain"
EOF
    chmod 600 /etc/ottoclaw/env
    info "Environment saved → /etc/ottoclaw/env"
}

install_setup_helper() {
    cat > /usr/local/bin/ottoclaw-setup << 'SETUPEOF'
#!/usr/bin/env bash
source /etc/ottoclaw/env
mkdir -p "${OTTOCLAW_HOME}" "${OTTOCLAW_WORKSPACE}"
cat > "${OTTOCLAW_CONFIG}" << EOF
{
  "agents": { "defaults": { "workspace": "${OTTOCLAW_WORKSPACE}", "model": "default" } },
  "model_list": [ { "model_name": "default", "model": "default", "api_base": "${OTTOCLAW_API_BASE}", "api_key": "${OTTOCLAW_API_KEY}" } ],
  "channels": { "siam_sync": { "enabled": true, "interval": 5, "master_url": "${MASTER_URL}", "api_key": "${MASTER_API_KEY}" } }
}
EOF
SETUPEOF
    chmod +x /usr/local/bin/ottoclaw-setup
}

# ── STEP 3: Build & Wrapper ───────────────────────────────────────────────────
build_binaries() {
    banner "Building OttoClaw Binaries"
    
    # If source is missing, clone it
    if [[ ! -d "${SCRIPT_DIR}/ottoclaw" ]]; then
        warn "Source code not found in ${SCRIPT_DIR}. Downloading from GitHub..."
        local tmp_src="${HOME}/ottoclaw-proot-temp"
        rm -rf "${tmp_src}"
        git clone --depth 1 "https://github.com/${REPO}.git" "${tmp_src}"
        SCRIPT_DIR="${tmp_src}"
    fi

    BIN_DIR="/usr/local/bin"
    
    echo "  Building ottoclaw-brain..."
    if pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null; then
        CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o "${BIN_DIR}/ottoclaw-brain" ./cmd/ottoclaw
        popd >/dev/null
        info "ottoclaw-brain ready."
    else
        err "Could not enter ottoclaw directory."
    fi

    echo "  Building siam-worker..."
    if pushd "${SCRIPT_DIR}/siam-arm" >/dev/null; then
        CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o "${BIN_DIR}/siam-worker" .
        popd >/dev/null
        info "siam-worker ready."
    else
        err "Could not enter siam-arm directory."
    fi
}

install_wrapper() {
    cat > /usr/local/bin/ottoclaw << 'WRAPEOF'
#!/usr/bin/env bash
BRAIN="/usr/local/bin/ottoclaw-brain"
WORKER="/usr/local/bin/siam-worker"
case "${1:-}" in
  start)
    /usr/local/bin/ottoclaw-setup
    nohup ollama serve > /var/lib/ottoclaw/logs/ollama.log 2>&1 &
    sleep 3
    set -o allexport; source /etc/ottoclaw/env; set +o allexport
    nohup "$WORKER" > /var/lib/ottoclaw/logs/siam-worker.log 2>&1 &
    nohup "$BRAIN" gateway --debug > /var/lib/ottoclaw/logs/ottoclaw-brain.log 2>&1 &
    echo "✅ Services started in background."
    ;;
  stop) pkill -f "ottoclaw-brain|siam-worker|ollama serve" || true; echo "✅ Stopped." ;;
  restart) $0 stop; sleep 1; $0 start ;;
  status) pgrep -fl "ottoclaw|siam-worker|ollama" || echo "No services running." ;;
  update)
    echo "🔄 OttoClaw Update"
    INSTALL_SH="$(find /opt/siam-synapse /home -name install-proot.sh -path '*/ottoclaw-worker/*' 2>/dev/null | head -1)"
    if [[ -z "$INSTALL_SH" ]]; then echo "❌ Cannot find install-proot.sh"; exit 1; fi
    REPO_DIR="$(dirname "$INSTALL_SH")"
    
    echo "⏳ ดึงข้อมูลจาก Github ล่าสุด..."
    if [[ -d "${REPO_DIR}/.git" ]]; then
        git config --global --add safe.directory "$REPO_DIR" 2>/dev/null || true
        git -C "$REPO_DIR" pull --ff-only || { echo "❌ git pull failed"; exit 1; }
    fi
    
    echo "🔨 Rebuilding binaries..."
    export CGO_ENABLED=0
    pushd "${REPO_DIR}/ottoclaw" >/dev/null
    go build -buildvcs=false -ldflags="-s -w" -o /tmp/ottoclaw-brain-new ./cmd/ottoclaw
    popd >/dev/null
    
    pushd "${REPO_DIR}/siam-arm" >/dev/null
    go build -buildvcs=false -ldflags="-s -w" -o /tmp/siam-worker-new .
    popd >/dev/null
    
    echo "🛑 หยุดการทำงาน Service ก่อน & Replacing binaries..."
    mv -f /tmp/ottoclaw-brain-new /usr/local/bin/ottoclaw-brain
    mv -f /tmp/siam-worker-new /usr/local/bin/siam-worker
    # Use background command to ensure the restart survives if this script is killed by the stop command
    nohup bash -c "\"$0\" stop 2>/dev/null || true; sleep 2; \"$0\" start" >/dev/null 2>&1 &
    
    echo "✅ Update complete! Services restarting in background..."
    ;;
  *) 
    if [ -f "$BRAIN" ]; then
        exec "$BRAIN" "$@"
    else
        echo "Error: $BRAIN not found. Please run installation again."
        exit 1
    fi
    ;;
esac
WRAPEOF
    chmod +x /usr/local/bin/ottoclaw
}

# ── MAIN ──────────────────────────────────────────────────────────────────────
check_resources
install_deps
run_config_wizard
install_setup_helper
build_binaries
pull_vision_model
install_wrapper

banner "✅ PRoot Installation Ready!"
echo "รัน 'ottoclaw start' เพื่อเริ่มระบบทั้งหมด"
