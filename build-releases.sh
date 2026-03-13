#!/usr/bin/env bash

# OttoClaw Binary Release Builder
# Cross-compiles ottoclaw-brain and siam-worker for multiple platforms.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${1:-v1.0.0}"
RELEASE_DIR="${SCRIPT_DIR}/releases/${VERSION}"

mkdir -p "${RELEASE_DIR}"

# Platforms to build for: OS/ARCH/EXT/SUFFIX
PLATFORMS=(
    "linux/amd64//linux-amd64"
    "linux/arm64//linux-arm64"
    "linux/arm64//android-arm64" # For Termux (using linux/arm64 is safer)
)

build_platform() {
    local os=$1
    local arch=$2
    local ext=$3
    local suffix=$4
    local target_name="ottoclaw-worker-${suffix}.tar.gz"
    local temp_build_dir=$(mktemp -d)

    echo "----------------------------------------------------------------"
    echo "  Building for ${os}/${arch} -> ${target_name}"
    echo "----------------------------------------------------------------"

    # 1. Build Brain (ottoclaw)
    echo "  [1/2] Building ottoclaw-brain..."
    pushd "${SCRIPT_DIR}/ottoclaw" >/dev/null
    # Prepare workspace for embedding
    local ONBOARD_DIR="cmd/ottoclaw/internal/onboard"
    mkdir -p "${ONBOARD_DIR}"
    rm -rf "${ONBOARD_DIR}/workspace"
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "${temp_build_dir}/ottoclaw-brain${ext}" ./cmd/ottoclaw
    popd >/dev/null

    # 2. Build Arm (siam-worker)
    echo "  [2/2] Building siam-worker..."
    pushd "${SCRIPT_DIR}/siam-arm" >/dev/null
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -ldflags="-s -w" -o "${temp_build_dir}/siam-worker${ext}" .
    popd >/dev/null

    # 3. Copy support files
    cp "${SCRIPT_DIR}/install.sh" "${temp_build_dir}/"
    cp "${SCRIPT_DIR}/install-termux.sh" "${temp_build_dir}/"
    cp -r "${SCRIPT_DIR}/skills" "${temp_build_dir}/"
    cp -r "${SCRIPT_DIR}/workspace" "${temp_build_dir}/"

    # 4. Package
    pushd "${temp_build_dir}" >/dev/null
    tar -czf "${RELEASE_DIR}/${target_name}" *
    popd >/dev/null

    rm -rf "${temp_build_dir}"
    echo "  ✓ Generated: ${RELEASE_DIR}/${target_name}"
}

for p in "${PLATFORMS[@]}"; do
    IFS='/' read -r os arch ext suffix <<< "$p"
    build_platform "$os" "$arch" "$ext" "$suffix"
done

echo "================================================================"
echo "  Build Complete! Releases are in: ${RELEASE_DIR}"
echo "================================================================"
ls -lh "${RELEASE_DIR}"
