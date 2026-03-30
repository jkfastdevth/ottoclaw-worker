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
    
    if [[ "$secret" == "true" && "${HIDE_SECRETS:-true}" == "true" ]]; then
        # Redirect prompt to stderr so it's not captured by $(prompt_val ...)
        echo -ne "  ${CYAN}?${RESET}  ${label} [${display_default}]: " >&2
        read -s value < /dev/tty; echo "" >&2
    else
        echo -ne "  ${CYAN}?${RESET}  ${label} [${display_default}]: " >&2
        read -r value < /dev/tty
    fi
    
    local result="${value:-$default}"
    # Strip any accidental newlines, spaces, or carriage returns
    echo -n "$result" | tr -dc '[:print:]' | xargs echo -n
}

get_tailscale_ip() {
    # Detect IP starting with 100. (Tailscale default range)
    local ts_ip
    ts_ip=$(ip addr show 2>/dev/null | grep -oE "\b100\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    if [[ -z "$ts_ip" ]]; then
        ts_ip=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep "^100\." | head -n 1)
    fi
    echo -n "$ts_ip"
}

get_local_ip() {
    # Detect IP in common LAN ranges: 192.168.x.x or 10.x.x.x
    local local_ip
    local_ip=$(ip addr show 2>/dev/null | grep -oE "\b(192\.168|10)\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" | head -n 1)
    if [[ -z "$local_ip" ]]; then
        local_ip=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E "^(192\.168|10)\." | head -n 1)
    fi
    echo -n "$local_ip"
}

# ── Locate Source ─────────────────────────────────────────────────────────────
# Handle curl | bash (unbound BASH_SOURCE) vs direct execution
SCRIPT_DIR="$(pwd)"
if [[ -n "${BASH_SOURCE+x}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd 2>/dev/null || echo "$SCRIPT_DIR")"
elif [[ -n "$0" && -f "$0" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd 2>/dev/null || echo "$SCRIPT_DIR")"
fi
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd 2>/dev/null || echo "$SCRIPT_DIR")"

# ── Detect Termux ─────────────────────────────────────────────────────────────
if [[ -d "/data/data/com.termux" ]] || [[ -n "${TERMUX_VERSION:-}" ]]; then
    banner "Termux Detected"
    echo "This script (install.sh) is for standard Linux with root access."
    echo "Redirecting to specialized Termux installer..."
    echo ""
    if [[ -f "${SCRIPT_DIR}/install-termux.sh" ]]; then
        exec bash "${SCRIPT_DIR}/install-termux.sh" "$@"
    else
        error "install-termux.sh not found. Please use the Termux-specific installer."
    fi
fi

# ── Root Check ────────────────────────────────────────────────────────────────
if [[ "$EUID" -ne 0 ]]; then
    error "Please run as root (or you might be on Termux, use install-termux.sh)"
fi

# ── Detect Architecture ───────────────────────────────────────────────────────
ARCH=$(uname -m)
SUFFIX=""
case "$ARCH" in
    x86_64)  GO_ARCH="amd64"; SUFFIX="linux-amd64" ;;
    aarch64) GO_ARCH="arm64"; SUFFIX="linux-arm64" ;;
    armv7l)  GO_ARCH="arm"; warn "No pre-built binary for armv7l — will compile from source." ;;
    *)       error "Unsupported architecture: $ARCH" ;;
esac
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# ── Release Configuration ─────────────────────────────────────────────────────
VERSION="latest"
REPO="jkfastdevth/ottoclaw-worker"
BINARY_URL="https://github.com/${REPO}/releases/latest/download/ottoclaw-worker-${SUFFIX}.tar.gz"

# ── Auto-load Credentials from .env ───────────────────────────────
if [[ -f "${REPO_ROOT}/.env" ]]; then
    # Try to extract keys if not already set
    [[ -z "${MASTER_API_KEY:-}" ]] && MASTER_API_KEY=$(grep "^MASTER_API_KEY=" "${REPO_ROOT}/.env" | cut -d'=' -f2- | tr -d '\r')
    [[ -z "${NODE_SECRET:-}"   ]] && NODE_SECRET=$(grep "^NODE_SECRET=" "${REPO_ROOT}/.env" | cut -d'=' -f2- | tr -d '\r')
    
    if [[ -n "${MASTER_API_KEY:-}" || -n "${NODE_SECRET:-}" ]]; then
        info "Auto-injected security keys from ${REPO_ROOT}/.env"
    fi
fi

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
        ORCHESTRATOR_NICKNAMES="${ORCHESTRATOR_NICKNAMES:-}"
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
    fi

    # Extract MASTER_HOST from existing MASTER_URL if reconfiguring
    if [[ -n "${MASTER_URL:-}" && -z "${MASTER_HOST:-}" ]]; then
        MASTER_HOST=$(echo "${MASTER_URL}" | sed 's|http://||;s|:8080||')
    fi
    MASTER_HOST="${MASTER_HOST:-192.168.1.1}"

    # ── [1/3] Master Server ───────────────────────────────────────────────────
    echo -e "${BOLD}[1/3] System Configuration${RESET}"
    AGENT_NAME=$(prompt_val "AGENT_NAME" "${AGENT_NAME:-Kaidos}")
    ORCHESTRATOR_NICKNAMES=$(prompt_val "ORCHESTRATOR_NICKNAMES" "${ORCHESTRATOR_NICKNAMES:-${AGENT_NAME}}")
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

    MASTER_HOST=$(    prompt_val "MASTER_HOST" "${DEFAULT_HOST}")
    
    # Only prompt if not auto-injected from .env
    if [[ -z "${MASTER_API_KEY:-}" ]]; then
        MASTER_API_KEY=$( prompt_val "MASTER_API_KEY" "${MASTER_API_KEY:-73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90}" "true")
    fi
    if [[ -z "${NODE_SECRET:-}" ]]; then
        NODE_SECRET=$(    prompt_val "NODE_SECRET"    "${NODE_SECRET:-ea710cf8c0f08298e9aa938dff0e0133}"    "true")
    fi

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
    ORCHESTRATOR_TELEGRAM_TOKEN=$( prompt_val "Telegram Bot Token"                    "${ORCHESTRATOR_TELEGRAM_TOKEN:-}" "true")
    TELEGRAM_ALLOW_FROM=""
    TELEGRAM_BRIDGE_CHAT_ID=""
    TELEGRAM_ORCHESTRATION_ENABLED="false"
    ORCHESTRATOR_NICKNAMES=""
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

    # ── [3/4] Google Skill Access (Optional) ──────────────────────────────────
    echo -e "${BOLD}[3/4] Google Skill Access (Optional — leave blank for General Worker)${RESET}"
    GOOGLE_EMAIL=$(prompt_val "GOOGLE_EMAIL" "${GOOGLE_EMAIL:-}")
    GOOGLE_APP_PASSWORD=$(prompt_val "GOOGLE_APP_PASSWORD" "${GOOGLE_APP_PASSWORD:-}" "true")

    echo ""

    # ── [4/4] Service User (Optional) ─────────────────────────────────────────
    echo -e "${BOLD}[4/4] Service Options (Optional)${RESET}"
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
ORCHESTRATOR_NICKNAMES="${ORCHESTRATOR_NICKNAMES}"

# ── Google Skill Access (Optional) ──────────────────────────
GOOGLE_EMAIL="${GOOGLE_EMAIL}"
GOOGLE_APP_PASSWORD="${GOOGLE_APP_PASSWORD}"

# ── Paths ──────────────────────────────────────────────────────
OTTOCLAW_HOME="${OTTOCLAW_HOME}"
OTTOCLAW_WORKSPACE="${OTTOCLAW_WORKSPACE}/v2"
OTTOCLAW_CONFIG="${OTTOCLAW_HOME}/config.json"

# ── Native Binary Path (for siam-worker to find brain) ───────
OTTOCLAW_BIN=/usr/local/bin/ottoclaw-brain
EOF
    chmod 600 /etc/ottoclaw/env
    info "Environment saved → /etc/ottoclaw/env (mode 600)"
    
    # 🔖 Write Version File
    if [[ -d "${REPO_ROOT}/.git" ]]; then
        git -C "${REPO_ROOT}" describe --tags --always > /etc/ottoclaw/version 2>/dev/null || true
    fi
}

# ══════════════════════════════════════════════════════════════════════════════
# BUILD
# ══════════════════════════════════════════════════════════════════════════════
build_binaries() {
    banner "Installing OttoClaw Binaries"
    
    local use_binary=false
    if [[ -n "${SUFFIX:-}" ]]; then
        echo -e "  Attempting to download pre-compiled binary for ${SUFFIX}..."
        if curl -fsSL --head "${BINARY_URL}" >/dev/null 2>&1; then
            local tmp_bin=$(mktemp -d)
            if curl -fsSL "${BINARY_URL}" -o "${tmp_bin}/release.tar.gz"; then
                echo -e "  Extracting binaries..."
                tar -xzf "${tmp_bin}/release.tar.gz" -C "${tmp_bin}"
                [[ -f "${tmp_bin}/ottoclaw-brain" ]] && cp "${tmp_bin}/ottoclaw-brain" /usr/local/bin/ottoclaw-brain
                [[ -f "${tmp_bin}/siam-worker" ]] && cp "${tmp_bin}/siam-worker" /usr/local/bin/siam-worker
                chmod +x /usr/local/bin/ottoclaw-brain /usr/local/bin/siam-worker
                
                # Update SCRIPT_DIR to point to where workspace/skills are
                SCRIPT_DIR="${tmp_bin}"
                use_binary=true
                info "Installed pre-compiled binaries."
            fi
        else
            warn "No pre-compiled binary found for ${SUFFIX} at ${VERSION}."
            echo -e "     ${YELLOW}💡 Tip:${RESET} To avoid building on low-spec remote machines:"
            echo -e "        1. Run ${CYAN}./build-releases.sh v1.0.0${RESET} on your local machine."
            echo -e "        2. ${CYAN}git tag v1.0.0${RESET} and ${CYAN}git push --tags${RESET} to GitHub."
            echo -e "     The installer will then find the binaries automatically.\n"
        fi
    fi

    if [[ "$use_binary" == "false" ]]; then
        # Check if source exists, if not clone it (for one-liner source build)
        if [[ ! -d "${SCRIPT_DIR}/ottoclaw" ]]; then
            warn "Source code not found in ${SCRIPT_DIR}. Downloading from GitHub..."
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

        # Check for Go
        if ! command -v go >/dev/null 2>&1; then
            warn "Go not found. Installing Go 1.21..."
            local TMP_GO=$(mktemp -d)
            local GO_VERSION="1.21.8"
            curl -fsSL "https://go.dev/dl/go${GO_VERSION}.${OS}-${GO_ARCH}.tar.gz" -o "${TMP_GO}/go.tar.gz"
            tar -C /usr/local -xzf "${TMP_GO}/go.tar.gz"
            export PATH="/usr/local/go/bin:$PATH"
            info "Go ${GO_VERSION} installed."
        fi

        echo -e "  Building ${BOLD}ottoclaw${RESET} (Brain)..."
        pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
        # Ensure workspace is available for embedding
        local ONBOARD_DIR="cmd/ottoclaw/internal/onboard"
        mkdir -p "${ONBOARD_DIR}"
        rm -rf "${ONBOARD_DIR}/workspace"
        # Create empty workspace if missing to avoid build failure
        mkdir -p "${SCRIPT_DIR}/workspace"
        touch "${SCRIPT_DIR}/workspace/placeholder.txt"
        cp -rf "${SCRIPT_DIR}/workspace" "${ONBOARD_DIR}/workspace"
        CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o /usr/local/bin/ottoclaw-brain ./cmd/ottoclaw
        popd >/dev/null
        info "ottoclaw-brain → /usr/local/bin/ottoclaw-brain"

        echo -e "  Building ${BOLD}siam-worker${RESET} (Arm)..."
        pushd "${SCRIPT_DIR}/siam-arm" >/dev/null
        CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o /usr/local/bin/siam-worker .
        popd >/dev/null
        info "siam-worker → /usr/local/bin/siam-worker"
    fi
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

# ── Colors & Helpers (Copied from installer) ───────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
info()   { echo -e "  ${GREEN}✓${RESET}  $1"; }
warn()   { echo -e "  ${YELLOW}⚠${RESET}  $1"; }
error()  { echo -e "  ${RED}✗${RESET}  $1"; exit 1; }

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

  update)
    # ── Pull latest code and rebuild binaries ─────────────────
    if [[ "$EUID" -ne 0 ]]; then
        exec sudo bash "$0" update
    fi
    INSTALL_SH="$(find /opt/siam-synapse /home -name install.sh -path '*/ottoclaw-worker/*' 2>/dev/null | head -1)"
    if [[ -z "$INSTALL_SH" ]]; then
        echo "❌ Cannot locate install.sh"; exit 1
    fi
    REPO_DIR="$(dirname "$INSTALL_SH")"
    echo ""
    echo "🔄 OttoClaw Update"
    echo "   Repo: $REPO_DIR"
    echo ""
    echo "⏳ Pulling latest code..."
    if [[ -d "${REPO_DIR}/.git" ]]; then
        # Ensure git doesn't complain about ownership when running as root
        git config --global --add safe.directory "$REPO_DIR" 2>/dev/null || true
        git -C "$REPO_DIR" pull --ff-only && git -C "$REPO_DIR" fetch --tags || { echo "❌ git pull failed — resolve conflicts manually"; exit 1; }
        git -C "$REPO_DIR" describe --tags --always > /etc/ottoclaw/version 2>/dev/null || true
    else
        warn "Not a git repository — downloading latest install.sh from GitHub..."
        local FRESH_INSTALL=$(mktemp /tmp/ottoclaw-install-XXXXXX.sh)
        if curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/install.sh" -o "$FRESH_INSTALL" 2>/dev/null; then
            chmod +x "$FRESH_INSTALL"
            cp "$FRESH_INSTALL" "$INSTALL_SH" 2>/dev/null || true
            info "install.sh updated from GitHub"
            exec sudo bash "$FRESH_INSTALL"
        else
            warn "Download failed — re-running existing installer"
            exec sudo bash "$INSTALL_SH"
        fi
        exit 0
    fi
    echo "🛑 Stopping services..."
    systemctl stop ottoclaw-worker siam-worker 2>/dev/null || true
    echo "🔨 Rebuilding ottoclaw-brain..."
    pushd "$(dirname "$INSTALL_SH")/ottoclaw" >/dev/null
    # Fix: exit 128 "error obtaining VCS status" by using -buildvcs=false
    CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o /usr/local/bin/ottoclaw-brain ./cmd/ottoclaw
    popd >/dev/null
    echo "   ✓ ottoclaw-brain rebuilt"
    echo "🔨 Rebuilding siam-worker..."
    pushd "$(dirname "$INSTALL_SH")/siam-arm" >/dev/null
    CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o /usr/local/bin/siam-worker .
    popd >/dev/null
    echo "   ✓ siam-worker rebuilt"
    echo "🚀 Restarting services..."
    systemctl restart siam-worker
    sleep 2
    systemctl restart ottoclaw-worker
    echo ""
    echo "✅ Update complete! Services are running with the latest code."
    echo "   journalctl -u ottoclaw-worker -f   → view logs"
    ;;

  help|--help|-h)
    echo ""
    echo "  OttoClaw Worker — Management CLI"
    echo ""
    echo "  ottoclaw config      Reconfigure Master URL, API key, Telegram, etc."
    echo "  ottoclaw update      Pull latest code & rebuild binaries (requires sudo)"
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
TG_TOKEN="${ORCHESTRATOR_TELEGRAM_TOKEN:-}"
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
      "max_tool_iterations": 50
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

# Create empty local dirs if missing to avoid copy errors
mkdir -p "${SCRIPT_DIR}/workspace" "${SCRIPT_DIR}/skills"
touch "${SCRIPT_DIR}/workspace/placeholder.txt"

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

# 5b. Install TTS + STT dependencies (edge-tts + faster-whisper)
banner "Installing TTS/STT Dependencies"
if command -v pip3 &>/dev/null || command -v pip &>/dev/null; then
    PIP_CMD="$(command -v pip3 2>/dev/null || command -v pip)"
    if "$PIP_CMD" install --quiet --break-system-packages edge-tts 2>/dev/null || \
       "$PIP_CMD" install --quiet edge-tts 2>/dev/null; then
        info "edge-tts installed (th-TH-PremwadeeNeural voice)"
    else
        warn "edge-tts install failed — will fallback to espeak-ng"
    fi
    if "$PIP_CMD" install --quiet --break-system-packages faster-whisper 2>/dev/null || \
       "$PIP_CMD" install --quiet faster-whisper 2>/dev/null; then
        info "faster-whisper installed (STT primary)"
    else
        warn "faster-whisper install failed — trying openai-whisper as fallback"
    fi
    info "STT fallback (openai-whisper) can be installed manually: pip3 install openai-whisper"
    # Phase 5.2: Vosk for wake word detection
    if "$PIP_CMD" install --quiet --break-system-packages vosk 2>/dev/null || \
       "$PIP_CMD" install --quiet vosk 2>/dev/null; then
        info "vosk installed (wake word detection)"
    else
        warn "vosk install failed — wake word detection will be unavailable"
    fi
elif command -v apt-get &>/dev/null; then
    apt-get install -y -q python3-pip 2>/dev/null
    pip3 install --quiet edge-tts 2>/dev/null && info "edge-tts installed" || warn "edge-tts install failed"
    pip3 install --quiet faster-whisper 2>/dev/null && info "faster-whisper installed" || warn "faster-whisper install failed"
    pip3 install --quiet resemblyzer 2>/dev/null && info "resemblyzer installed (Speaker ID)" || warn "resemblyzer install failed"
    pip3 install --quiet vosk 2>/dev/null && info "vosk installed (wake word)" || warn "vosk install failed"
else
    warn "pip not found — skipping TTS/STT install (espeak-ng fallback will be used)"
fi
# Install audio recording tools — parec (PulseAudio/PipeWire) preferred on desktop Linux
if ! command -v parec &>/dev/null && ! command -v parecord &>/dev/null; then
    if command -v apt-get &>/dev/null; then
        apt-get install -y -q pulseaudio-utils 2>/dev/null && info "parec (pulseaudio-utils) installed" || warn "pulseaudio-utils install failed"
    fi
fi
# Also install arecord (ALSA) as fallback for headless/server systems
if ! command -v arecord &>/dev/null; then
    if command -v apt-get &>/dev/null; then
        apt-get install -y -q alsa-utils 2>/dev/null && info "arecord installed" || warn "arecord install failed"
    fi
fi
# Install ffmpeg if missing (required for audio conversion)
if ! command -v ffmpeg &>/dev/null; then
    if command -v apt-get &>/dev/null; then
        apt-get install -y -q ffmpeg 2>/dev/null && info "ffmpeg installed" || warn "ffmpeg install failed"
    fi
fi

# Phase 5.1: Install Piper TTS (high-quality local neural TTS)
banner "Installing Piper TTS"
PIPER_DIR="${HOME}/.picoclaw/piper"
PIPER_BIN="${PIPER_DIR}/piper"
if [[ -f "${PIPER_BIN}" ]]; then
    info "Piper TTS already installed: ${PIPER_BIN}"
else
    ARCH="$(uname -m)"
    case "${ARCH}" in
        x86_64)   PIPER_ARCHIVE="piper_linux_x86_64.tar.gz" ;;
        aarch64)  PIPER_ARCHIVE="piper_linux_aarch64.tar.gz" ;;
        armv7l)   PIPER_ARCHIVE="piper_linux_armv7l.tar.gz" ;;
        *)        warn "Piper TTS: unsupported arch ${ARCH} — skipping"; PIPER_ARCHIVE="" ;;
    esac
    if [[ -n "${PIPER_ARCHIVE}" ]]; then
        mkdir -p "${PIPER_DIR}/models"
        PIPER_URL="https://github.com/rhasspy/piper/releases/latest/download/${PIPER_ARCHIVE}"
        TMP_PIPER="/tmp/${PIPER_ARCHIVE}"
        if curl -fsSL "${PIPER_URL}" -o "${TMP_PIPER}" 2>/dev/null; then
            tar -xzf "${TMP_PIPER}" -C "${PIPER_DIR}" 2>/dev/null
            # Binary may extract as piper/piper subdirectory
            [[ -f "${PIPER_DIR}/piper/piper" ]] && mv "${PIPER_DIR}/piper/piper" "${PIPER_BIN}" && rm -rf "${PIPER_DIR}/piper"
            chmod +x "${PIPER_BIN}" 2>/dev/null
            rm -f "${TMP_PIPER}"
            if [[ -f "${PIPER_BIN}" ]]; then
                info "Piper TTS installed: ${PIPER_BIN}"
                # Download EN model (smaller, faster to download)
                EN_ONNX="${PIPER_DIR}/models/en_US-lessac-medium.onnx"
                EN_JSON="${EN_ONNX}.json"
                if [[ ! -f "${EN_ONNX}" ]]; then
                    HF_BASE="https://huggingface.co/rhasspy/piper-voices/resolve/main"
                    curl -fsSL "${HF_BASE}/en/en_US/lessac/medium/en_US-lessac-medium.onnx" -o "${EN_ONNX}" 2>/dev/null \
                        && curl -fsSL "${HF_BASE}/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json" -o "${EN_JSON}" 2>/dev/null \
                        && info "Piper EN voice model downloaded" \
                        || warn "Piper EN model download failed (will retry at runtime)"
                fi
                # Download Thai model
                TH_ONNX="${PIPER_DIR}/models/th_TH-tacotron_ddc-medium.onnx"
                TH_JSON="${TH_ONNX}.json"
                if [[ ! -f "${TH_ONNX}" ]]; then
                    curl -fsSL "${HF_BASE}/th/th_TH/tacotron_ddc/medium/th_TH-tacotron_ddc-medium.onnx" -o "${TH_ONNX}" 2>/dev/null \
                        && curl -fsSL "${HF_BASE}/th/th_TH/tacotron_ddc/medium/th_TH-tacotron_ddc-medium.onnx.json" -o "${TH_JSON}" 2>/dev/null \
                        && info "Piper Thai voice model downloaded" \
                        || warn "Piper Thai model download failed (will retry at runtime)"
                fi
            else
                warn "Piper TTS extraction failed — will retry at runtime"
            fi
        else
            warn "Piper TTS download failed — will install at runtime"
        fi
    fi
fi

# 6. Copy installer to known location for wrapper
mkdir -p /opt/siam-synapse
if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]:-}" ]]; then
    cp "${BASH_SOURCE[0]}" /opt/siam-synapse/install.sh
    chmod +x /opt/siam-synapse/install.sh
    info "Installer copied  → /opt/siam-synapse/install.sh"
else
    # Running via curl | bash — download script directly
    curl -fsSL https://raw.githubusercontent.com/jkfastdevth/ottoclaw-worker/main/install.sh \
        -o /opt/siam-synapse/install.sh 2>/dev/null \
        && chmod +x /opt/siam-synapse/install.sh \
        && info "Installer downloaded → /opt/siam-synapse/install.sh" \
        || warn "Could not save installer to /opt/siam-synapse/install.sh"
fi

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
