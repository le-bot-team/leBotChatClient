#!/bin/bash

# 编译 ALSA-lib 和 PortAudio
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"
ALSA_VERSION="1.2.8"
PORTAUDIO_VERSION="v190700_20210406"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}编译音频依赖库${NC}"
echo -e "${GREEN}========================================${NC}"

# 检查工具链
if [ ! -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${RED}错误：工具链未找到${NC}"
    exit 1
fi

# 检查依赖库是否已经存在
if [ -f "${TOOLCHAIN_PATH}/lib/libasound.a" ] && [ -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
    echo -e "${GREEN}✓ 依赖库已存在，跳过编译${NC}"
    ls -lh "${TOOLCHAIN_PATH}/lib/libasound.a"
    ls -lh "${TOOLCHAIN_PATH}/lib/libportaudio.a"
    exit 0
fi

# 设置环境变量
export PATH="${TOOLCHAIN_PATH}/bin:$PATH"
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib
export STAGING_DIR="${TOOLCHAIN_PATH}"

# 构建 ALSA-lib
if [ ! -f "${TOOLCHAIN_PATH}/lib/libasound.a" ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}编译 ALSA-lib${NC}"
    echo -e "${YELLOW}========================================${NC}"
    
    WORK_DIR="/tmp/alsa_build"
    rm -rf "$WORK_DIR"
    mkdir -p "$WORK_DIR"
    cd "$WORK_DIR"
    
    echo -e "${YELLOW}下载 ALSA-lib 源码...${NC}"
    wget "https://www.alsa-project.org/files/pub/lib/alsa-lib-${ALSA_VERSION}.tar.bz2"
    tar xjf "alsa-lib-${ALSA_VERSION}.tar.bz2"
    cd "alsa-lib-${ALSA_VERSION}"
    
    echo -e "${YELLOW}配置 ALSA-lib...${NC}"
    ./configure \
        --host=arm-openwrt-linux-muslgnueabi \
        --prefix="${TOOLCHAIN_PATH}" \
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
    
    echo -e "${YELLOW}编译 ALSA-lib...${NC}"
    make -j$(nproc)
    make install
    
    if [ -f "${TOOLCHAIN_PATH}/lib/libasound.a" ]; then
        echo -e "${GREEN}✓ ALSA-lib 编译成功${NC}"
    else
        echo -e "${RED}✗ ALSA-lib 编译失败${NC}"
        exit 1
    fi
    
    rm -rf "$WORK_DIR"
fi

# 构建 PortAudio
if [ ! -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}编译 PortAudio${NC}"
    echo -e "${YELLOW}========================================${NC}"
    
    WORK_DIR="/tmp/portaudio_build"
    rm -rf "$WORK_DIR"
    mkdir -p "$WORK_DIR"
    cd "$WORK_DIR"
    
    # 设置 pkg-config 环境变量
    export PKG_CONFIG_PATH="${TOOLCHAIN_PATH}/lib/pkgconfig:${TOOLCHAIN_PATH}/share/pkgconfig"
    export PKG_CONFIG_LIBDIR="${TOOLCHAIN_PATH}/lib/pkgconfig"
    export PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_PATH}"
    export PKG_CONFIG="pkg-config"
    export PKG_CONFIG_ALLOW_SYSTEM_CFLAGS=1
    export PKG_CONFIG_ALLOW_SYSTEM_LIBS=1
    
    echo -e "${YELLOW}下载 PortAudio 源码...${NC}"
    wget "http://files.portaudio.com/archives/pa_stable_${PORTAUDIO_VERSION}.tgz"
    tar xzf "pa_stable_${PORTAUDIO_VERSION}.tgz"
    cd portaudio
    
    ALSA_CFLAGS="-I${TOOLCHAIN_PATH}/include"
    ALSA_LIBS="-L${TOOLCHAIN_PATH}/lib -lasound -lm -ldl -lpthread -lrt"
    
    echo -e "${YELLOW}配置 PortAudio...${NC}"
    ./configure \
        --host=arm-openwrt-linux-muslgnueabi \
        --prefix="${TOOLCHAIN_PATH}" \
        --with-alsa \
        --without-jack \
        --without-oss \
        --disable-shared \
        --enable-static \
        CFLAGS="-O2 -fPIC ${ALSA_CFLAGS}" \
        CPPFLAGS="${ALSA_CFLAGS}" \
        LDFLAGS="-L${TOOLCHAIN_PATH}/lib" \
        LIBS="-lm -ldl -lpthread -lrt" \
        ALSA_CFLAGS="${ALSA_CFLAGS}" \
        ALSA_LIBS="${ALSA_LIBS}" \
        ac_cv_header_alsa_asoundlib_h=yes \
        ac_cv_lib_asound_snd_pcm_open=yes
    
    echo -e "${YELLOW}编译 PortAudio...${NC}"
    make -j$(nproc)
    make install
    
    if [ -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
        echo -e "${GREEN}✓ PortAudio 编译成功${NC}"
    else
        echo -e "${RED}✗ PortAudio 编译失败${NC}"
        exit 1
    fi
    
    rm -rf "$WORK_DIR"
fi

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}所有依赖库编译完成！${NC}"
echo -e "${GREEN}========================================${NC}"
ls -lh "${TOOLCHAIN_PATH}/lib/libasound.a"
ls -lh "${TOOLCHAIN_PATH}/lib/libportaudio.a"
