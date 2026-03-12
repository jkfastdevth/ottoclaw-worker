#!/usr/bin/env bash
# OttoClaw Release Helper
# Automates: Sync -> Git Add -> Commit -> Push -> Tag

set -e

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${CYAN}${BOLD}══ OttoClaw Release Helper ══${RESET}\n"

# 1. Sync latest files
echo "🔄 Syncing latest installer scripts..."
cp install.sh install-termux.sh . 2>/dev/null || true
echo -e "  ${GREEN}✓${RESET} Synced."

# 2. Git Status
echo -e "\n${BOLD}Current changes:${RESET}"
git status -s

# 3. Commit Message
echo -ne "\n${BOLD}Commit message${RESET} (Enter for 'update: release'): "
read -r msg
msg="${msg:-update: release}"

# 4. Push to Main
echo -e "\n🚀 Pushing to main..."
git add .
git commit -m "$msg"
git push origin main
echo -e "  ${GREEN}✓${RESET} Pushed to GitHub main."

echo -e "\n${GREEN}${BOLD}✅ เสร็จสมบูรณ์!${RESET} โค้ดของคุณออนไลน์แล้วครับ 🐯🚀🛸"
