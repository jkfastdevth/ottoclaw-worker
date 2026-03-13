#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# 🦞 Siam-Synapse OttoClaw Worker — GUI Installer
#    สำหรับ: Linux (GNOME/KDE), macOS, Windows (WSL/Git Bash)
# ═══════════════════════════════════════════════════════════════════════════════
# ใช้ UI dialogs แบบ native ของแต่ละ OS:
#   Linux  → Zenity (GNOME) / kdialog (KDE) / yad / terminal fallback
#   macOS  → osascript (AppleScript dialogs)
#   Windows WSL/Git Bash → PowerShell InputBox / terminal fallback
#
# Usage:
#   Linux/Mac:   bash install-gui.sh
#   Windows WSL: bash install-gui.sh
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── Auto-load Credentials from .env ───────────────────────────────
if [[ -f "${REPO_ROOT}/.env" ]]; then
    # Try to extract keys if not already set
    [[ -z "${MASTER_API_KEY:-}" ]] && MASTER_API_KEY=$(grep "^MASTER_API_KEY=" "${REPO_ROOT}/.env" | cut -d'=' -f2- | tr -d '\r')
    [[ -z "${NODE_SECRET:-}"   ]] && NODE_SECRET=$(grep "^NODE_SECRET=" "${REPO_ROOT}/.env" | cut -d'=' -f2- | tr -d '\r')
fi

# ── Detect OS & Architecture ──────────────────────────────────────────────────
OS_TYPE="$(uname -s)"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  GO_ARCH="amd64" ;;
    aarch64|arm64) GO_ARCH="arm64" ;;
    armv7l)  GO_ARCH="arm" ;;
    *)       GO_ARCH="amd64" ;;
esac

case "$OS_TYPE" in
    Linux*)  PLATFORM="linux" ;;
    Darwin*) PLATFORM="darwin" ;;
    MINGW*|CYGWIN*|MSYS*) PLATFORM="windows" ;;
    *)       PLATFORM="linux" ;;
esac

GUI_ENGINE=""   # zenity / kdialog / yad / osascript / powershell / terminal

# ── Colors (terminal fallback) ────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
banner() { echo -e "\n${CYAN}${BOLD}══ $1 ══${RESET}\n"; }
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
err()    { echo -e "  ${RED}✗${RESET}  $1\n"; exit 1; }

# ══════════════════════════════════════════════════════════════════════════════
# GUI ENGINE DETECTION
# ══════════════════════════════════════════════════════════════════════════════
detect_gui_engine() {
    if [[ "$PLATFORM" == "darwin" ]]; then
        GUI_ENGINE="osascript"
    elif [[ "$PLATFORM" == "windows" ]]; then
        if command -v powershell.exe &>/dev/null; then
            GUI_ENGINE="powershell"
        else
            GUI_ENGINE="terminal"
        fi
    else
        # Linux — try in order: zenity > yad > kdialog > terminal
        if command -v zenity &>/dev/null; then
            GUI_ENGINE="zenity"
        elif command -v yad &>/dev/null; then
            GUI_ENGINE="yad"
        elif command -v kdialog &>/dev/null; then
            GUI_ENGINE="kdialog"
        else
            GUI_ENGINE="terminal"
        fi
    fi
    info "GUI engine: ${GUI_ENGINE}"
}

# ══════════════════════════════════════════════════════════════════════════════
# CROSS-PLATFORM DIALOG FUNCTIONS
# ══════════════════════════════════════════════════════════════════════════════

# gui_input "Title" "Label" "Default" → prints value
gui_input() {
    local title="$1" label="$2" default="$3" result=""
    case "$GUI_ENGINE" in
        zenity)
            result=$(zenity --entry --title="$title" --text="$label" --entry-text="$default" 2>/dev/null) || result="$default"
            ;;
        yad)
            result=$(yad --entry --title="$title" --text="$label" --entry-text="$default" 2>/dev/null) || result="$default"
            ;;
        kdialog)
            result=$(kdialog --title "$title" --inputbox "$label" "$default" 2>/dev/null) || result="$default"
            ;;
        osascript)
            result=$(osascript -e "text returned of (display dialog \"${label}\" default answer \"${default}\" with title \"${title}\")" 2>/dev/null) || result="$default"
            ;;
        powershell)
            result=$(powershell.exe -Command "Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.Interaction]::InputBox('${label}', '${title}', '${default}')" 2>/dev/null | tr -d '\r') || result="$default"
            ;;
        terminal|*)
            echo -ne "  ${CYAN}?${RESET}  ${label} [${default}]: " >&2; read -r result
            result="${result:-$default}"
            ;;
    esac
    echo -n "$result"
}

# gui_password "Title" "Label" → prints value
gui_password() {
    local title="$1" label="$2" result=""
    case "$GUI_ENGINE" in
        zenity)
            result=$(zenity --password --title="$title" --text="$label" 2>/dev/null) || result=""
            ;;
        yad)
            result=$(yad --entry --title="$title" --text="$label" --hide-text 2>/dev/null) || result=""
            ;;
        kdialog)
            result=$(kdialog --title "$title" --password "$label" 2>/dev/null) || result=""
            ;;
        osascript)
            result=$(osascript -e "text returned of (display dialog \"${label}\" default answer \"\" with title \"${title}\" with hidden answer)" 2>/dev/null) || result=""
            ;;
        powershell)
            # PowerShell masked input via SecureString
            result=$(powershell.exe -Command "
                \$secure = Read-Host '${label}' -AsSecureString
                [System.Runtime.InteropServices.Marshal]::PtrToStringAuto([System.Runtime.InteropServices.Marshal]::SecureStringToBSTR(\$secure))
            " 2>/dev/null | tr -d '\r') || result=""
            ;;
        terminal|*)
            echo -ne "  ${CYAN}?${RESET}  ${label}: " >&2; read -rs result; echo "" >&2
            ;;
    esac
    echo -n "$result"
}

# gui_select "Title" "Label" "opt1" "opt2" ... → prints selected option
gui_select() {
    local title="$1" label="$2"; shift 2
    local options=("$@") result="${options[0]}"
    case "$GUI_ENGINE" in
        zenity)
            local col_args=()
            for o in "${options[@]}"; do col_args+=("FALSE" "$o"); done
            result=$(zenity --list --radiolist --title="$title" --text="$label" \
                --column="Select" --column="Option" "${col_args[@]}" 2>/dev/null) || result="${options[0]}"
            ;;
        yad)
            local col_args=()
            for o in "${options[@]}"; do col_args+=("FALSE" "$o"); done
            result=$(yad --list --radiolist --title="$title" --text="$label" \
                --column="Select" --column="Option" "${col_args[@]}" 2>/dev/null | cut -d'|' -f2) || result="${options[0]}"
            ;;
        kdialog)
            local menu_args=()
            for i in "${!options[@]}"; do menu_args+=("$i" "${options[$i]}"); done
            local idx
            idx=$(kdialog --title "$title" --radiolist "$label" "${menu_args[@]}" 2>/dev/null) || idx=0
            result="${options[$idx]}"
            ;;
        osascript)
            local as_list
            as_list=$(printf '"%s", ' "${options[@]}" | sed 's/, $//')
            result=$(osascript -e "choose from list {${as_list}} with title \"${title}\" with prompt \"${label}\"" 2>/dev/null | tr -d '{}') || result="${options[0]}"
            ;;
        terminal|*)
            echo -e "\n  ${label}" >&2
            for i in "${!options[@]}"; do echo -e "    $((i+1))) ${options[$i]}" >&2; done
            echo -ne "  เลือก [1-${#options[@]}]: " >&2; read -r idx
            idx=$((${idx:-1} - 1))
            result="${options[$idx]:-${options[0]}}"
            ;;
    esac
    echo -n "$result"
}

# gui_confirm "Title" "Message" → 0=yes 1=no
gui_confirm() {
    local title="$1" msg="$2"
    case "$GUI_ENGINE" in
        zenity)  zenity --question --title="$title" --text="$msg" 2>/dev/null ;;
        yad)     yad --question --title="$title" --text="$msg" 2>/dev/null ;;
        kdialog) kdialog --title "$title" --yesno "$msg" 2>/dev/null ;;
        osascript)
            local btn
            btn=$(osascript -e "button returned of (display alert \"${title}\" message \"${msg}\" buttons {\"No\",\"Yes\"} default button \"Yes\")" 2>/dev/null)
            [[ "$btn" == "Yes" ]]
            ;;
        terminal|*)
            echo -ne "  ${CYAN}?${RESET}  ${msg} [Y/n]: " >&2; read -r val
            [[ "${val,,}" != "n" ]]
            ;;
    esac
}

# gui_info "Title" "Message"
gui_info() {
    local title="$1" msg="$2"
    case "$GUI_ENGINE" in
        zenity)  zenity --info --title="$title" --text="$msg" 2>/dev/null &;;
        yad)     yad --info --title="$title" --text="$msg" 2>/dev/null &;;
        kdialog) kdialog --title "$title" --msgbox "$msg" 2>/dev/null &;;
        osascript) osascript -e "display alert \"${title}\" message \"${msg}\"" 2>/dev/null &;;
        terminal|*) echo -e "\n  ${CYAN}ℹ${RESET}  ${title}: ${msg}\n" ;;
    esac
}

# gui_progress "Title" "Message" <command>
gui_progress() {
    local title="$1" msg="$2"; shift 2
    case "$GUI_ENGINE" in
        zenity)
            "$@" 2>&1 | zenity --progress --title="$title" --text="$msg" --pulsate --auto-close 2>/dev/null || "$@"
            ;;
        osascript)
            echo -e "  ⏳ ${title}: ${msg}"; "$@"
            ;;
        terminal|*) echo -e "  ⏳ ${title}…"; "$@" ;;
    esac
}

# ══════════════════════════════════════════════════════════════════════════════
# INSTALL DEPENDENCIES
# ══════════════════════════════════════════════════════════════════════════════
install_deps() {
    banner "Installing System Dependencies"

    case "$PLATFORM" in
        linux)
            # Install Go if missing
            if ! command -v go &>/dev/null; then
                local GO_VERSION="1.22.4"
                info "Installing Go ${GO_VERSION}..."
                local TMP_GO; TMP_GO=$(mktemp -d)
                curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o "${TMP_GO}/go.tar.gz"
                sudo tar -C /usr/local -xzf "${TMP_GO}/go.tar.gz"
                export PATH="/usr/local/go/bin:$PATH"
                echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.bashrc
            fi
            # Install Zenity for GUI if missing and on desktop
            if [[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]] && ! command -v zenity &>/dev/null && ! command -v yad &>/dev/null; then
                warn "Installing zenity for GUI dialogs..."
                if command -v apt-get &>/dev/null; then sudo apt-get install -y -q zenity
                elif command -v dnf &>/dev/null; then sudo dnf install -y zenity
                elif command -v pacman &>/dev/null; then sudo pacman -S --noconfirm zenity
                fi
            fi
            ;;
        darwin)
            # Install Homebrew + Go on macOS
            if ! command -v brew &>/dev/null; then
                info "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            if ! command -v go &>/dev/null; then
                info "Installing Go via Homebrew..."
                brew install go
            fi
            ;;
        windows)
            if ! command -v go &>/dev/null; then
                warn "Go not found. Please install from: https://go.dev/dl/"
                warn "Then re-run this installer."
                if command -v powershell.exe &>/dev/null; then
                    powershell.exe -Command "Start-Process 'https://go.dev/dl/'"
                fi
                exit 1
            fi
            ;;
    esac
    info "Go $(go version | awk '{print $3}') ready"
}

# ══════════════════════════════════════════════════════════════════════════════
# BUILD BINARIES
# ══════════════════════════════════════════════════════════════════════════════
build_binaries() {
    banner "Building OttoClaw Binaries"

    # Determine install prefix
    case "$PLATFORM" in
        linux)  INSTALL_BIN="/usr/local/bin" ;;
        darwin) INSTALL_BIN="/usr/local/bin" ;;
        windows)
            INSTALL_BIN="${HOME}/.local/bin"
            mkdir -p "$INSTALL_BIN"
            ;;
    esac

    local BRAIN_NAME="ottoclaw-brain"
    local WORKER_NAME="siam-worker"
    [[ "$PLATFORM" == "windows" ]] && BRAIN_NAME="ottoclaw-brain.exe" && WORKER_NAME="siam-worker.exe"

    gui_progress "Building" "กำลัง build ottoclaw-brain..." bash -c "
        cd '${SCRIPT_DIR}/ottoclaw'
        # Ensure workspace is available for embedding
        local ONBOARD_DIR='cmd/ottoclaw/internal/onboard'
        mkdir -p \"\${ONBOARD_DIR}\"
        rm -rf \"\${ONBOARD_DIR}/workspace\"
        mkdir -p \"\${ONBOARD_DIR}/workspace\"
        mkdir -p \"\${SCRIPT_DIR}/workspace\"
        touch \"\${SCRIPT_DIR}/workspace/placeholder.txt\"
        cp -rf \"\${SCRIPT_DIR}/workspace\" \"\${ONBOARD_DIR}/workspace\"
        CGO_ENABLED=0 go build -buildvcs=false -ldflags='-s -w' -o '${INSTALL_BIN}/${BRAIN_NAME}' ./cmd/ottoclaw
    "
    info "ottoclaw-brain → ${INSTALL_BIN}/${BRAIN_NAME}"

    gui_progress "Building" "กำลัง build siam-worker..." bash -c "
        cd '${REPO_ROOT}/siam-arm'
        CGO_ENABLED=0 go build -buildvcs=false -ldflags='-s -w' -o '${INSTALL_BIN}/${WORKER_NAME}' .
    "
    info "siam-worker → ${INSTALL_BIN}/${WORKER_NAME}"
}

# ══════════════════════════════════════════════════════════════════════════════
# GUI CONFIG WIZARD
# ══════════════════════════════════════════════════════════════════════════════

# Defaults
MASTER_HOST="${MASTER_HOST:-192.168.1.100}"
MASTER_API_KEY="${MASTER_API_KEY:-73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90}"
NODE_SECRET="${NODE_SECRET:-ea710cf8c0f08298e9aa938dff0e0133}"
AGENT_NAME="${AGENT_NAME:-Kaidos}"
ORCHESTRATOR_NICKNAMES="${ORCHESTRATOR_NICKNAMES:-}"
WORKER_TELEGRAM_TOKEN="${WORKER_TELEGRAM_TOKEN:-}"
TELEGRAM_ALLOW_FROM="${TELEGRAM_ALLOW_FROM:-}"
TELEGRAM_BRIDGE_CHAT_ID="${TELEGRAM_BRIDGE_CHAT_ID:-}"
TELEGRAM_ORCHESTRATION_ENABLED="${TELEGRAM_ORCHESTRATION_ENABLED:-false}"

run_config_wizard() {
    local is_reconfigure="${1:-false}"

    if [[ "$is_reconfigure" == "true" ]]; then
        banner "Reconfiguration"
        set -o allexport; source "${OTTOCLAW_ENV_FILE}" 2>/dev/null || true; set +o allexport
        if [[ -n "${MASTER_URL:-}" && -z "${MASTER_HOST:-}" ]]; then
            MASTER_HOST=$(echo "${MASTER_URL}" | sed 's|http://||;s|:8080||')
        fi
    fi

    # --- Page 1: Agent Identity ---
    AGENT_NAME=$(gui_input "🦞 OttoClaw Setup [1/3]" "AGENT_NAME:" "${AGENT_NAME}")
    [[ -z "$AGENT_NAME" ]] && AGENT_NAME="Kaidos"

    ORCHESTRATOR_NICKNAMES=$(gui_input "🦞 OttoClaw Setup [1/3]" "ORCHESTRATOR_NICKNAMES:" "${ORCHESTRATOR_NICKNAMES:-$AGENT_NAME}")

    # --- Page 2: Network ---
    local NET_CHOICE
    NET_CHOICE=$(gui_select "🦞 OttoClaw Setup [2/3] — Network" "เลือกประเภทการเชื่อมต่อกับ Master Server:" \
        "🏠 Local LAN (192.168.x.x)" \
        "🛡️ Tailscale VPN (100.x.x.x)" \
        "🌐 VPS / Public IP / Domain")

    PROTOCOL="http"
    case "$NET_CHOICE" in
        *"Tailscale"*) NET_LABEL="Tailscale VPN"; DEFAULT_HOST="${MASTER_HOST:-100.x.x.x}" ;;
        *"VPS"*)
            NET_LABEL="VPS / Public"
            DEFAULT_HOST="${MASTER_HOST:-1.2.3.4}"
            if gui_confirm "HTTPS" "ใช้ HTTPS? (ถ้า Master มี SSL certificate)"; then
                PROTOCOL="https"
            fi
            ;;
        *) NET_LABEL="Local LAN"; DEFAULT_HOST="${MASTER_HOST:-192.168.1.100}" ;;
    esac

    MASTER_HOST=$(gui_input "🦞 Master Server [2/3]" "MASTER_HOST (IP หรือ Domain):\n\nPorts → HTTP :8080  |  gRPC :50051" "${DEFAULT_HOST}")
    
    # Only prompt if not auto-injected from .env
    if [[ -z "${MASTER_API_KEY:-}" ]]; then
        MASTER_API_KEY=$(gui_password "🦞 Master Server [2/3]" "MASTER_API_KEY:")
        [[ -z "$MASTER_API_KEY" ]] && MASTER_API_KEY="73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90"
    fi
    if [[ -z "${NODE_SECRET:-}" ]]; then
        NODE_SECRET=$(gui_password "🦞 Master Server [2/3]" "NODE_SECRET:")
        [[ -z "$NODE_SECRET" ]] && NODE_SECRET="ea710cf8c0f08298e9aa938dff0e0133"
    fi

    MASTER_URL="${PROTOCOL}://${MASTER_HOST}:8080"
    MASTER_GRPC_URL="${MASTER_HOST}:50051"
    MASTER_API_URL="${MASTER_URL}"
    SIAM_MASTER_URL="${MASTER_URL}"
    SIAM_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_API_BASE="${MASTER_URL}/api/agent/v1/llm/proxy"
    OTTOCLAW_API_KEY="${MASTER_API_KEY}"
    OTTOCLAW_MODEL_ID="default"
    OTTOCLAW_MODEL_NAME="default"

    # --- Page 3: Telegram (Optional) ---
    if gui_confirm "🦞 Telegram [3/3]" "ต้องการเชื่อม Telegram Bot หรือไม่? (Optional)\n\nถ้าไม่ต้องการให้กด 'No'"; then
        WORKER_TELEGRAM_TOKEN=$(gui_password "🦞 Telegram Bot [3/3]" "Telegram Bot Token:" )
        if [[ -n "${WORKER_TELEGRAM_TOKEN:-}" ]]; then
            TELEGRAM_ALLOW_FROM=$(gui_input "🦞 Telegram [3/3]" "Allowed User IDs (Telegram, คั่นด้วย ,):" "${TELEGRAM_ALLOW_FROM:-}")
            if gui_confirm "🦞 Telegram Bridge [3/3]" "เปิด Agent-to-Agent Orchestration ผ่าน Telegram Group หรือไม่?"; then
                TELEGRAM_ORCHESTRATION_ENABLED="true"
                TELEGRAM_BRIDGE_CHAT_ID=$(gui_input "🦞 Telegram Bridge [3/3]" "Telegram Bridge Group ID (e.g. -100123456):" "${TELEGRAM_BRIDGE_CHAT_ID:-}")
            fi
        fi
    fi

    # Fixed
    NODE_ID="$(hostname)"
    OTTOCLAW_MODE="worker"
}

# ══════════════════════════════════════════════════════════════════════════════
# FILE WRITING
# ══════════════════════════════════════════════════════════════════════════════
setup_paths() {
    case "$PLATFORM" in
        linux)
            OTTOCLAW_HOME="/var/lib/ottoclaw"
            OTTOCLAW_ENV_FILE="/etc/ottoclaw/env"
            OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"
            OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"
            ;;
        darwin)
            OTTOCLAW_HOME="${HOME}/.ottoclaw"
            OTTOCLAW_ENV_FILE="${OTTOCLAW_HOME}/env"
            OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"
            OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"
            ;;
        windows)
            OTTOCLAW_HOME="${HOME}/.ottoclaw"
            OTTOCLAW_ENV_FILE="${OTTOCLAW_HOME}/env"
            OTTOCLAW_WORKSPACE="${OTTOCLAW_HOME}/workspace"
            OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"
            ;;
    esac
    mkdir -p "${OTTOCLAW_HOME}" "${OTTOCLAW_WORKSPACE}/v2"
    [[ "$PLATFORM" == "linux" ]] && sudo mkdir -p "$(dirname "$OTTOCLAW_ENV_FILE")" 2>/dev/null || mkdir -p "$(dirname "$OTTOCLAW_ENV_FILE")"
}

write_env_file() {
    local env_content
    env_content="# OttoClaw Worker — Environment (${AGENT_NAME})
# Generated: $(date)
# Platform: ${PLATFORM}
NODE_ID=${NODE_ID}
AGENT_NAME=${AGENT_NAME}
ORCHESTRATOR_NICKNAMES=${ORCHESTRATOR_NICKNAMES}
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
OTTOCLAW_HOME=${OTTOCLAW_HOME}
OTTOCLAW_WORKSPACE=${OTTOCLAW_WORKSPACE}/v2
OTTOCLAW_CONFIG=${OTTOCLAW_CONFIG}
OTTOCLAW_BIN=${INSTALL_BIN}/ottoclaw-brain"

    if [[ "$PLATFORM" == "linux" ]]; then
        echo "$env_content" | sudo tee "${OTTOCLAW_ENV_FILE}" > /dev/null
        sudo chmod 600 "${OTTOCLAW_ENV_FILE}"
    else
        echo "$env_content" > "${OTTOCLAW_ENV_FILE}"
        chmod 600 "${OTTOCLAW_ENV_FILE}"
    fi
    info "Environment → ${OTTOCLAW_ENV_FILE}"
}

generate_config_json() {
    local TG_FRAG=""
    if [[ -n "${WORKER_TELEGRAM_TOKEN:-}" ]]; then
        TG_FRAG=", \"telegram\": { \"enabled\": true, \"token\": \"${WORKER_TELEGRAM_TOKEN}\" }"
    fi
    cat > "${OTTOCLAW_CONFIG}" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "${OTTOCLAW_WORKSPACE}/v2",
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
    info "Config → ${OTTOCLAW_CONFIG}"
}

forge_soul() {
    local SOUL_PATH="${OTTOCLAW_WORKSPACE}/v2/SOUL.md"
    cat > "$SOUL_PATH" <<SOULEOF
# AI Soul Persona

- **Name:** ${AGENT_NAME}
- **Platform:** ${PLATFORM}
- **Personality:** Adaptable, precise — built for multi-agent orchestration.

## Core Directives
You are **${AGENT_NAME}**, operating within the Siam-Synapse network.
Always identify yourself as **${AGENT_NAME}**.

*Forged: $(date -u +%Y-%m-%dT%H:%M:%SZ) — GUI Installer*
SOULEOF
    info "Soul → ${SOUL_PATH}"
}

# ══════════════════════════════════════════════════════════════════════════════
# SERVICE INSTALLATION (Platform-specific)
# ══════════════════════════════════════════════════════════════════════════════
install_service() {
    banner "Installing Services"

    case "$PLATFORM" in
        linux)
            # systemd (requires sudo)
            sudo tee /etc/systemd/system/siam-worker.service > /dev/null <<EOF
[Unit]
Description=Siam-Synapse gRPC Arm (siam-worker)
After=network.target

[Service]
Type=simple
EnvironmentFile=${OTTOCLAW_ENV_FILE}
WorkingDirectory=${OTTOCLAW_HOME}
ExecStart=${INSTALL_BIN}/siam-worker
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF
            sudo tee /etc/systemd/system/ottoclaw-worker.service > /dev/null <<EOF
[Unit]
Description=Siam-Synapse OttoClaw Brain
After=network.target siam-worker.service
Requires=siam-worker.service

[Service]
Type=simple
EnvironmentFile=${OTTOCLAW_ENV_FILE}
WorkingDirectory=${OTTOCLAW_HOME}
ExecStart=${INSTALL_BIN}/ottoclaw-brain gateway --debug
Restart=on-failure
RestartSec=15s

[Install]
WantedBy=multi-user.target
EOF
            sudo systemctl daemon-reload
            sudo systemctl enable siam-worker ottoclaw-worker
            sudo systemctl restart siam-worker
            sleep 2
            sudo systemctl restart ottoclaw-worker
            info "systemd services started"
            ;;

        darwin)
            # launchd plist (macOS)
            local PLIST_DIR="${HOME}/Library/LaunchAgents"
            mkdir -p "$PLIST_DIR"

            cat > "${PLIST_DIR}/com.siam-synapse.siam-worker.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.siam-synapse.siam-worker</string>
  <key>ProgramArguments</key>
  <array><string>${INSTALL_BIN}/siam-worker</string></array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>AGENT_NAME</key><string>${AGENT_NAME}</string>
    <key>MASTER_URL</key><string>${MASTER_URL}</string>
    <key>MASTER_GRPC_URL</key><string>${MASTER_GRPC_URL}</string>
    <key>MASTER_API_KEY</key><string>${MASTER_API_KEY}</string>
    <key>OTTOCLAW_HOME</key><string>${OTTOCLAW_HOME}</string>
    <key>OTTOCLAW_WORKSPACE</key><string>${OTTOCLAW_WORKSPACE}/v2</string>
    <key>OTTOCLAW_BIN</key><string>${INSTALL_BIN}/ottoclaw-brain</string>
    <key>NODE_ID</key><string>${NODE_ID}</string>
    <key>OTTOCLAW_MODE</key><string>${OTTOCLAW_MODE}</string>
    <key>SIAM_MASTER_URL</key><string>${SIAM_MASTER_URL}</string>
    <key>SIAM_API_KEY</key><string>${SIAM_API_KEY}</string>
    <key>OTTOCLAW_API_BASE</key><string>${OTTOCLAW_API_BASE}</string>
    <key>OTTOCLAW_API_KEY</key><string>${OTTOCLAW_API_KEY}</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${OTTOCLAW_HOME}/logs/siam-worker.log</string>
  <key>StandardErrorPath</key><string>${OTTOCLAW_HOME}/logs/siam-worker-err.log</string>
</dict>
</plist>
EOF

            cat > "${PLIST_DIR}/com.siam-synapse.ottoclaw-brain.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.siam-synapse.ottoclaw-brain</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_BIN}/ottoclaw-brain</string>
    <string>gateway</string><string>--debug</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>AGENT_NAME</key><string>${AGENT_NAME}</string>
    <key>MASTER_URL</key><string>${MASTER_URL}</string>
    <key>MASTER_GRPC_URL</key><string>${MASTER_GRPC_URL}</string>
    <key>MASTER_API_KEY</key><string>${MASTER_API_KEY}</string>
    <key>OTTOCLAW_HOME</key><string>${OTTOCLAW_HOME}</string>
    <key>OTTOCLAW_WORKSPACE</key><string>${OTTOCLAW_WORKSPACE}/v2</string>
    <key>OTTOCLAW_CONFIG</key><string>${OTTOCLAW_CONFIG}</string>
    <key>OTTOCLAW_BIN</key><string>${INSTALL_BIN}/ottoclaw-brain</string>
    <key>NODE_ID</key><string>${NODE_ID}</string>
    <key>OTTOCLAW_MODE</key><string>${OTTOCLAW_MODE}</string>
    <key>SIAM_MASTER_URL</key><string>${SIAM_MASTER_URL}</string>
    <key>SIAM_API_KEY</key><string>${SIAM_API_KEY}</string>
    <key>OTTOCLAW_API_BASE</key><string>${OTTOCLAW_API_BASE}</string>
    <key>OTTOCLAW_API_KEY</key><string>${OTTOCLAW_API_KEY}</string>
    <key>OTTOCLAW_MODEL_ID</key><string>${OTTOCLAW_MODEL_ID}</string>
    <key>OTTOCLAW_MODEL_NAME</key><string>${OTTOCLAW_MODEL_NAME}</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${OTTOCLAW_HOME}/logs/ottoclaw-brain.log</string>
  <key>StandardErrorPath</key><string>${OTTOCLAW_HOME}/logs/ottoclaw-brain-err.log</string>
</dict>
</plist>
EOF
            mkdir -p "${OTTOCLAW_HOME}/logs"
            launchctl load "${PLIST_DIR}/com.siam-synapse.siam-worker.plist" 2>/dev/null || true
            launchctl load "${PLIST_DIR}/com.siam-synapse.ottoclaw-brain.plist" 2>/dev/null || true
            info "launchd agents loaded (auto-start on login)"
            ;;

        windows)
            # Windows: create a start script + Task Scheduler entry
            local START_SCRIPT="${OTTOCLAW_HOME}/start.bat"
            cat > "$START_SCRIPT" <<EOF
@echo off
setlocal
set "AGENT_NAME=${AGENT_NAME}"
set "MASTER_URL=${MASTER_URL}"
set "MASTER_GRPC_URL=${MASTER_GRPC_URL}"
set "MASTER_API_KEY=${MASTER_API_KEY}"
set "SIAM_MASTER_URL=${SIAM_MASTER_URL}"
set "SIAM_API_KEY=${SIAM_API_KEY}"
set "OTTOCLAW_API_BASE=${OTTOCLAW_API_BASE}"
set "OTTOCLAW_API_KEY=${OTTOCLAW_API_KEY}"
set "OTTOCLAW_HOME=${OTTOCLAW_HOME}"
set "OTTOCLAW_WORKSPACE=${OTTOCLAW_WORKSPACE}\\v2"
set "OTTOCLAW_CONFIG=${OTTOCLAW_CONFIG}"
set "OTTOCLAW_MODE=${OTTOCLAW_MODE}"
set "NODE_ID=${NODE_ID}"

echo Starting siam-worker...
start /B "${INSTALL_BIN}/siam-worker.exe" > "${OTTOCLAW_HOME}\\logs\\siam-worker.log" 2>&1
timeout /t 2
echo Starting ottoclaw-brain...
start /B "${INSTALL_BIN}/ottoclaw-brain.exe" gateway --debug > "${OTTOCLAW_HOME}\\logs\\ottoclaw-brain.log" 2>&1
echo OttoClaw started!
EOF
            local STOP_SCRIPT="${OTTOCLAW_HOME}/stop.bat"
            cat > "$STOP_SCRIPT" <<'EOF'
@echo off
taskkill /F /IM ottoclaw-brain.exe 2>nul
taskkill /F /F /IM siam-worker.exe 2>nul
echo OttoClaw stopped.
EOF
            mkdir -p "${OTTOCLAW_HOME}/logs"
            info "start.bat  → ${OTTOCLAW_HOME}/start.bat"
            info "stop.bat   → ${OTTOCLAW_HOME}/stop.bat"

            # Try to add to Task Scheduler via PowerShell
            if command -v powershell.exe &>/dev/null; then
                powershell.exe -Command "
                    \$action = New-ScheduledTaskAction -Execute '$(cygpath -w "${START_SCRIPT}" 2>/dev/null || echo "${START_SCRIPT}")'
                    \$trigger = New-ScheduledTaskTrigger -AtLogOn
                    \$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable
                    Register-ScheduledTask -TaskName 'OttoClaw Worker' -Action \$action -Trigger \$trigger -Settings \$settings -Force -RunLevel Highest 2>&1
                " 2>/dev/null && info "Task Scheduler: OttoClaw Worker registered (starts at login)" || \
                    warn "Task Scheduler registration failed — run start.bat manually"
            fi
            ;;
    esac
}

# ══════════════════════════════════════════════════════════════════════════════
# UPDATE: git pull + rebuild + restart
# ══════════════════════════════════════════════════════════════════════════════
do_update() {
    detect_gui_engine
    setup_paths

    # Detect INSTALL_BIN based on platform
    case "$PLATFORM" in
        linux)   INSTALL_BIN="/usr/local/bin" ;;
        darwin)  INSTALL_BIN="/usr/local/bin" ;;
        windows) INSTALL_BIN="${HOME}/.local/bin" ;;
    esac

    local BRAIN_NAME="ottoclaw-brain"
    local WORKER_NAME="siam-worker"
    [[ "$PLATFORM" == "windows" ]] && BRAIN_NAME="ottoclaw-brain.exe" && WORKER_NAME="siam-worker.exe"

    banner "🔄 OttoClaw Update"
    echo -e "  Repo: ${SCRIPT_DIR}"
    echo ""

    # 1. Pull
    echo "⏳ Pulling latest code..."
    if [[ -d "${REPO_ROOT}/.git" ]]; then
        git -C "${REPO_ROOT}" pull --ff-only || { err "git pull failed — resolve conflicts first"; }
    else
        warn "Not a git repository. Re-running installer..."
        bash "$0"
        exit 0
    fi

    # 2. Stop services
    echo "🛑 Stopping services..."
    case "$PLATFORM" in
        linux)
            sudo systemctl stop ottoclaw-worker siam-worker 2>/dev/null || true
            ;;
        darwin)
            launchctl unload "${HOME}/Library/LaunchAgents/com.siam-synapse.ottoclaw-brain.plist" 2>/dev/null || true
            launchctl unload "${HOME}/Library/LaunchAgents/com.siam-synapse.siam-worker.plist" 2>/dev/null || true
            ;;
        windows)
            powershell.exe -Command "Get-Process ottoclaw-brain,siam-worker -ErrorAction SilentlyContinue | Stop-Process -Force" 2>/dev/null || true
            ;;
    esac

    # 3. Rebuild
    echo "🔨 Rebuilding ottoclaw-brain..."
    pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
    # Ensure workspace is available for embedding
    local ONBOARD_DIR="cmd/ottoclaw/internal/onboard"
    mkdir -p "${ONBOARD_DIR}"
    rm -rf "${ONBOARD_DIR}/workspace"
    mkdir -p "${SCRIPT_DIR}/workspace"
    touch "${SCRIPT_DIR}/workspace/placeholder.txt"
    cp -rf "${SCRIPT_DIR}/workspace" "${ONBOARD_DIR}/workspace"
    CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o "${INSTALL_BIN}/${BRAIN_NAME}" ./cmd/ottoclaw
    popd >/dev/null
    info "ottoclaw-brain rebuilt"

    echo "🔨 Rebuilding siam-worker..."
    pushd "${REPO_ROOT}/siam-arm" >/dev/null
    CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o "${INSTALL_BIN}/${WORKER_NAME}" .
    popd >/dev/null
    info "siam-worker rebuilt"

    # 4. Restart services
    echo "🚀 Restarting services..."
    case "$PLATFORM" in
        linux)
            sudo systemctl restart siam-worker
            sleep 2
            sudo systemctl restart ottoclaw-worker
            info "systemd services restarted"
            ;;
        darwin)
            launchctl load "${HOME}/Library/LaunchAgents/com.siam-synapse.siam-worker.plist" 2>/dev/null || true
            sleep 1
            launchctl load "${HOME}/Library/LaunchAgents/com.siam-synapse.ottoclaw-brain.plist" 2>/dev/null || true
            info "launchd agents restarted"
            ;;
        windows)
            start "${HOME}/.ottoclaw/start.bat" 2>/dev/null || \
                warn "Run ${HOME}/.ottoclaw/start.bat manually to restart"
            ;;
    esac

    echo ""
    echo "✅ Update complete! Running latest code."
    gui_info "✅ OttoClaw Updated!" "Binaries rebuilt and services restarted."
}

# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════
main() {
    # Handle --update / -u flag
    if [[ "${1:-}" == "--update" || "${1:-}" == "-u" || "${1:-}" == "update" ]]; then
        do_update
        exit 0
    fi

    detect_gui_engine

    # ── Show welcome dialog ────────────────────────────────────────────────────
    gui_info "🦞 Siam-Synapse OttoClaw" "OttoClaw Worker Installer\n\nPlatform: ${PLATFORM} (${ARCH})\nGUI: ${GUI_ENGINE}\n\nกด OK เพื่อเริ่มการติดตั้ง"

    echo -e "\n${CYAN}${BOLD}"
    echo "  ╔═══════════════════════════════════════════════════╗"
    echo "  ║   🦞  Siam-Synapse OttoClaw  —  GUI Installer     ║"
    printf  "  ║   Platform: %-37s║\n" "${PLATFORM} (${ARCH}) • GUI: ${GUI_ENGINE}"
    echo "  ╚═══════════════════════════════════════════════════╝"
    echo -e "${RESET}"

    # 1. Setup paths
    setup_paths

    # 2. Install dependencies
    install_deps

    # 3. Build binaries
    build_binaries

    # 4. Config wizard (GUI dialogs)
    run_config_wizard "false"

    # 5. Write files
    banner "Writing Configuration"
    write_env_file
    generate_config_json
    forge_soul

    # Copy workspace data
    [[ -d "${SCRIPT_DIR}/workspace" ]] && cp -rf "${SCRIPT_DIR}/workspace/." "${OTTOCLAW_WORKSPACE}/" 2>/dev/null || true

    # 6. Install service
    install_service

    # 7. Summary dialog
    local SUMMARY="✅ OttoClaw Worker ติดตั้งเสร็จสมบูรณ์!

Agent: ${AGENT_NAME}
Master: ${MASTER_URL}
Platform: ${PLATFORM}

คำสั่ง:
• ottoclaw update    → อัปเดต code และ rebuild
• ottoclaw config    → ตั้งค่าใหม่
• ottoclaw uninstall → ลบออก"

    gui_info "✅ ติดตั้งเสร็จแล้ว!" "$SUMMARY"

    banner "✅ Installation Complete!"
    echo -e "  ${GREEN}✓${RESET}  Agent:  ${AGENT_NAME}"
    echo -e "  ${GREEN}✓${RESET}  Master: ${MASTER_URL}"
    echo -e "  ${GREEN}✓${RESET}  Binaries: ${INSTALL_BIN}"
    echo ""
    case "$PLATFORM" in
        linux)
            echo -e "${BOLD}📋 Commands:${RESET}"
            echo -e "  ${CYAN}sudo ottoclaw update${RESET}                       → อัปเดต & rebuild"
            echo -e "  ${CYAN}sudo systemctl status ottoclaw-worker${RESET}      → ดูสถานะ"
            echo -e "  ${CYAN}journalctl -u ottoclaw-worker -f${RESET}           → ดู log"
            echo -e "  ${CYAN}sudo systemctl restart ottoclaw-worker${RESET}     → รีสตาร์ท"
            ;;
        darwin)
            echo -e "${BOLD}📋 Commands:${RESET}"
            echo -e "  ${CYAN}bash install-gui.sh update${RESET}                 → อัปเดต & rebuild"
            echo -e "  ${CYAN}launchctl list | grep siam${RESET}                 → ดูสถานะ"
            echo -e "  ${CYAN}tail -f ~/.ottoclaw/logs/ottoclaw-brain.log${RESET} → ดู log"
            ;;
        windows)
            echo -e "${BOLD}📋 Commands:${RESET}"
            echo -e "  ${CYAN}bash install-gui.sh update${RESET}  → อัปเดต & rebuild"
            echo -e "  ${CYAN}${OTTOCLAW_HOME}/start.bat${RESET}  → เริ่มทำงาน"
            echo -e "  ${CYAN}${OTTOCLAW_HOME}/stop.bat${RESET}   → หยุดทำงาน"
            echo -e "  ${CYAN}${OTTOCLAW_HOME}/logs/${RESET}      → ดู logs"
            ;;
    esac
    echo ""
}

main "$@"
