#!/bin/bash

# 构建或提取工具链
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}设置 ARM OpenWrt 工具链${NC}"
echo -e "${GREEN}========================================${NC}"

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"

# 如果工具链目录已经挂载且包含必要文件，则跳过下载
if [ -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${GREEN}✓ 工具链已存在，跳过下载${NC}"
    ${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc --version
    exit 0
fi

echo -e "${YELLOW}注意：此脚本需要你提供工具链${NC}"
echo -e "${YELLOW}选项 1：将工具链挂载到 ${TOOLCHAIN_PATH}${NC}"
echo -e "${YELLOW}选项 2：将工具链压缩包放到 /build/toolchain.tar.gz${NC}"
echo ""

# 检查是否有工具链压缩包
if [ -f "/build/toolchain.tar.gz" ]; then
    echo -e "${YELLOW}解压工具链...${NC}"
    tar xzf /build/toolchain.tar.gz -C /opt/
    
    # 查找解压后的工具链目录
    EXTRACTED_DIR=$(find /opt -maxdepth 1 -type d -name "*toolchain*" | head -1)
    if [ -n "$EXTRACTED_DIR" ] && [ "$EXTRACTED_DIR" != "$TOOLCHAIN_PATH" ]; then
        mv "$EXTRACTED_DIR" "$TOOLCHAIN_PATH"
    fi
    
    if [ -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
        echo -e "${GREEN}✓ 工具链解压成功${NC}"
        exit 0
    fi
fi

echo -e "${RED}错误：未找到工具链${NC}"
echo -e "${YELLOW}请使用以下方式之一提供工具链：${NC}"
echo "1. Docker Compose 挂载工具链目录"
echo "2. 将工具链打包为 toolchain.tar.gz 并放到项目根目录"
exit 1
