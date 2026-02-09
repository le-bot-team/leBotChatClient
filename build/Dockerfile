FROM golang:latest

# Install cross-compilation tools and dependencies
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
    rsync \
    && rm -rf /var/lib/apt/lists/*

# Toolchain directory
ENV TOOLCHAIN_PATH=/opt/toolchain
ENV PATH="${TOOLCHAIN_PATH}/bin:${PATH}"
ENV STAGING_DIR="${TOOLCHAIN_PATH}"

# Cross compilation tool configuration
ENV CC=arm-openwrt-linux-muslgnueabi-gcc
ENV CXX=arm-openwrt-linux-muslgnueabi-g++
ENV AR=arm-openwrt-linux-muslgnueabi-ar
ENV RANLIB=arm-openwrt-linux-muslgnueabi-ranlib

# pkg-config configuration
ENV PKG_CONFIG_PATH="${TOOLCHAIN_PATH}/lib/pkgconfig"
ENV PKG_CONFIG_LIBDIR="${TOOLCHAIN_PATH}/lib/pkgconfig"
ENV PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN_PATH}"

# Go cross-compilation configuration
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm
ENV GOARM=7

# Create toolchain directory
RUN mkdir -p ${TOOLCHAIN_PATH}

# Copy build scripts
COPY build_toolchain.sh build_dependencies.sh build_app.sh /build/
RUN chmod +x /build/*.sh

CMD ["/bin/bash"]
