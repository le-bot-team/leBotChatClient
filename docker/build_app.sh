#!/bin/bash

# 编译 Go 应用程序
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}编译 Go 应用程序${NC}"
echo -e "${GREEN}========================================${NC}"

# 检查工具链
if [ ! -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${RED}错误：工具链未找到${NC}"
    exit 1
fi

# 检查依赖库
if [ ! -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
    echo -e "${RED}错误：PortAudio 库未找到${NC}"
    echo -e "${YELLOW}请先运行依赖库构建脚本${NC}"
    exit 1
fi

# 设置环境变量
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

echo -e "${YELLOW}编译配置：${NC}"
echo "工具链: ${TOOLCHAIN_PATH}"
echo "CC: ${CC}"
echo "GOARCH: ${GOARCH}"
echo "GOARM: ${GOARM}"
echo ""

cd /workspace

echo -e "${YELLOW}清理旧的构建...${NC}"
go clean -cache
rm -rf ./build/
mkdir -p ./build/

echo -e "${YELLOW}下载 Go 模块...${NC}"
go mod download

echo -e "${YELLOW}开始编译...${NC}"
go build -v -ldflags="-s -w" -o ./build/chat_client_openwrt ./cmd

if [ -f ./build/chat_client_openwrt ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}编译成功！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    file ./build/chat_client_openwrt
    ls -lh ./build/chat_client_openwrt
    echo ""
    echo -e "${YELLOW}可执行文件位置：${NC}./build/chat_client_openwrt"
else
    echo -e "${RED}编译失败！${NC}"
    exit 1
fi
