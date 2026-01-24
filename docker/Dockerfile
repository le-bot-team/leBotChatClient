FROM golang:1.21-bookworm

# 安装交叉编译所需的工具和依赖
RUN apt-get update && apt-get install -y \
    build-essential \
    wget \
    curl \
    git \
    pkg-config \
    autoconf \
    automake \
    libtool \
    file \
    python3 \
    && rm -rf /var/lib/apt/lists/*

# 设置工作目录
WORKDIR /build

# 工具链目录
ENV TOOLCHAIN_PATH=/opt/toolchain
ENV PATH="${TOOLCHAIN_PATH}/bin:${PATH}"
ENV STAGING_DIR="${TOOLCHAIN_PATH}"

# 交叉编译工具配置
ENV CC=arm-openwrt-linux-muslgnueabi-gcc
ENV CXX=arm-openwrt-linux-muslgnueabi-g++
ENV AR=arm-openwrt-linux-muslgnueabi-ar
ENV RANLIB=arm-openwrt-linux-muslgnueabi-ranlib

# pkg-config 配置
ENV PKG_CONFIG_PATH="${TOOLCHAIN_PATH}/lib/pkgconfig"
ENV PKG_CONFIG_LIBDIR="${TOOLCHAIN_PATH}/lib/pkgconfig"
ENV PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_PATH}"

# Go 交叉编译配置
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm
ENV GOARM=7

# 创建工具链目录
RUN mkdir -p ${TOOLCHAIN_PATH}

# 复制构建脚本
COPY build_toolchain.sh /build/
COPY build_dependencies.sh /build/

# 可选：复制已打包的工具链（存在则解压使用）
COPY toolchain.tar.gz /build/toolchain.tar.gz

# 构建工具链和依赖库（这会在镜像构建时执行）
# 如果需要在运行时构建，可以注释掉这两行
RUN chmod +x /build/build_toolchain.sh /build/build_dependencies.sh

CMD ["/bin/bash"]
