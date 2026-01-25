#!/bin/bash

# Build or extract the toolchain
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Setting ARM OpenWrt toolchain${NC}"
echo -e "${GREEN}========================================${NC}"

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"
TOOLCHAIN_TAR_PATH="${TOOLCHAIN_TAR_PATH:-/build/toolchain.tar.gz}"

# Skip if toolchain is already mounted with required files
if [ -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${GREEN}[OK] Toolchain already present, skipping download${NC}"
    "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" --version
    exit 0
fi

echo -e "${YELLOW}Note: this script requires you to provide the toolchain${NC}"
echo -e "${YELLOW}Option 1: mount the toolchain to ${TOOLCHAIN_PATH}${NC}"
echo -e "${YELLOW}Option 2: place the toolchain archive at /build/toolchain.tar.gz${NC}"
echo ""

# Check for toolchain archive
if [ -f "$TOOLCHAIN_TAR_PATH" ]; then
    echo -e "${YELLOW}Extracting toolchain...${NC}"
    mkdir -p "$TOOLCHAIN_PATH"
    tar xzf "$TOOLCHAIN_TAR_PATH" -C "$TOOLCHAIN_PATH"

    # If the toolchain is nested inside a subdirectory, flatten it
    if [ ! -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
        # Skip the root TOOLCHAIN_PATH itself; we only want nested toolchain dirs
        NESTED_DIR=$(find "$TOOLCHAIN_PATH" -mindepth 1 -maxdepth 2 -type d -name "*toolchain*" | head -1)
        if [ -n "$NESTED_DIR" ] && [ "$NESTED_DIR" != "$TOOLCHAIN_PATH" ]; then
            rsync -a "$NESTED_DIR"/ "$TOOLCHAIN_PATH"/
            rm -rf "$NESTED_DIR"
        fi
    fi
    
    if [ -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
        echo -e "${GREEN}[OK] Toolchain extracted successfully${NC}"
        exit 0
    fi
fi

echo -e "${RED}Error: toolchain not found${NC}"
echo -e "${YELLOW}Please provide the toolchain in one of the following ways:${NC}"
echo "1. Mount the toolchain directory via Docker Compose"
echo "2. Package it as toolchain.tar.gz and place it in the project root"
exit 1
