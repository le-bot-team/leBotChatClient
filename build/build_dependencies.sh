#!/bin/bash

# Build ALSA-lib and PortAudio
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TOOLCHAIN_PATH="${TOOLCHAIN_PATH:-/opt/toolchain}"
ALSA_VERSION="1.2.8"
PORTAUDIO_VERSION="v190700_20210406"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Build audio dependencies${NC}"
echo -e "${GREEN}========================================${NC}"

# Check toolchain
if [ ! -f "${TOOLCHAIN_PATH}/bin/arm-openwrt-linux-muslgnueabi-gcc" ]; then
    echo -e "${RED}Error: toolchain not found${NC}"
    exit 1
fi

# Skip if both libs already exist
#if [ -f "${TOOLCHAIN_PATH}/lib/libasound.a" ] && [ -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ]; then
#    echo -e "${GREEN}[OK] Dependencies already present, skipping build${NC}"
#    ls -lh "${TOOLCHAIN_PATH}/lib/libasound.a"
#    ls -lh "${TOOLCHAIN_PATH}/lib/libportaudio.a"
#    exit 0
#fi

# Set environment variables
export PATH="${TOOLCHAIN_PATH}/bin:$PATH"
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib
export STAGING_DIR="${TOOLCHAIN_PATH}"

# Build ALSA-lib
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Building ALSA-lib${NC}"
echo -e "${YELLOW}========================================${NC}"

WORK_DIR="/tmp/alsa_build"
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

echo -e "${YELLOW}Downloading ALSA-lib source...${NC}"
wget "https://www.alsa-project.org/files/pub/lib/alsa-lib-${ALSA_VERSION}.tar.bz2"
tar xjf "alsa-lib-${ALSA_VERSION}.tar.bz2"
cd "alsa-lib-${ALSA_VERSION}"

echo -e "${YELLOW}Configuring ALSA-lib...${NC}"
./configure \
    --host=arm-openwrt-linux-muslgnueabi \
    --prefix="${TOOLCHAIN_PATH}" \
    --enable-shared \
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

echo -e "${YELLOW}Building ALSA-lib...${NC}"
make -j$(nproc)
make install

if [ -f "${TOOLCHAIN_PATH}/lib/libasound.a" ]; then
    echo -e "${GREEN}[OK] ALSA-lib build succeeded${NC}"
else
    echo -e "${RED}[FAIL] ALSA-lib build failed${NC}"
    exit 1
fi

rm -rf "$WORK_DIR"
# End build ALSA-lib


# Build PortAudio
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Building PortAudio${NC}"
echo -e "${YELLOW}========================================${NC}"

WORK_DIR="/tmp/portaudio_build"
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"
# Set pkg-config environment
export PKG_CONFIG_PATH="${TOOLCHAIN_PATH}/lib/pkgconfig:${TOOLCHAIN_PATH}/share/pkgconfig"
export PKG_CONFIG_LIBDIR="${TOOLCHAIN_PATH}/lib/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_PATH}"
export PKG_CONFIG="pkg-config"
export PKG_CONFIG_ALLOW_SYSTEM_CFLAGS=1
export PKG_CONFIG_ALLOW_SYSTEM_LIBS=1

echo -e "${YELLOW}Downloading PortAudio source...${NC}"
wget "http://files.portaudio.com/archives/pa_stable_${PORTAUDIO_VERSION}.tgz"
tar xzf "pa_stable_${PORTAUDIO_VERSION}.tgz"
cd portaudio

ALSA_CFLAGS="-I${TOOLCHAIN_PATH}/include"
ALSA_LIBS="-L${TOOLCHAIN_PATH}/lib -lasound -lm -ldl -lpthread -lrt"

echo -e "${YELLOW}Configuring PortAudio...${NC}"
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

echo -e "${YELLOW}Building PortAudio...${NC}"
make -j$(nproc)
make install

if [ -f "${TOOLCHAIN_PATH}/lib/libportaudio.a" ] && [ -f "${TOOLCHAIN_PATH}/lib/libportaudio.so.2" ]; then
    echo -e "${GREEN}[OK] PortAudio build succeeded${NC}"
else
    echo -e "${RED}[FAIL] PortAudio build failed${NC}"
    exit 1
fi

rm -rf "$WORK_DIR"
# End build PortAudio

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All dependencies built${NC}"
echo -e "${GREEN}========================================${NC}"
ls -lh "${TOOLCHAIN_PATH}/lib/libasound.a"
ls -lh "${TOOLCHAIN_PATH}/lib/libportaudio.a"
ls -lh "${TOOLCHAIN_PATH}/lib/libportaudio.so.2"
