# 更新日志

## [未发布] - 2025-11-14

### 新增功能

#### WebSocket 消息类型
- 添加 `outputTextStream` 消息处理：实时接收助手和用户的文本流
- 添加 `outputTextComplete` 消息处理：文本传输完成通知
- 添加 `chatComplete` 消息处理：聊天会话完成通知（包含错误处理）
- 添加 `cancelOutput` 请求方法：可以取消正在进行的输出
- 添加 `clearContext` 请求方法：清除对话上下文

#### 音频播放打断逻辑
- 实现智能打断：当用户发送新消息时（文本长度>=2），自动停止当前播放
- 添加 `Player.StopPlayback()` 方法：立即停止音频播放
- 添加 `Player.ClearBuffer()` 方法：清空音频缓冲区
- 添加 `RingBuffer.Clear()` 方法：支持快速清空缓冲区

#### 配置增强
- `DeviceConfig` 添加 `Timezone` 字段：支持时区配置（例如："Asia/Shanghai"）
- 默认启用 `OutputText` 选项以支持打断逻辑
- `UpdateConfigRequest` 和 `SendUpdateConfig` 支持发送时区信息

### 改进

#### 音频播放
- 优化播放停止逻辑：更平滑的打断体验
- 改进缓冲区管理：支持动态清空和重置

#### 消息处理
- 增强错误日志：`chatComplete` 消息会显示详细的错误代码和消息
- 改进日志输出：所有新消息类型都有详细的日志记录
- 完善 `outputAudioComplete` 响应：现在包含 `chatId` 和 `conversationId`

### 对齐前端实现
本次更新主要参考了前端 `ChatPage.vue` 的实现，实现了以下对齐：

1. **WebSocket 消息协议**：完全对齐所有消息类型和数据结构
2. **打断逻辑**：实现与前端相同的打断机制（检测用户新消息并停止播放）
3. **文本流处理**：支持实时接收和处理文本流数据
4. **配置同步**：支持时区等新配置项

### 技术细节

#### 新增类型定义（`internal/websocket/types.go`）
```go
- OutputTextStreamResponse
- OutputTextCompleteResponse  
- ChatCompleteResponse
- CancelOutputRequest
- ClearContextRequest
```

#### 新增接口方法（`internal/websocket/client.go`）
```go
- HandleOutputTextStream(resp *OutputTextStreamResponse)
- HandleOutputTextComplete(resp *OutputTextCompleteResponse)
- HandleChatComplete(resp *ChatCompleteResponse)
- SendCancelOutput(requestID string)
- SendClearContext(requestID string)
```

#### 新增音频控制方法（`internal/audio/player.go`）
```go
- StopPlayback()  // 停止播放
- ClearBuffer()   // 清空缓冲区
```

### 升级说明

#### 配置文件变化
如果使用自定义配置文件，请添加以下字段：
```json
{
  "device": {
    "outputText": true,
    "timezone": "Asia/Shanghai"
  }
}
```

#### API 变化
实现 `MessageHandler` 接口的代码需要添加新方法：
```go
HandleOutputTextStream(resp *OutputTextStreamResponse)
HandleOutputTextComplete(resp *OutputTextCompleteResponse)
HandleChatComplete(resp *ChatCompleteResponse)
```

### 已知问题
- 无

### 未来计划
- 实现流式音频播放优化（类似前端的 AudioContext 无缝播放）
- 添加音频可视化支持
- 支持多会话管理
