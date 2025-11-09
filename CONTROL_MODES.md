# Control Modes

This application supports two control modes for recording audio:

## 1. Standard Input Mode (Debug Mode) - **Default**

This mode allows you to control recording by typing commands directly in the terminal. It's more convenient for development and debugging.

### Usage

The application starts in this mode by default. Simply run the application and you'll see:

```
语音对讲系统启动成功 (标准输入控制模式)
输入命令:
  1 或 start - 开始录音
  2 或 stop  - 停止录音并发送
  q 或 quit  - 退出程序
```

### Commands

- `1` or `start` - Start recording
- `2` or `stop` - Stop recording and send audio
- `q` or `quit` or `exit` - Exit the program

### Configuration

To enable this mode, set `UseStdin` to `true` in the Control configuration:

```go
Control: ControlConfig{
    FilePath:      "/tmp/chatctrl",
    MonitorDelay:  100 * time.Millisecond,
    ChannelBuffer: 1,
    UseStdin:      true,  // Enable stdin control mode
}
```

## 2. File Control Mode

This mode monitors a control file (`/tmp/chatctrl` by default) for commands. It's useful for automation or when running as a service.

### Usage

To use this mode, set `UseStdin` to `false` in the configuration.

When the application starts, you'll see:

```
语音对讲系统启动成功 (文件控制模式)
使用说明:
向/tmp/chatctrl写入:
  1 - 开始录音
  2 - 停止录音并发送
```

### Commands

Write to the control file to send commands:

```bash
# Start recording
echo "1" > /tmp/chatctrl

# Stop recording
echo "2" > /tmp/chatctrl
```

### Configuration

To enable this mode, set `UseStdin` to `false` in the Control configuration:

```go
Control: ControlConfig{
    FilePath:      "/tmp/chatctrl",
    MonitorDelay:  100 * time.Millisecond,
    ChannelBuffer: 1,
    UseStdin:      false,  // Enable file control mode
}
```

## Switching Between Modes

To switch between modes, modify the `internal/config/config.go` file and change the `UseStdin` field in the `DefaultConfig()` function:

```go
Control: ControlConfig{
    FilePath:      "/tmp/chatctrl",
    MonitorDelay:  100 * time.Millisecond,
    ChannelBuffer: 1,
    UseStdin:      true,  // true for stdin mode, false for file mode
}
```

Then rebuild the application:

```bash
go build -o bin/chatclient ./cmd/
```

