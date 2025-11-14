# 代码迁移指南

## 迁移概述

本次重构将原来的单一文件（834行）拆分成多个模块，实现了清晰的架构分层和职责分离。

## 主要变化

### 1. 文件结构变化

**原来 (旧代码)**:
```
websocket_client_chat.go  # 所有代码在一个文件中
```

**现在 (新代码)**:
```
cmd/
  main.go         # 程序入口
  app.go          # 应用核心逻辑
internal/
  config/         # 配置管理
  websocket/      # WebSocket处理
  audio/          # 音频处理
  control/        # 控制逻辑
pkg/
  buffer/         # 环形缓冲区
  utils/          # 工具函数
```

### 2. 全局变量消除

**旧代码**:
```go
// 全局变量
var g_updateFlag int32 = 0

const (
    deviceSN      = "DEV-001"
    sampleRate    = 16000
    // ... 更多常量
)
```

**新代码**:
```go
// 配置化管理
type Config struct {
    Audio     AudioConfig
    WebSocket WebSocketConfig
    Device    DeviceConfig
    // ...
}

// 应用状态管理
type App struct {
    updateFlag int32 // 封装在应用结构中
    // ...
}
```

### 3. 函数重构对照表

| 旧函数 | 新位置 | 说明 |
|--------|--------|------|
| `main()` | `cmd/main.go` | 简化的程序入口 |
| `fileMonitor()` | `internal/control/monitor.go` | 文件监控逻辑 |
| `recordThread()` | `internal/audio/recorder.go` | 录音处理 |
| `websocketThread()` | `internal/websocket/client.go` | WebSocket处理 |
| `playAudio()` | `internal/audio/player.go` | 音频播放 |
| `RingBuffer` | `pkg/buffer/ring.go` | 环形缓冲区 |
| `generateWAVHeader()` | `pkg/utils/audio.go` | 工具函数 |
| `convertSamplesToWAV()` | `pkg/utils/audio.go` | 工具函数 |

### 4. 接口抽象

**新增接口**:

```go
// 控制命令处理器
type Handler interface {
    HandleCommand(cmd Command)
}

// 音频数据处理器
type AudioHandler interface {
    OnAudioChunk(requestID string, samples []int16, isLast bool)
    OnRecordingComplete(requestID string, samples []int16)
}

// WebSocket消息处理器
type MessageHandler interface {
    HandleOutputAudioStream(resp *OutputAudioStreamResponse)
    HandleOutputAudioComplete(resp *OutputAudioCompleteResponse)
    HandleUpdateConfig(resp *UpdateConfigResponse)
}
```

## 运行方式变化

### 旧方式
```bash
go run websocket_client_chat.go
```

### 新方式
```bash
# 方式1: 直接运行
go run ./cmd

# 方式2: 构建后运行
go build -o chat-client ./cmd
./chat-client

# 方式3: 从cmd目录运行
cd cmd && go run .
```

## 配置变化

### 旧代码配置（硬编码）
```go
const (
    deviceSN         = "DEV-001"
    sampleRate       = 16000
    wsURL            = "wss://..."
    controlFile      = "/tmp/chat-control"
)
```

### 新代码配置（结构化）
```go
// 在 internal/config/config.go 中
func DefaultConfig() *Config {
    return &Config{
        Audio: AudioConfig{
            SampleRate: 16000,
            // ...
        },
        WebSocket: WebSocketConfig{
            URL: "wss://...",
            // ...
        },
        // ...
    }
}
```

## 错误处理改进

### 旧代码
```go
if err != nil {
    log.Printf("错误: %v", err)
    // 继续执行或简单返回
}
```

### 新代码
```go
if err != nil {
    return fmt.Errorf("具体操作失败: %w", err)
}

// 或在应用层统一处理
if err := app.Start(); err != nil {
    log.Fatalf("启动应用程序失败: %v", err)
}
```

## 并发处理改进

### 旧代码
```go
go recordThread(state)
go websocketThread(state)
go fileMonitor(state)
```

### 新代码
```go
// 统一的生命周期管理
func (app *App) Start() error {
    if err := app.wsClient.Start(); err != nil {
        return err
    }
    if err := app.fileMonitor.Start(); err != nil {
        return err
    }
    return nil
}

func (app *App) Stop() error {
    // 优雅关闭所有组件
    app.cancel()
    app.wg.Wait()
    return nil
}
```

## 测试支持

### 旧代码
- 难以进行单元测试
- 全局状态导致测试隔离困难

### 新代码
```go
// 可以轻松mock任何接口进行测试
type MockAudioHandler struct{}

func (m *MockAudioHandler) OnAudioChunk(requestID string, samples []int16, isLast bool) {
    // 测试实现
}

// 测试示例
recorder := audio.NewRecorder(&config.AudioConfig{}, &MockAudioHandler{})
```

## 扩展能力

### 新架构支持的扩展：

1. **新的音频格式**: 在 `pkg/utils` 中添加转换函数
2. **新的传输协议**: 实现 `MessageHandler` 接口
3. **配置文件支持**: 扩展 `config` 包
4. **插件系统**: 基于接口的插件架构
5. **监控和指标**: 添加 `metrics` 包

## 性能改进

1. **内存优化**: 减少了内存分配和复制
2. **并发优化**: 更好的goroutine管理
3. **网络优化**: 连接池和自动重连
4. **代码优化**: 更少的全局状态，更好的数据局部性

## 迁移建议

1. **备份原代码**: 保留 `websocket_client_chat.go` 作为参考
2. **逐步迁移**: 可以先运行新代码，确认功能正常
3. **配置调整**: 根据需要修改 `internal/config/config.go` 中的默认值
4. **测试验证**: 使用相同的测试场景验证新代码功能
5. **监控性能**: 对比新旧代码的性能表现

## 问题排查

如果遇到问题，可以：

1. **检查日志**: 新代码有更详细的日志输出
2. **对比行为**: 与原代码的行为进行对比
3. **分模块调试**: 可以独立测试各个模块
4. **查看配置**: 确认配置是否正确

## 后续优化建议

1. 添加配置文件支持（JSON/YAML）
2. 添加性能监控和指标
3. 实现插件系统
4. 添加更多的单元测试
5. 添加集成测试
6. 考虑添加Web管理界面
