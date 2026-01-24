# Docker Build Environment Guide

This project provides a Docker Compose setup for ARM cross-compilation, so you do not need to install the toolchain on the host.

## Prerequisites

- Docker
- Docker Compose

## Quick start

### Option 1: use a local toolchain (recommended)

If you already have a local OpenWrt toolchain:

1. Edit `compose.yaml`, uncomment the toolchain mount line, and update the path:
```yaml
volumes:
  - ~/coding/t113-s3_sunxi-musl_toolchain:/opt/toolchain:ro
```

2. Build the Docker image:
```bash
docker compose build
```

3. Build the dependency libraries (first run or when deps change):
```bash
docker compose run builder bash /build/build_dependencies.sh
```

4. Build the application:
```bash
docker compose run builder bash /build/build_app.sh
```

5. One-shot build (after dependencies are built):
```bash
docker compose run builder bash -c "/build/build_dependencies.sh && /build/build_app.sh"
```

### Option 2: use a toolchain archive

If you do not have a local toolchain or want to package it into the container:

1. Package the toolchain:
```bash
cd ~/coding/
tar czf ~/coding/leBotChatClient/toolchain.tar.gz t113-s3_sunxi-musl_toolchain/
```

2. Build the Docker image:
```bash
docker compose build
```

3. Set up the toolchain:
```bash
docker compose run builder bash /build/build_toolchain.sh
```

4. Subsequent steps are the same as Option 1

## Common commands

### Show help
```bash
docker compose run builder
```

### Enter an interactive shell
```bash
docker compose run builder bash
```

### Clean Go build cache
```bash
docker compose run builder go clean -cache -modcache
```

### Rebuild Docker image
```bash
docker compose build --no-cache
```

### Clean Docker volumes
```bash
docker compose down -v
```

## Directory layout

```
leBotChatClient/
├── compose.yaml              # Docker Compose config
├── .dockerignore            # Docker ignore file
├── docker/                  # Docker-related files
│   ├── Dockerfile           # Docker image definition
│   ├── build_toolchain.sh   # Toolchain setup script
│   ├── build_dependencies.sh # Dependency build script
│   └── build_app.sh         # Application build script
├── build/                   # Build output directory
│   └── chat_client_openwrt  # Cross-compiled binary
└── ...
```

## Persistent data

Docker Compose uses the following volumes to persist data and avoid rebuilding:

- `go-mod-cache`: Go module cache
- `go-build-cache`: Go build cache
- `toolchain-libs`: Toolchain and dependency libraries

To clean these caches:
```bash
docker compose down -v
```

## Build flow

### 1. Toolchain setup
- Detect and mount the OpenWrt cross toolchain
- Includes `arm-openwrt-linux-muslgnueabi-gcc` and related tools

### 2. Dependency build
- **ALSA-lib**: audio base library (version 1.2.8)
- **PortAudio**: cross-platform audio I/O (v190700_20210406)

### 3. Go app build
- Use CGO for cross-compilation
- Statically link PortAudio and ALSA
- Output a slim executable

## FAQ

### Q: Build is slow?
A: The first build downloads and builds dependencies. Later builds use caches and are much faster.

### Q: Toolchain not found?
A: Make sure the toolchain path in `compose.yaml` is correct, or use the toolchain archive method.

### Q: Build failed?
A: Check:
1. Whether the toolchain is installed correctly
2. Whether dependencies built successfully
3. The detailed error logs

### Q: How to update dependency versions?
A: Edit the versions in `docker/build_dependencies.sh`, then rerun:
```bash
docker compose run builder bash /build/build_dependencies.sh
```

## Mapping to existing scripts

- `build_alsa.sh` → `docker/build_dependencies.sh` (ALSA part)
- `build_portaudio.sh` → `docker/build_dependencies.sh` (PortAudio part)
- `build_arm.sh` → `docker/build_app.sh`

## Support

If you run into issues, check:
1. Docker and Docker Compose versions
2. Toolchain integrity
3. Disk space (at least 2GB required)

### Dev environment shell (auto-sets toolchain & deps)
```bash
# Start dev shell; toolchain + dependencies run automatically before dropping to bash
# (uses shared caches: toolchain-libs, go-mod-cache, go-build-cache)
docker compose run dev

# From inside the shell, build the app as needed
/build/build_app.sh
```
