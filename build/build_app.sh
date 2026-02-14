#!/bin/bash

# Build Go application
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Build Go application${NC}"
echo -e "${GREEN}========================================${NC}"

# Check toolchain
if [ ! -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${RED}Error: toolchain not found${NC}"
    exit 1
fi

# Check dependencies
if [ ! -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
    echo -e "${RED}Error: PortAudio library not found${NC}"
    echo -e "${YELLOW}Please run the dependency build script first${NC}"
    exit 1
fi

# Set environment variables
export PATH="${TOOLCHAIN_PATH}/bin:$PATH"
export STAGING_DIR="${TOOLCHAIN_PATH}"

export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib

export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm
export GOARM=7

export CGO_CFLAGS="-I${TOOLCHAIN_PATH}/include"
export CGO_LDFLAGS="-L${TOOLCHAIN_PATH}/lib -L${TOOLCHAIN_PATH}/arm-openwrt-linux-muslgnueabi/lib -lportaudio -lasound -lm -lpthread -ldl -lrt"

export PKG_CONFIG_PATH="${TOOLCHAIN_PATH}/lib/pkgconfig"
export PKG_CONFIG_LIBDIR="${TOOLCHAIN_PATH}/lib/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_PATH}"

echo -e "${YELLOW}Build config:${NC}"
echo "Toolchain: ${TOOLCHAIN_PATH}"
echo "CC: ${CC}"
echo "GOARCH: ${GOARCH}"
echo "GOARM: ${GOARM}"
echo ""

cd /workspace

echo -e "${YELLOW}Cleaning old build...${NC}"
go clean -cache
rm -rf ./dist/
mkdir -p ./dist/

echo -e "${YELLOW}Downloading Go modules...${NC}"
go mod download

echo -e "${YELLOW}Starting build...${NC}"
go build -v -buildvcs=false -ldflags="-s -w" -o ./dist/chat_client_openwrt ./cmd

if [ -f ./dist/chat_client_openwrt ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Build succeeded${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    file ./dist/chat_client_openwrt
    ls -lh ./dist/chat_client_openwrt
    echo ""
    echo -e "${GREEN}Binary path:${NC} ./dist/chat_client_openwrt"
else
    echo -e "${RED}Build failed${NC}"
    exit 1
fi
