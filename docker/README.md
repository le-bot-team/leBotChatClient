# Docker 编译环境使用指南

本项目提供了 Docker Compose 环境来进行 ARM 交叉编译，无需在主机上安装交叉编译工具链。

## 前置条件

- Docker
- Docker Compose

## 快速开始

### 方式 1：使用本地工具链（推荐）

如果你已经有本地的 OpenWrt 工具链：

1. 编辑 `compose.yaml`，取消注释工具链挂载行并修改路径：
```yaml
volumes:
  - ~/coding/t113-s3_sunxi-musl_toolchain:/opt/toolchain:ro
```

2. 构建 Docker 镜像：
```bash
docker compose build
```

3. 编译依赖库（首次运行或依赖库更新时）：
```bash
docker compose run builder bash /build/build_dependencies.sh
```

4. 编译应用程序：
```bash
docker compose run builder bash /build/build_app.sh
```

5. 一键编译（依赖库已编译后）：
```bash
docker compose run builder bash -c "/build/build_dependencies.sh && /build/build_app.sh"
```

### 方式 2：使用工具链压缩包

如果你没有本地工具链，或想要打包工具链到容器中：

1. 将工具链打包：
```bash
cd ~/coding/
tar czf ~/coding/leBotChatClient/toolchain.tar.gz t113-s3_sunxi-musl_toolchain/
```

2. 构建 Docker 镜像：
```bash
docker compose build
```

3. 设置工具链：
```bash
docker compose run builder bash /build/build_toolchain.sh
```

4. 后续步骤与方式 1 相同

## 常用命令

### 查看帮助信息
```bash
docker compose run builder
```

### 进入交互式 Shell
```bash
docker compose run builder bash
```

### 清理构建缓存
```bash
docker compose run builder go clean -cache -modcache
```

### 重新构建 Docker 镜像
```bash
docker compose build --no-cache
```

### 清理 Docker 卷
```bash
docker compose down -v
```

## 目录结构

```
leBotChatClient/
├── compose.yaml              # Docker Compose 配置
├── .dockerignore            # Docker 忽略文件
├── docker/                  # Docker 相关文件
│   ├── Dockerfile           # Docker 镜像定义
│   ├── build_toolchain.sh   # 工具链设置脚本
│   ├── build_dependencies.sh # 依赖库编译脚本
│   └── build_app.sh         # 应用程序编译脚本
├── build/                   # 编译产物目录
│   └── chat_client_openwrt  # 交叉编译的可执行文件
└── ...
```

## 持久化数据

Docker Compose 配置使用了以下卷来持久化数据，避免重复编译：

- `go-mod-cache`: Go 模块缓存
- `go-build-cache`: Go 构建缓存
- `toolchain-libs`: 工具链和依赖库

如需清理这些缓存：
```bash
docker compose down -v
```

## 编译流程说明

### 1. 工具链设置
- 检测并挂载 OpenWrt 交叉编译工具链
- 包含 `arm-openwrt-linux-muslgnueabi-gcc` 等工具

### 2. 依赖库编译
- **ALSA-lib**: 音频底层库（版本 1.2.8）
- **PortAudio**: 跨平台音频 I/O 库（v190700_20210406）

### 3. Go 应用编译
- 使用 CGO 进行交叉编译
- 静态链接 PortAudio 和 ALSA 库
- 输出精简的可执行文件

## 常见问题

### Q: 编译速度慢？
A: 首次编译需要下载并编译依赖库，后续编译会使用缓存，速度会大幅提升。

### Q: 工具链找不到？
A: 确保 `compose.yaml` 中的工具链路径正确，或使用工具链压缩包方式。

### Q: 编译失败？
A: 检查：
1. 工具链是否正确安装
2. 依赖库是否编译成功
3. 查看详细错误日志

### Q: 如何更新依赖库版本？
A: 修改 `docker/build_dependencies.sh` 中的版本号，然后重新运行：
```bash
docker compose run builder bash /build/build_dependencies.sh
```

## 与现有脚本的对应关系

- `build_alsa.sh` → `docker/build_dependencies.sh` (ALSA 部分)
- `build_portaudio.sh` → `docker/build_dependencies.sh` (PortAudio 部分)
- `build_arm.sh` → `docker/build_app.sh`

## 技术支持

如遇到问题，请检查：
1. Docker 和 Docker Compose 版本
2. 工具链完整性
3. 磁盘空间是否充足（至少需要 2GB）
