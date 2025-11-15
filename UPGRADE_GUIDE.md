# 升级指南：对齐前端 WebSocket 协议

本指南帮助你了解本次更新的主要变化和如何使用新功能。

## 主要变化概览

### 1. WebSocket 消息类型扩展

#### 新增的服务端消息
- **outputTextStream**: 实时接收文本流（助手和用户）
- **outputTextComplete**: 文本传输完成
- **chatComplete**: 会话完成（包含成功/失败状态）

#### 新增的客户端请求
- **cancelOutput**: 取消当前输出
- **clearContext**: 清除对话上下文

### 2. 打断逻辑实现

当用户发送新消息时，系统会自动：
1. 检测 `outputTextComplete` 中的用户消息（文本长度>=2）
2. 立即停止当前音频播放
3. 清空音频缓冲区
4. 准备接收新的助手响应

```go
// 在 HandleOutputTextComplete 中自动处理
if resp.Data.Role == "user" && len(resp.Data.Text) >= 2 {
    if app.player.IsPlaying() {
        log.Println("检测到用户新消息，执行打断逻辑")
        app.player.StopPlayback()
    }
}
```

### 3. 配置更新

#### DeviceConfig 新增字段
```go
type DeviceConfig struct {
    // ... 其他字段
    Timezone string `json:"timezone,omitempty"` // 时区配置
}
```

#### 默认配置变化
```go
Device: DeviceConfig{
    OutputText: true,              // 现在默认启用（用于支持打断）
    Timezone:   "Asia/Shanghai",   // 默认时区
}
```

## 使用示例

### 1. 基本使用（无需修改）

如果你使用的是默认配置和 `App` 结构，无需任何修改即可享受新功能：

```go
app := NewApp()
if err := app.Start(); err != nil {
    log.Fatal(err)
}
app.Wait()
```

### 2. 手动取消输出

```go
// 发送取消输出请求
requestID := utils.GenerateRequestID(deviceConfig.SerialNumber)
if err := wsClient.SendCancelOutput(requestID); err != nil {
    log.Printf("取消输出失败: %v", err)
}
```

### 3. 清除对话上下文

```go
// 清除对话历史
requestID := utils.GenerateRequestID(deviceConfig.SerialNumber)
if err := wsClient.SendClearContext(requestID); err != nil {
    log.Printf("清除上下文失败: %v", err)
}
```

### 4. 手动打断播放

```go
// 如果需要手动打断播放
if player.IsPlaying() {
    player.StopPlayback() // 停止播放并清空缓冲区
}
```

## 实现自定义 MessageHandler

如果你实现了自己的 `MessageHandler`，需要添加新方法：

```go
type MyHandler struct {
    // ...
}

// 必须实现的新方法
func (h *MyHandler) HandleOutputTextStream(resp *websocket.OutputTextStreamResponse) {
    // 处理文本流
    log.Printf("收到文本: %s (角色: %s)", resp.Data.Text, resp.Data.Role)
}

func (h *MyHandler) HandleOutputTextComplete(resp *websocket.OutputTextCompleteResponse) {
    // 处理文本完成
    log.Printf("文本完成: %s", resp.Data.Text)
    
    // 可选：实现打断逻辑
    if resp.Data.Role == "user" {
        // 用户新消息，执行打断
    }
}

func (h *MyHandler) HandleChatComplete(resp *websocket.ChatCompleteResponse) {
    // 处理会话完成
    if !resp.Success {
        // 处理错误
        for _, err := range resp.Data.Errors {
            log.Printf("错误 [%d]: %s", err.Code, err.Message)
        }
    }
}
```

## 测试打断功能

1. 启动客户端并开始录音
2. 让助手开始回复（音频播放中）
3. 在助手说话时，再次发送语音消息
4. 观察日志输出，应该看到：
   ```
   检测到用户新消息，执行打断逻辑
   打断播放，停止音频流...
   已清除音频缓冲区
   ```

## 与前端协议对齐

本次更新完全对齐了前端 `ChatPage.vue` 的实现：

| 功能 | 前端 | Go 客户端 | 状态 |
|------|------|-----------|------|
| outputTextStream | ✅ | ✅ | 完成 |
| outputTextComplete | ✅ | ✅ | 完成 |
| chatComplete | ✅ | ✅ | 完成 |
| cancelOutput | ✅ | ✅ | 完成 |
| clearContext | ✅ | ✅ | 完成 |
| 打断逻辑 | ✅ | ✅ | 完成 |
| 时区配置 | ✅ | ✅ | 完成 |
| 文本输出 | ✅ | ✅ | 完成 |

## 故障排除

### 打断功能不工作
- 确认 `OutputText` 配置为 `true`
- 检查日志中是否收到 `outputTextComplete` 消息
- 确认 `HandleOutputTextComplete` 方法被正确调用

### 文本消息未收到
- 确认 WebSocket 连接正常
- 检查 `updateConfig` 请求中 `outputText` 字段为 `true`
- 查看服务端日志确认配置已更新

### 时区配置无效
- 确认时区格式正确（如 "Asia/Shanghai"）
- 检查 `SendUpdateConfig` 是否发送了 timezone 字段
- 查看服务端是否支持时区配置

## 下一步

- 查看 [CHANGELOG.md](CHANGELOG.md) 了解详细变更
- 阅读 [CONTROL_MODES.md](CONTROL_MODES.md) 了解控制模式
- 参考前端实现：`le-bot-frontend/src/pages/stack/ChatPage.vue`
