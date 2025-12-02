#!/bin/bash

# ALSA-lib 交叉编译脚本
set -e

TOOLCHAIN_PATH="$HOME/coding/t113-s3_sunxi-musl_toolchain"
WORK_DIR="/tmp/alsa_build"
ALSA_VERSION="1.2.8"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}ALSA-lib 交叉编译脚本${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 检查工具链
if [ ! -d "$TOOLCHAIN_PATH" ]; then
    echo -e "${RED}错误: 工具链目录不存在: $TOOLCHAIN_PATH${NC}"
    exit 1
fi

echo -e "${YELLOW}1. 清理并创建工作目录${NC}"
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

echo -e "${YELLOW}2. 下载 ALSA-lib 源码${NC}"
if [ ! -f "alsa-lib-${ALSA_VERSION}.tar.bz2" ]; then
    if command -v wget &> /dev/null; then
        wget "https://www.alsa-project.org/files/pub/lib/alsa-lib-${ALSA_VERSION}.tar.bz2"
    elif command -v curl &> /dev/null; then
        curl -L -O "https://www.alsa-project.org/files/pub/lib/alsa-lib-${ALSA_VERSION}.tar.bz2"
    else
        echo -e "${RED}错误: 需要 wget 或 curl 来下载源码${NC}"
        exit 1
    fi
fi

echo -e "${YELLOW}3. 解压源码${NC}"
tar xjf "alsa-lib-${ALSA_VERSION}.tar.bz2"
cd "alsa-lib-${ALSA_VERSION}"

echo -e "${YELLOW}4. 配置交叉编译环境${NC}"
export PATH="$TOOLCHAIN_PATH/bin:$PATH"
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib
export STAGING_DIR="$TOOLCHAIN_PATH"

echo -e "${YELLOW}5. 配置 ALSA-lib${NC}"
./configure \
    --host=arm-openwrt-linux-muslgnueabi \
    --prefix="$TOOLCHAIN_PATH" \
    --disable-shared \
    --enable-static \
    --disable-python \
    --disable-mixer \
    --disable-ucm \
    --disable-topology \
    --disable-alisp \
    --disable-old-symbols \
    --with-configdir=/etc/alsa \
    --with-plugindir=/usr/lib/alsa-lib \
    CFLAGS="-O2 -fPIC"

echo -e "${YELLOW}6. 编译 ALSA-lib${NC}"
make -j$(nproc)

echo -e "${YELLOW}7. 安装到工具链${NC}"
make install

echo -e "${YELLOW}8. 验证安装${NC}"
if [ -f "$TOOLCHAIN_PATH/lib/libasound.a" ]; then
    echo -e "${GREEN}✓ 静态库安装成功${NC}"
    ls -lh "$TOOLCHAIN_PATH/lib/libasound.a"
else
    echo -e "${RED}✗ 静态库安装失败${NC}"
    exit 1
fi

if [ -f "$TOOLCHAIN_PATH/include/alsa/asoundlib.h" ]; then
    echo -e "${GREEN}✓ 头文件安装成功${NC}"
else
    echo -e "${RED}✗ 头文件安装失败${NC}"
    exit 1
fi

if [ -f "$TOOLCHAIN_PATH/lib/pkgconfig/alsa.pc" ]; then
    echo -e "${GREEN}✓ pkg-config 文件安装成功${NC}"
    cat "$TOOLCHAIN_PATH/lib/pkgconfig/alsa.pc"
else
    echo -e "${YELLOW}警告: pkg-config 文件未找到${NC}"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}ALSA-lib 编译完成!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}库文件位置:${NC} $TOOLCHAIN_PATH/lib/libasound.a"
echo -e "${YELLOW}头文件位置:${NC} $TOOLCHAIN_PATH/include/alsa/"
echo ""
echo -e "${YELLOW}下一步: 运行 ./rebuild_portaudio.sh 重新编译 PortAudio${NC}"
echo ""

# 清理
read -p "是否删除临时编译目录? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    rm -rf "$WORK_DIR"
    echo -e "${GREEN}临时目录已清理${NC}"
fi
