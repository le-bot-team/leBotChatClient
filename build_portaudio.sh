#!/bin/bash

# PortAudio with ALSA 支持的交叉编译脚本
set -e

TOOLCHAIN_PATH="$HOME/coding/t113-s3_sunxi-musl_toolchain"
WORK_DIR="/tmp/portaudio_build"
PORTAUDIO_VERSION="v190700_20210406"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}PortAudio 交叉编译脚本 (with ALSA)${NC}"
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

echo -e "${YELLOW}2. 下载 PortAudio 源码${NC}"
if [ ! -f "pa_stable_${PORTAUDIO_VERSION}.tgz" ]; then
    if command -v wget &> /dev/null; then
        wget "http://files.portaudio.com/archives/pa_stable_${PORTAUDIO_VERSION}.tgz"
    elif command -v curl &> /dev/null; then
        curl -L -O "http://files.portaudio.com/archives/pa_stable_${PORTAUDIO_VERSION}.tgz"
    else
        echo -e "${RED}错误: 需要 wget 或 curl 来下载源码${NC}"
        exit 1
    fi
fi

echo -e "${YELLOW}3. 解压源码${NC}"
tar xzf "pa_stable_${PORTAUDIO_VERSION}.tgz"
cd portaudio

echo -e "${YELLOW}4. 配置交叉编译环境${NC}"
export PATH="$TOOLCHAIN_PATH/bin:$PATH"
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib
export STAGING_DIR="$TOOLCHAIN_PATH"

# 设置 pkg-config 环境变量
export PKG_CONFIG_PATH="$TOOLCHAIN_PATH/lib/pkgconfig:$TOOLCHAIN_PATH/share/pkgconfig"
export PKG_CONFIG_LIBDIR="$TOOLCHAIN_PATH/lib/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$TOOLCHAIN_PATH"

# 确保 pkg-config 使用交叉编译的库
export PKG_CONFIG="pkg-config"
export PKG_CONFIG_ALLOW_SYSTEM_CFLAGS=1
export PKG_CONFIG_ALLOW_SYSTEM_LIBS=1

echo -e "${YELLOW}5. 检查 ALSA 库${NC}"
if [ -f "$TOOLCHAIN_PATH/lib/libasound.a" ]; then
    echo -e "${GREEN}✓ 找到 ALSA 静态库: $TOOLCHAIN_PATH/lib/libasound.a${NC}"
    ls -lh "$TOOLCHAIN_PATH/lib/libasound.a"
    
    # 明确指定 ALSA 的路径
    ALSA_CFLAGS="-I$TOOLCHAIN_PATH/include"
    ALSA_LIBS="-L$TOOLCHAIN_PATH/lib -lasound -lm -ldl -lpthread -lrt"
    
    # 验证 pkg-config 是否能找到 alsa
    if pkg-config --exists alsa 2>/dev/null; then
        echo -e "${GREEN}✓ pkg-config 可以找到 ALSA${NC}"
        echo "  ALSA version: $(pkg-config --modversion alsa)"
    else
        echo -e "${YELLOW}⚠ pkg-config 找不到 ALSA，使用手动指定的路径${NC}"
    fi
else
    echo -e "${RED}✗ 错误: 未找到 ALSA 库${NC}"
    echo "请确保工具链包含 libasound.a"
    exit 1
fi

echo -e "${YELLOW}6. 配置 PortAudio${NC}"
./configure \
    --host=arm-openwrt-linux-muslgnueabi \
    --prefix="$TOOLCHAIN_PATH" \
    --with-alsa \
    --without-jack \
    --without-oss \
    --disable-shared \
    --enable-static \
    CFLAGS="-O2 -fPIC $ALSA_CFLAGS" \
    CPPFLAGS="$ALSA_CFLAGS" \
    LDFLAGS="-L$TOOLCHAIN_PATH/lib" \
    LIBS="-lm -ldl -lpthread -lrt" \
    ALSA_CFLAGS="$ALSA_CFLAGS" \
    ALSA_LIBS="$ALSA_LIBS" \
    ac_cv_header_alsa_asoundlib_h=yes \
    ac_cv_lib_asound_snd_pcm_open=yes

echo -e "${YELLOW}7. 查看配置结果${NC}"
echo ""
echo "============ 配置摘要 ============"
grep -A 20 "Configuration summary:" config.log || true
echo "=================================="
echo ""

# 检查是否启用了 ALSA (检查多个可能的标志)
if grep -q "PA_USE_ALSA" config.log || grep -q "ALSA.*yes" config.log || grep -q "HAVE_ALSA 1" config.log; then
    echo -e "${GREEN}✓ ALSA 支持已启用${NC}"
    # 显示更多详细信息
    if grep -q "#define PA_USE_ALSA 1" config.log; then
        echo -e "${GREEN}  - PA_USE_ALSA 宏已定义${NC}"
    fi
    if grep -q "pa_linux_alsa" config.log; then
        echo -e "${GREEN}  - ALSA 后端代码已包含${NC}"
    fi
else
    echo -e "${RED}✗ ALSA 支持未启用${NC}"
    echo -e "${YELLOW}查看详细配置日志:${NC}"
    grep -i "alsa" config.log | head -20
    echo ""
    read -p "是否继续编译? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo -e "${YELLOW}8. 编译 PortAudio${NC}"
make -j$(nproc)

echo -e "${YELLOW}9. 安装到工具链${NC}"
make install

echo -e "${YELLOW}10. 验证安装${NC}"
if [ -f "$TOOLCHAIN_PATH/lib/libportaudio.a" ]; then
    echo -e "${GREEN}✓ 静态库安装成功: $TOOLCHAIN_PATH/lib/libportaudio.a${NC}"
    ls -lh "$TOOLCHAIN_PATH/lib/libportaudio.a"
else
    echo -e "${RED}✗ 静态库安装失败${NC}"
fi

if [ -f "$TOOLCHAIN_PATH/include/portaudio.h" ]; then
    echo -e "${GREEN}✓ 头文件安装成功${NC}"
else
    echo -e "${RED}✗ 头文件安装失败${NC}"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}编译完成!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}库文件位置:${NC} $TOOLCHAIN_PATH/lib/libportaudio.a"
echo -e "${YELLOW}头文件位置:${NC} $TOOLCHAIN_PATH/include/portaudio.h"
echo ""
echo -e "${YELLOW}下一步:${NC}"
echo "1. 重新编译你的项目: ./build_arm.sh"
echo "2. 上传新的可执行文件到设备"
echo ""

# 清理
read -p "是否删除临时编译目录? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    rm -rf "$WORK_DIR"
    echo -e "${GREEN}临时目录已清理${NC}"
fi
