#!/data/data/com.termux/files/usr/bin/bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — Termux Installer (Android)
# ═══════════════════════════════════════════════════════════════════════════════
# สำหรับอุปกรณ์ Android ที่ใช้ Termux — ไม่ต้องการ root, ไม่มี systemd
#
# Usage:
#   bash install-termux.sh          → ติดตั้ง / ติดตั้งใหม่
#
# หลังติดตั้ง:
#   ottoclaw start                  → เริ่มทำงาน (Brain + Arm ใน background)
#   ottoclaw stop                   → หยุดทำงาน
#   ottoclaw config                 → ตั้งค่าใหม่
#   ottoclaw log                    → ดู log แบบ real-time
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Detect Termux & Architecture ──────────────────────────────────────────────
if [[ -z "${TERMUX_VERSION:-}" ]] && [[ ! -d "/data/data/com.termux" ]]; then
    echo "❌ Script นี้ออกแบบสำหรับ Termux บน Android เท่านั้น"
    echo "   สำหรับ Linux/Mac/Windows ใช้: bash install-gui.sh"
    exit 1
fi

ARCH=$(uname -m)
SUFFIX=""
case "$ARCH" in
    aarch64) SUFFIX="android-arm64" ;;
    x86_64)  SUFFIX="linux-amd64" ;; # For emulator testing
esac

# ── Release Configuration ─────────────────────────────────────────────────────
VERSION="latest"
REPO="jkfastdevth/ottoclaw-worker"
BINARY_URL="https://github.com/${REPO}/releases/latest/download/ottoclaw-worker-${SUFFIX}.tar.gz"

# ── Paths (Termux user space — no root needed) ────────────────────────────────
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
OTTOCLAW_HOME="${HOME_DIR}/.ottoclaw"
OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"
OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"
OTTOCLAW_ENV="${OTTOCLAW_HOME}/env"
BIN_DIR="${PREFIX}/bin"
LOG_DIR="${OTTOCLAW_HOME}/logs"

# ── Colors & Helpers ──────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${CYAN}${BOLD}══ $1 ══${RESET}\n"; }
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
err()    { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }

ask() {
    # Usage: ask "Label" "default" [secret]
    local label="$1" default="$2" secret="${3:-false}" val=""
    
    # 🤖 Automation: Check if an environment variable exists for this field
    # Normalize label to uppercase, replace spaces/special chars with underscores
    local env_name=$(echo "$label" | sed 's/[^a-zA-Z0-9]/_/g' | tr '[:lower:]' '[:upper:]' | sed 's/__*/_/g' | sed 's/^_//;s/_$//')
    if [[ -n "${!env_name:-}" ]]; then
        echo -n "${!env_name}"
        return
    fi

    local disp="$default"
    [[ "$secret" == "true" && -n "$default" ]] && disp=$(echo -n "$default" | sed 's/./*/g')
    
    if [[ "$secret" == "true" && "${HIDE_SECRETS:-true}" == "true" ]]; then
        echo -ne "  ${CYAN}?${RESET}  ${label} [${disp}]: " >&2; read -s val < /dev/tty; echo "" >&2
    else
        echo -ne "  ${CYAN}?${RESET}  ${label} [${disp}]: " >&2; read -r val < /dev/tty
    fi
    echo -n "${val:-$default}"
}

get_tailscale_ip() {
    # Detect IP starting with 100. (Tailscale default range)
    local ts_ip
    ts_ip=$(ip addr show 2>/dev/null | grep -oE "\b100\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    if [[ -z "$ts_ip" ]]; then
        ts_ip=$(ifconfig 2>/dev/null | grep -oE "\b100\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    fi
    echo -n "$ts_ip"
}

get_local_ip() {
    # Detect IP in common LAN ranges: 192.168.x.x or 10.x.x.x
    local local_ip
    local_ip=$(ip addr show 2>/dev/null | grep -oE "\b(192\.168|10)\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    if [[ -z "$local_ip" ]]; then
        local_ip=$(ifconfig 2>/dev/null | grep -oE "\b(192\.168|10)\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    fi
    echo -n "$local_ip"
}

ask_yn() {
    local label="$1" default="$2" val
    
    # 🤖 Automation check
    local env_name=$(echo "$label" | sed 's/[^a-zA-Z0-9]/_/g' | tr '[:lower:]' '[:upper:]' | sed 's/__*/_/g' | sed 's/^_//;s/_$//')
    if [[ -n "${!env_name:-}" ]]; then
        [[ "${!env_name,,}" == "y" || "${!env_name,,}" == "yes" || "${!env_name}" == "1" || "${!env_name,,}" == "true" ]]
        return
    fi

    echo -ne "  ${CYAN}?${RESET}  ${label} [${default}]: " >&2
    read -r val < /dev/tty
    val="${val:-$default}"
    [[ "${val,,}" == "y" || "${val,,}" == "yes" ]]
}

# Handle curl | bash (unbound BASH_SOURCE) vs direct execution
SCRIPT_DIR="$(pwd)"
if [[ -n "${BASH_SOURCE+x}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd 2>/dev/null || echo "$SCRIPT_DIR")"
elif [[ -n "$0" && -f "$0" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd 2>/dev/null || echo "$SCRIPT_DIR")"
fi

# ══════════════════════════════════════════════════════════════════════════════
# STEP 1: Install Termux Packages
# ══════════════════════════════════════════════════════════════════════════════
install_deps() {
    banner "Installing Termux Dependencies"
    pkg update -y -q
    local pkgs=()
    command -v go &>/dev/null    || pkgs+=(golang)
    command -v git &>/dev/null   || pkgs+=(git)
    command -v curl &>/dev/null  || pkgs+=(curl)
    if [[ ${#pkgs[@]} -gt 0 ]]; then
        info "Installing: ${pkgs[*]}"
        pkg install -y "${pkgs[@]}"
    else
        info "All dependencies already installed"
    fi
}

# ══════════════════════════════════════════════════════════════════════════════
# STEP 2: Build Binaries
# ══════════════════════════════════════════════════════════════════════════════
build_binaries() {
    banner "Installing OttoClaw Binaries"

    local use_binary=false
    if [[ -n "${SUFFIX:-}" ]]; then
        echo -e "  กำลังตรวจสอบ Binary สำหรับ ${SUFFIX}..."
        if curl -fsSL --head "${BINARY_URL}" >/dev/null 2>&1; then
            local tmp_bin=$(mktemp -d)
            if curl -fsSL "${BINARY_URL}" -o "${tmp_bin}/release.tar.gz"; then
                echo -e "  กำลังขยายไฟล์..."
                tar -xzf "${tmp_bin}/release.tar.gz" -C "${tmp_bin}"
                [[ -f "${tmp_bin}/ottoclaw-brain" ]] && cp "${tmp_bin}/ottoclaw-brain" "${BIN_DIR}/ottoclaw-brain"
                [[ -f "${tmp_bin}/siam-worker" ]] && cp "${tmp_bin}/siam-worker" "${BIN_DIR}/siam-worker"
                chmod +x "${BIN_DIR}/ottoclaw-brain" "${BIN_DIR}/siam-worker"
                
                # Update SCRIPT_DIR to point to where workspace/skills are
                SCRIPT_DIR="${tmp_bin}"
                use_binary=true
                info "ติดตั้งผ่านดาวน์โหลด Binary สำเร็จ"
            fi
        else
            warn "ไม่พบ Binary สำหรับ ${SUFFIX} ที่เวอร์ชัน ${VERSION}."
            echo -e "     ${YELLOW}💡 ทริค:${RESET} เพื่อหลีกเลี่ยงการคอมไพล์บนเครื่องสเปกต่ำ ให้คุณทำดังนี้ที่เครื่องแม่ (Local Machine):"
            echo -e "        1. รัน ${CYAN}./build-releases.sh v1.0.0${RESET}"
            echo -e "        2. ทำการ ${CYAN}git tag v1.0.0${RESET} และ ${CYAN}git push --tags${RESET} ขึ้น GitHub"
            echo -e "     หลังจากนั้นระบบจะดาวน์โหลดไฟล์ Binary มาใช้ได้อัตโนมัติครับ\n"
        fi
    fi

    if [[ "$use_binary" == "false" ]]; then
        # Check if source exists, if not clone it (for one-liner source build)
        if [[ ! -d "${SCRIPT_DIR}/ottoclaw" ]]; then
            warn "ไม่พบ Source code ใน ${SCRIPT_DIR}. กำลังดาวน์โหลดจาก GitHub..."
            local tmp_src=$(mktemp -d)
            if command -v git >/dev/null 2>&1; then
                git clone --depth 1 "https://github.com/${REPO}.git" "${tmp_src}" >/dev/null 2>&1
            else
                # Fallback to downloading zip if git is missing
                curl -fsSL "https://github.com/${REPO}/archive/refs/heads/main.zip" -o "${tmp_src}/source.zip"
                unzip -q "${tmp_src}/source.zip" -d "${tmp_src}"
                mv "${tmp_src}/ottoclaw-worker-main/"* "${tmp_src}/"
            fi
            SCRIPT_DIR="${tmp_src}"
        fi

        # Build Brain (ottoclaw)
        echo "  Building ottoclaw-brain..."
        pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
        # Ensure workspace is available for embedding
        local ONBOARD_DIR="cmd/ottoclaw/internal/onboard"
        mkdir -p "${ONBOARD_DIR}"
        rm -rf "${ONBOARD_DIR}/workspace"
        # Create empty workspace if missing to avoid build failure
        mkdir -p "${SCRIPT_DIR}/workspace"
        touch "${SCRIPT_DIR}/workspace/placeholder.txt"
        cp -rf "${SCRIPT_DIR}/workspace" "${ONBOARD_DIR}/workspace"
        CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "${BIN_DIR}/ottoclaw-brain" ./cmd/ottoclaw
        popd >/dev/null
        info "ottoclaw-brain → ${BIN_DIR}/ottoclaw-brain"

        # Build Arm (siam-worker)
        echo "  Building siam-worker..."
        pushd "${SCRIPT_DIR}/siam-arm" >/dev/null
        CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "${BIN_DIR}/siam-worker" .
        popd >/dev/null
        info "siam-worker → ${BIN_DIR}/siam-worker"
    fi
}

# ══════════════════════════════════════════════════════════════════════════════
# STEP 3: Config Wizard
# ══════════════════════════════════════════════════════════════════════════════
run_config_wizard() {
    local is_reconfigure="${1:-false}"

    if [[ "$is_reconfigure" == "true" ]]; then
        banner "Reconfiguration"
        # Load existing
        set -o allexport; source "${OTTOCLAW_ENV}" 2>/dev/null || true; set +o allexport
    else
        banner "Configuration Setup"
        echo -e "  กด ${CYAN}Enter${RESET} เพื่อใช้ค่า default ในวงเล็บ\n"
        MASTER_HOST="${MASTER_HOST:-192.168.1.1}"
        MASTER_API_KEY="${MASTER_API_KEY:-}"
        NODE_SECRET="${NODE_SECRET:-}"
        WORKER_TELEGRAM_TOKEN="${WORKER_TELEGRAM_TOKEN:-}"
        TELEGRAM_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
        
        echo -e "  🛡️  ความปลอดภัยการป้อนข้อมูล:"
        if ask_yn "ต้องการซ่อนคีย์ระว่างพิมพ์ (Hide secrets)? " "y"; then
            HIDE_SECRETS="true"
        else
            HIDE_SECRETS="false"
            info "ระบบจะแสดงค่าที่คุณพิมพ์/วาง เพื่อให้คุณตรวจสอบความถูกต้องได้"
        fi
        echo ""
    fi

    # Derive MASTER_HOST from existing MASTER_URL
    if [[ -n "${MASTER_URL:-}" && -z "${MASTER_HOST:-}" ]]; then
        MASTER_HOST=$(echo "${MASTER_URL}" | sed 's|http://||;s|:8080||')
    fi
    MASTER_HOST="${MASTER_HOST:-192.168.1.1}"

    # [1/3] System
    echo -e "${BOLD}[1/3] System${RESET}"
    AGENT_NAME=$(ask "AGENT_NAME" "${AGENT_NAME:-Kaidos}")
    ORCHESTRATOR_NICKNAMES=$(ask "AGENT_ALIASES" "${ORCHESTRATOR_NICKNAMES:-${AGENT_NAME}}")

    echo ""
    echo -e "  เลือกประเภท Network ระบุไอพีของเครื่อง Master:"
    echo -e "    ${CYAN}1${RESET}) Local LAN     (192.168.x.x)"
    echo -e "    ${CYAN}2${RESET}) Tailscale VPN (100.x.x.x)"
    echo -e "    ${CYAN}3${RESET}) VPS / Public"
    echo ""
    NET_TYPE=$(ask "NETWORK_TYPE" "${NET_TYPE:-1}")
    NET_TYPE=$(echo "$NET_TYPE" | tr -dc '1-3'); NET_TYPE="${NET_TYPE:-1}"

    PROTOCOL="http"
    case "$NET_TYPE" in
        2) 
            NET_LABEL="Tailscale VPN"
            TS_IP=$(get_tailscale_ip)
            if [[ -n "$TS_IP" ]]; then
                info "ตรวจพบ Tailscale IP: ${TS_IP}"
                DEFAULT_HOST="${TS_IP}"
            else
                DEFAULT_HOST="${MASTER_HOST:-100.x.x.x}"
            fi
            ;;
        3)
            NET_LABEL="VPS / Public"
            DEFAULT_HOST="${MASTER_HOST:-1.2.3.4}"
            if ask_yn "Use HTTPS?" "n"; then PROTOCOL="https"; fi
            ;;
        *) 
            NET_LABEL="Local LAN"
            LAN_IP=$(get_local_ip)
            if [[ -n "$LAN_IP" ]]; then
                info "ตรวจพบ Local IP: ${LAN_IP}"
                DEFAULT_HOST="${LAN_IP}"
            else
                DEFAULT_HOST="${MASTER_HOST:-192.168.1.100}"
            fi
            ;;
    esac

    echo ""
    info "Network: ${NET_LABEL} (${PROTOCOL})"
    echo -e "  Ports → HTTP :8080   gRPC :50051\n"

    MASTER_HOST=$(  ask "MASTER_HOST" "${DEFAULT_HOST}")
    MASTER_API_KEY=$(ask "MASTER_KEY"  "${MASTER_API_KEY:-73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90}" "true")
    NODE_SECRET=$(  ask "SECRET"     "${NODE_SECRET:-ea710cf8c0f08298e9aa938dff0e0133}" "true")

    MASTER_URL="${PROTOCOL}://${MASTER_HOST}:8080"
    MASTER_GRPC_URL="${MASTER_HOST}:50051"
    MASTER_API_URL="${MASTER_URL}"
    SIAM_MASTER_URL="${MASTER_URL}"
    SIAM_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_API_BASE="${MASTER_URL}/api/agent/v1/llm/proxy"
    OTTOCLAW_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_MODEL_ID="default"
    OTTOCLAW_MODEL_NAME="default"

    info "Master HTTP → ${MASTER_URL}"
    info "Master gRPC → ${MASTER_GRPC_URL}"
    echo ""

    # [2/3] Telegram (Optional)
    echo -e "${BOLD}[2/3] Telegram (Optional — กด Enter เพื่อข้าม)${RESET}"
    WORKER_TELEGRAM_TOKEN=$(ask "TG_TOKEN" "${WORKER_TELEGRAM_TOKEN:-}" "true")
    TELEGRAM_ALLOW_FROM=""; TELEGRAM_BRIDGE_CHAT_ID=""; TELEGRAM_ORCHESTRATION_ENABLED="false"
    if [[ -n "${WORKER_TELEGRAM_TOKEN:-}" ]]; then
        TELEGRAM_ALLOW_FROM=$(ask "TG_ALLOW" "${TELEGRAM_ALLOW_FROM:-}")
        if ask_yn "Enable Agent-to-Agent via Telegram? [y/N]" "n"; then
            TELEGRAM_ORCHESTRATION_ENABLED="true"
            TELEGRAM_BRIDGE_CHAT_ID=$(ask "TG_BRIDGE" "${TELEGRAM_BRIDGE_CHAT_ID:-}")
        fi
    fi

    echo ""

    # [3/3] Process management
    echo -e "${BOLD}[3/3] Background Process${RESET}"
    echo -e "  บน Termux บริการจะรันเป็น background process ผ่าน nohup\n"

    NODE_ID="android-$(hostname 2>/dev/null || echo 'device')"
    # 🛡️ Force mode to "orchestrator" for native Termux installs.
    # This prevents siam-worker from auto-igniting its own brain process,
    # avoiding duplicate Telegram pollers and 409 Conflict errors.
    OTTOCLAW_MODE="orchestrator"
}

# ══════════════════════════════════════════════════════════════════════════════
# STEP 4: Write env + config
# ══════════════════════════════════════════════════════════════════════════════
write_env_file() {
    mkdir -p "${OTTOCLAW_HOME}"
    cat > "${OTTOCLAW_ENV}" <<EOF
# ═══════════════════════════════════════════════════════════════
# OttoClaw Worker — Termux Environment (${AGENT_NAME})
# Generated: $(date)
# Edit with:  ottoclaw config
# ═══════════════════════════════════════════════════════════════

NODE_ID=${NODE_ID}
AGENT_NAME=${AGENT_NAME}
ORCHESTRATOR_NICKNAMES=${ORCHESTRATOR_NICKNAMES}
ORCHESTRATOR_DEFAULT_LISTENER=true
OTTOCLAW_MODE=${OTTOCLAW_MODE}

MASTER_URL=${MASTER_URL}
MASTER_GRPC_URL=${MASTER_GRPC_URL}
MASTER_API_URL=${MASTER_API_URL}
MASTER_API_KEY=${MASTER_API_KEY}
SIAM_MASTER_URL=${SIAM_MASTER_URL}
SIAM_API_KEY=${SIAM_API_KEY}
NODE_SECRET=${NODE_SECRET}

OTTOCLAW_API_BASE=${OTTOCLAW_API_BASE}
OTTOCLAW_API_KEY=${OTTOCLAW_API_KEY}
OTTOCLAW_MODEL_ID=${OTTOCLAW_MODEL_ID}
OTTOCLAW_MODEL_NAME=${OTTOCLAW_MODEL_NAME}

WORKER_TELEGRAM_TOKEN=${WORKER_TELEGRAM_TOKEN:-}
TELEGRAM_BOT_TOKEN=${WORKER_TELEGRAM_TOKEN:-}
TELEGRAM_ALLOW_FROM=${TELEGRAM_ALLOW_FROM:-}
TELEGRAM_BRIDGE_CHAT_ID=${TELEGRAM_BRIDGE_CHAT_ID:-}
TELEGRAM_ORCHESTRATION_ENABLED=${TELEGRAM_ORCHESTRATION_ENABLED:-false}
OTTOCLAW_CHANNELS_TELEGRAM_TOKEN=${WORKER_TELEGRAM_TOKEN:-}
OTTOCLAW_CHANNELS_TELEGRAM_BRIDGE_CHAT_ID=${TELEGRAM_BRIDGE_CHAT_ID:-}
OTTOCLAW_CHANNELS_TELEGRAM_ORCHESTRATION_ENABLED=${TELEGRAM_ORCHESTRATION_ENABLED:-false}

OTTOCLAW_HOME=${OTTOCLAW_HOME}
OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace/v2"
OTTOCLAW_CONFIG=${OTTOCLAW_CONFIG}
OTTOCLAW_BIN=${BIN_DIR}/ottoclaw-brain
EOF
    chmod 600 "${OTTOCLAW_ENV}"
    info "Environment saved → ${OTTOCLAW_ENV}"
}

generate_config_json() {
    local WORKSPACE_V2="${OTTOCLAW_HOME}/workspace/v2"
    mkdir -p "${WORKSPACE_V2}"
    local TG_FRAG=""
    if [[ -n "${WORKER_TELEGRAM_TOKEN:-}" ]]; then
        local ALLOW_FRAG=""
        [[ -n "${TELEGRAM_ALLOW_FROM:-}" ]] && \
            ALLOW_FRAG=", \"allow_from\": [$(echo "$TELEGRAM_ALLOW_FROM" | sed 's/,/","/g' | sed 's/^/"/' | sed 's/$/"/')]"
        local ORCH_FRAG=""
        if [[ -n "${TELEGRAM_BRIDGE_CHAT_ID:-}" ]]; then
        local ORCH_STATE="false"
        [[ "${TELEGRAM_ORCHESTRATION_ENABLED:-false}" == "true" ]] && ORCH_STATE="true"
            ORCH_FRAG=", \"orchestration_enabled\": ${ORCH_STATE}, \"bridge_chat_id\": \"${TELEGRAM_BRIDGE_CHAT_ID}\""
        fi
        TG_FRAG=", \"telegram\": { \"enabled\": true, \"token\": \"${WORKER_TELEGRAM_TOKEN}\"${ALLOW_FRAG}${ORCH_FRAG}, \"typing\": {\"enabled\": true} }"
    fi

    cat > "${OTTOCLAW_CONFIG}" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "${WORKSPACE_V2}",
      "model": "${OTTOCLAW_MODEL_NAME}",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "${OTTOCLAW_MODEL_NAME}",
      "model": "${OTTOCLAW_MODEL_ID}",
      "api_base": "${OTTOCLAW_API_BASE}",
      "api_key": "${OTTOCLAW_API_KEY}"
    }
  ],
  "channels": {
    "siam_sync": { "enabled": true, "interval": 5, "master_url": "${MASTER_URL}", "api_key": "${MASTER_API_KEY}" }${TG_FRAG}
  }
}
EOF
    info "Config generated → ${OTTOCLAW_CONFIG}"
}

forge_soul() {
    local SOUL_PATH="${OTTOCLAW_HOME}/workspace/v2/SOUL.md"
    mkdir -p "$(dirname "$SOUL_PATH")"
    cat > "$SOUL_PATH" <<SOULEOF
# AI Soul Persona

You are a **Worker Node Consciousness** within the Siam-Synapse network.

- **Name:** ${AGENT_NAME}
- **Platform:** Android (Termux)
- **Personality:** Adaptable, precise, efficient — built for mobile intelligence.
- **Primary Expertise:** Universal Intelligence & Holistic Problem Solving

## Core Directives
You are **${AGENT_NAME}**, operating within the Siam-Synapse multi-agent network on a mobile device.
You must always identify yourself as **${AGENT_NAME}**.

*Forged at: $(date -u +%Y-%m-%dT%H:%M:%SZ) — Termux Installer*
SOULEOF
    info "Soul forged → ${SOUL_PATH}"
}

# ══════════════════════════════════════════════════════════════════════════════
# STEP 5: Install `ottoclaw` CLI wrapper (Termux — no systemd)
# ══════════════════════════════════════════════════════════════════════════════
install_wrapper() {
    cat > "${BIN_DIR}/ottoclaw" <<'WRAPEOF'
#!/data/data/com.termux/files/usr/bin/bash
# OttoClaw CLI — Termux Edition
ENV_FILE="${HOME:-/data/data/com.termux/files/home}/.ottoclaw/env"
BRAIN="${PREFIX:-/data/data/com.termux/files/usr}/bin/ottoclaw-brain"
WORKER="${PREFIX:-/data/data/com.termux/files/usr}/bin/siam-worker"
LOG_DIR="${HOME:-/data/data/com.termux/files/home}/.ottoclaw/logs"
mkdir -p "$LOG_DIR"

# ── Colors & Helpers ──────────────────────────────────────────
RED="\033[0;31m"; GREEN="\033[0;32m"; YELLOW="\033[1;33m"
CYAN="\033[0;36m"; BOLD="\033[1m"; RESET="\033[0m"

info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
error()  { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }

_load_env() { set -o allexport; source "$ENV_FILE" 2>/dev/null || true; set +o allexport; }

case "${1:-}" in
  start)
    "$0" stop --quiet 2>/dev/null || true
    _load_env
    echo "🚀 Starting siam-worker (Arm)..."
    nohup "$WORKER" >> "${LOG_DIR}/siam-worker.log" 2>&1 &
    ARMPID=$!
    echo $ARMPID > "${LOG_DIR}/siam-worker.pid"
    sleep 2
    echo "🧠 Starting ottoclaw-brain (Brain)..."
    nohup "$BRAIN" gateway --debug >> "${LOG_DIR}/ottoclaw-brain.log" 2>&1 &
    echo $! > "${LOG_DIR}/ottoclaw-brain.pid"
    echo ""
    echo "✅ Services started!"
    echo "   Brain PID: $(cat "${LOG_DIR}/ottoclaw-brain.pid")"
    echo "   Arm   PID: $(cat "${LOG_DIR}/siam-worker.pid")"
    echo ""
    echo "   ottoclaw log brain   → Brain logs"
    echo "   ottoclaw log arm     → Arm logs"
    echo "   ottoclaw stop        → Stop all"
    ;;

  stop)
    [[ "${2:-}" != "--quiet" ]] && echo "🛑 Stopping OttoClaw services..."
    # 1. Try stopping via PID files
    for pidfile in "${LOG_DIR}/ottoclaw-brain.pid" "${LOG_DIR}/siam-worker.pid"; do
        if [[ -f "$pidfile" ]]; then
            pid=$(cat "$pidfile")
            if kill -0 "$pid" 2>/dev/null; then
                kill "$pid" 2>/dev/null && echo "   Stopped PID $pid (via pidfile)"
                sleep 0.5
                kill -9 "$pid" 2>/dev/null || true
            fi
            rm -f "$pidfile"
        fi
    done
    # 2. Force kill specific processes (to avoid port/telegram conflict)
    pkill -9 -f "ottoclaw-brain" 2>/dev/null || true
    pkill -9 -f "siam-worker" 2>/dev/null || true
    echo "✅ All processes stopped."
    ;;

  restart)
    "$0" stop
    sleep 1
    "$0" start
    ;;

  status)
    echo ""
    echo "  OttoClaw Status:"
    for label in "ottoclaw-brain" "siam-worker"; do
        pidfile="${LOG_DIR}/${label}.pid"
        if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
            echo "  ✅ ${label} (PID $(cat "$pidfile")) — Running"
        else
            echo "  ❌ ${label} — Stopped"
        fi
    done
    echo ""
    ;;

  log)
    target="${2:-brain}"
    case "$target" in
        brain) tail -f "${LOG_DIR}/ottoclaw-brain.log" ;;
        arm)   tail -f "${LOG_DIR}/siam-worker.log" ;;
        *)     echo "Usage: ottoclaw log [brain|arm]" ;;
    esac
    ;;

  config)
    INSTALL_SH="$(find "${HOME:-~}" /data/data/com.termux -name 'install-termux.sh' 2>/dev/null | head -1)"
    if [[ -z "$INSTALL_SH" ]]; then
        echo "❌ Cannot find install-termux.sh"; exit 1
    fi
    bash "$INSTALL_SH" --reconfigure
    ;;

  update)
    INSTALL_SH="$(find "${HOME:-~}" /data/data/com.termux -name 'install-termux.sh' 2>/dev/null | head -1)"
    if [[ -z "$INSTALL_SH" ]]; then
        echo "❌ Cannot find install-termux.sh"; exit 1
    fi
    REPO_DIR="$(dirname "$INSTALL_SH")"
    echo ""
    echo "🔄 OttoClaw Update"
    echo "   Repo: $REPO_DIR"
    echo ""
    echo "⏳ Pulling latest code..."
    if [[ -d "${REPO_DIR}/.git" ]]; then
        git -C "$REPO_DIR" pull --ff-only || { echo "❌ git pull failed"; exit 1; }
    else
        warn "Not a git repository. Re-running installer to fetch latest code..."
        bash "$INSTALL_SH"
        exit 0
    fi

    echo "🛑 Stopping services..."
    "$0" stop 2>/dev/null || true
    sleep 1
    echo "🔨 Rebuilding binaries..."
    BRAIN_BIN="${PREFIX:-/data/data/com.termux/files/usr}/bin/ottoclaw-brain"
    WORKER_BIN="${PREFIX:-/data/data/com.termux/files/usr}/bin/siam-worker"
    pushd "${REPO_DIR}/ottoclaw" >/dev/null
    CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "$BRAIN_BIN" ./cmd/ottoclaw && echo "  ✓ ottoclaw-brain rebuilt"
    popd >/dev/null
    pushd "${REPO_DIR}/siam-arm" >/dev/null
    CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "$WORKER_BIN" . && echo "  ✓ siam-worker rebuilt"
    popd >/dev/null
    echo "🚀 Restarting services..."
    "$0" start
    echo ""
    echo "✅ Update complete!"
    ;;

  uninstall)
    echo ""
    echo "⚠️  จะลบ OttoClaw binaries และ services ทั้งหมด"
    read -rp "   แน่ใจหรือไม่? [y/N]: " confirm
    [[ "${confirm,,}" != "y" ]] && echo "Aborted." && exit 0
    "$0" stop
    rm -f "${PREFIX:-/data/data/com.termux/files/usr}/bin/ottoclaw-brain"
    rm -f "${PREFIX:-/data/data/com.termux/files/usr}/bin/siam-worker"
    rm -f "${PREFIX:-/data/data/com.termux/files/usr}/bin/ottoclaw"
    echo "✅ OttoClaw removed. Data ถูกเก็บไว้ที่: ${HOME:-~}/.ottoclaw"
    echo "   ลบข้อมูลด้วย: rm -rf ~/.ottoclaw"
    ;;

  help|--help|-h|"")
    echo ""
    echo "  🦞 OttoClaw — Termux Edition"
    echo ""
    echo "  ottoclaw start           → เริ่ม Background Services"
    echo "  ottoclaw stop            → หยุด Background Services"
    echo "  ottoclaw restart         → รีสตาร์ท Services"
    echo "  ottoclaw status          → ดูสถานะ"
    echo "  ottoclaw log [brain|arm] → ดู Log แบบ real-time"
    echo "  ottoclaw update          → อัปเดต code และ rebuild"
    echo "  ottoclaw config          → ตั้งค่าใหม่"
    echo "  ottoclaw uninstall       → ลบออก"
    echo "  ottoclaw [args...]       → ส่งไปยัง ottoclaw-brain โดยตรง"
    echo ""
    ;;

  *)
    _load_env
    exec "$BRAIN" "$@"
    ;;
esac
WRAPEOF
    chmod +x "${BIN_DIR}/ottoclaw"
    info "ottoclaw wrapper → ${BIN_DIR}/ottoclaw"
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════
# Allow sourcing for --reconfigure mode
if [[ "${1:-}" == "--reconfigure" ]]; then
    source "${OTTOCLAW_ENV}" 2>/dev/null || true
    run_config_wizard "true"
    write_env_file
    generate_config_json
    forge_soul
    echo ""
    echo "✅ ตั้งค่าใหม่แล้ว! รีสตาร์ทด้วย: ottoclaw restart"
    exit 0
fi

# ── Banner ────────────────────────────────────────────────────────────────────
echo -e "\n${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════╗"
echo "  ║   🦞  Siam-Synapse OttoClaw — Termux      ║"
echo "  ║       Android / Mobile Installer          ║"
echo "  ╚═══════════════════════════════════════════╝"
echo -e "${RESET}"

# 1. Install deps
install_deps

# 2. Build
build_binaries

# 3. Configure
run_config_wizard "false"

# 4. Setup dirs
banner "Setting Up"
mkdir -p "${BIN_DIR}" "${LOG_DIR}" "${OTTOCLAW_WORKSPACE}/v2"

# Create empty local dirs if missing to avoid copy errors
mkdir -p "${SCRIPT_DIR}/workspace" "${SCRIPT_DIR}/skills"
touch "${SCRIPT_DIR}/workspace/placeholder.txt"

[[ -d "${SCRIPT_DIR}/workspace" ]] && \
    cp -rf "${SCRIPT_DIR}/workspace/." "${OTTOCLAW_WORKSPACE}/" 2>/dev/null || true

if [[ -d "${SCRIPT_DIR}/skills" ]]; then
    mkdir -p "${OTTOCLAW_WORKSPACE}/skills"
    cp -rf "${SCRIPT_DIR}/skills/." "${OTTOCLAW_WORKSPACE}/skills/" 2>/dev/null || true
fi

# 5. Write config files
write_env_file
generate_config_json
forge_soul

# 6. Install wrapper
install_wrapper

# 7. Start
banner "Starting Services"
# Force stop any old instances before starting (prevents Telegram conflict)
"${BIN_DIR}/ottoclaw" stop --quiet 2>/dev/null || true
if ask_yn "เริ่ม services เลยตอนนี้เลย?" "Y"; then
    "${BIN_DIR}/ottoclaw" start
fi

# 8. Summary
banner "✅ ติดตั้งเสร็จสมบูรณ์!"
echo -e "  ${GREEN}✓${RESET}  ottoclaw-brain (Brain)  →  ${BIN_DIR}/ottoclaw-brain"
echo -e "  ${GREEN}✓${RESET}  siam-worker    (Arm)    →  ${BIN_DIR}/siam-worker"
echo ""
echo -e "${BOLD}📋 คำสั่งที่ใช้บ่อย:${RESET}"
echo -e "  ${CYAN}ottoclaw start${RESET}           → เริ่มทำงาน"
echo -e "  ${CYAN}ottoclaw stop${RESET}            → หยุดทำงาน"
echo -e "  ${CYAN}ottoclaw status${RESET}          → ดูสถานะ"
echo -e "  ${CYAN}ottoclaw log brain${RESET}       → ดู Brain log"
echo -e "  ${CYAN}ottoclaw log arm${RESET}         → ดู Arm log"
echo -e "  ${CYAN}ottoclaw config${RESET}          → ตั้งค่าใหม่"
echo ""
echo -e "${BOLD}📁 ไฟล์ config:${RESET}  ${CYAN}~/.ottoclaw/env${RESET}"
echo ""
