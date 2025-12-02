# 为 OpenWrt 交叉编译 PortAudio

## 方法 1: 使用 OpenWrt SDK（推荐）

### 1. 下载 OpenWrt SDK
```bash
# 根据你的 OpenWrt 版本和架构下载对应的 SDK
# 示例：对于 sunxi/cortexa7 架构
wget https://downloads.openwrt.org/releases/22.03.5/targets/sunxi/cortexa7/openwrt-sdk-22.03.5-sunxi-cortexa7_gcc-11.2.0_musl.Linux-x86_64.tar.xz
tar xJf openwrt-sdk-*.tar.xz
cd openwrt-sdk-*
```

### 2. 配置 SDK
```bash
./scripts/feeds update -a
./scripts/feeds install portaudio
```

### 3. 编译 portaudio
```bash
make package/portaudio/compile V=s
```

### 4. 找到编译好的库
```bash
find ./staging_dir -name "libportaudio*"
```

### 5. 复制库文件到工具链
```bash
# 复制 .so 文件到工具链
cp ./staging_dir/toolchain-*/lib/libportaudio.* /home/$USER/coding/t113-s3_sunxi-musl_toolchain/lib/
# 复制头文件
cp -r ./staging_dir/target-*/usr/include/portaudio* /home/$USER/coding/t113-s3_sunxi-musl_toolchain/include/
```

## 方法 2: 手动交叉编译 PortAudio

### 1. 下载 PortAudio 源码
```bash
cd /tmp
wget http://files.portaudio.com/archives/pa_stable_v190700_20210406.tgz
tar xzf pa_stable_v190700_20210406.tgz
cd portaudio
```

### 2. 配置交叉编译
```bash
export PATH=/home/$USER/coding/t113-s3_sunxi-musl_toolchain/bin:$PATH
export CC=arm-openwrt-linux-muslgnueabi-gcc
export CXX=arm-openwrt-linux-muslgnueabi-g++
export AR=arm-openwrt-linux-muslgnueabi-ar
export RANLIB=arm-openwrt-linux-muslgnueabi-ranlib
export STAGING_DIR=/home/$USER/coding/t113-s3_sunxi-musl_toolchain

./configure \
    --host=arm-openwrt-linux-muslgnueabi \
    --prefix=/home/$USER/coding/t113-s3_sunxi-musl_toolchain \
    --with-alsa \
    --without-jack \
    --without-oss
```

### 3. 编译和安装
```bash
make
make install
```

## 方法 3: 禁用音频功能（最简单，但失去音频功能）

如果你不需要音频功能，可以修改代码移除 portaudio 依赖。这需要：

1. 移除 `internal/audio/player.go` 和 `recorder.go` 中的 portaudio 导入
2. 修改相关代码使其不依赖音频功能
3. 使用 `CGO_ENABLED=0` 编译

参考 `DISABLE_AUDIO.md` 文件（待创建）。
