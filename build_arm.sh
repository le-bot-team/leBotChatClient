#!/bin/bash

set -e  # 遇到错误立即退出

# 工具链路径
TOOLCHAIN_DIR="/home/$USER/coding/t113-s3_sunxi-musl_toolchain"
export PATH="${TOOLCHAIN_DIR}/bin:$PATH"

# OpenWrt 需要的环境变量
export STAGING_DIR="${TOOLCHAIN_DIR}"

# 交叉编译配置
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib

# Go 编译配置
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm
export GOARM=7

# CGO 编译器和链接器标志
# 添加工具链的 include 和 lib 路径
# 注意：明确指定 portaudio 的链接参数，避免使用 ALSA
export CGO_CFLAGS="-I${TOOLCHAIN_DIR}/include"
export CGO_LDFLAGS="-L${TOOLCHAIN_DIR}/lib -L${TOOLCHAIN_DIR}/arm-openwrt-linux-muslgnueabi/lib -lportaudio -lm -lpthread"

# 配置 pkg-config 使用工具链的库（仅使用工具链，不使用系统库）
export PKG_CONFIG_PATH="${TOOLCHAIN_DIR}/lib/pkgconfig"
export PKG_CONFIG_LIBDIR="${TOOLCHAIN_DIR}/lib/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_DIR}"

echo "=================================="
echo "编译配置："
echo "工具链: ${TOOLCHAIN_DIR}"
echo "CC: ${CC}"
echo "STAGING_DIR: ${STAGING_DIR}"
echo "PKG_CONFIG_PATH: ${PKG_CONFIG_PATH}"
echo "=================================="

# 检查工具链
if ! command -v ${CC} &> /dev/null; then
    echo "错误: 找不到编译器 ${CC}"
    exit 1
fi

echo "编译器版本:"
${CC} --version | head -1

# 检查 portaudio 库
echo ""
echo "检查 portaudio 库..."
if ! find ${TOOLCHAIN_DIR} -name "libportaudio*" | grep -q .; then
    echo "警告: 工具链中未找到 portaudio 库"
    echo "需要为 OpenWrt 交叉编译 portaudio"
    echo ""
    echo "解决方案："
    echo "1. 使用 OpenWrt SDK 编译 portaudio 包"
    echo "2. 或者禁用音频功能编译（需要修改代码）"
    echo ""
    exit 1
fi

# 创建输出目录
mkdir -p ./build

# 编译
echo ""
echo "开始编译..."
go build -ldflags="-s -w" -o ./build/chat_client_openwrt ./cmd

# 显示结果
if [ -f ./build/chat_client_openwrt ]; then
    echo ""
    echo "=================================="
    echo "编译成功！"
    echo ""
    file ./build/chat_client_openwrt
    ls -lh ./build/chat_client_openwrt
    echo "=================================="
else
    echo "编译失败！"
    exit 1
fi