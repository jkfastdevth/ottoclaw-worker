#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — PRoot Linux Installer (No Systemd)
# ═══════════════════════════════════════════════════════════════════════════════
# สำหรับการใช้งานบน PRoot Distro (เช่น Ubuntu ใน Termux)
# รองรับการติดตั้ง Ollama และรันแบบ Background Process โดยไม่ต้องใช้ systemd
# ═══════════════════════════════════════════════════════════════════════════════
set -uo pipefail

# ── Colors & Helpers ──────────────────────────────────────────────────────────
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
        curl -L https://ollama.com/download/ollama-linux-arm64 -o /usr/local/bin/ollama
        chmod +x /usr/local/bin/ollama
        info "Ollama installed to /usr/local/bin/ollama"
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
        warn "Ollama server failed to start — please run 'ollama pull moondream' manually later."
    fi
    
    pkill -f "ollama serve" || kill $ollama_pid || true
}

# ── STEP 2: Full Config Wizard ────────────────────────────────────────────────
run_config_wizard() {
    banner "Configuration Setup"
    local HIDE_SECRETS="true"
    AGENT_NAME=$(prompt_val "AGENT_NAME" "Kaidos")
    MASTER_HOST=$(prompt_val "MASTER_HOST (IP)" "192.168.1.100")
    MASTER_API_KEY=$(prompt_val "MASTER_API_KEY" "73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90" "true")
    NODE_SECRET=$(prompt_val "NODE_SECRET" "ea710cf8c0f08298e9aa938dff0e0133" "true")

    MASTER_URL="http://${MASTER_HOST}:8080"
    NODE_ID="$(hostname)-proot"
    
    mkdir -p /etc/ottoclaw
    cat > /etc/ottoclaw/env << EOF
NODE_ID="${NODE_ID}"
AGENT_NAME="${AGENT_NAME}"
ORCHESTRATOR_NICKNAMES="${AGENT_NAME}"
OTTOCLAW_MODE="worker"
MASTER_URL="${MASTER_URL}"
MASTER_GRPC_URL="${MASTER_HOST}:50051"
MASTER_API_KEY="${MASTER_API_KEY}"
SIAM_API_KEY="${MASTER_API_KEY}"
NODE_SECRET="${NODE_SECRET}"
OTTOCLAW_API_BASE="${MASTER_URL}/api/agent/v1/llm/proxy"
OTTOCLAW_API_KEY="${MASTER_API_KEY}"
OTTOCLAW_MODEL_NAME="default"
OTTOCLAW_MODEL_ID="default"
OTTOCLAW_HOME="/var/lib/ottoclaw"
OTTOCLAW_WORKSPACE="/var/lib/ottoclaw/workspace/v2"
OTTOCLAW_CONFIG="/var/lib/ottoclaw/config.json"
OTTOCLAW_BIN=/usr/local/bin/ottoclaw-brain
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
