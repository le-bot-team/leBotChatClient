package control

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"websocket_client_chat/internal/config"
)

// StdinMonitor 标准输入监控器（用于调试）
type StdinMonitor struct {
	config  *config.ControlConfig
	handler Handler

	ctx    context.Context
	cancel context.CancelFunc
}

// NewStdinMonitor 创建新的标准输入监控器
func NewStdinMonitor(cfg *config.ControlConfig, handler Handler) *StdinMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &StdinMonitor{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动标准输入监控
func (sm *StdinMonitor) Start() error {
	go sm.monitorLoop()
	return nil
}

// Stop 停止标准输入监控
func (sm *StdinMonitor) Stop() error {
	sm.cancel()
	return nil
}

// monitorLoop 监控循环
func (sm *StdinMonitor) monitorLoop() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== 调试控制台 ===")
	fmt.Println("输入命令:")
	fmt.Println("  1 或 start - 开始录音")
	fmt.Println("  2 或 stop  - 停止录音并发送")
	fmt.Println("  3 或 test  - 测试录音(录制5秒并保存到文件)")
	fmt.Println("  q 或 quit  - 退出程序")
	fmt.Println("==================")

	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
			fmt.Print("> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("读取输入失败: %v", err)
				continue
			}

			// 去除首尾空白字符
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			// 处理命令
			sm.processCommand(input)
		}
	}
}

// processCommand 处理命令
func (sm *StdinMonitor) processCommand(input string) {
	input = strings.ToLower(input)

	var cmd Command
	switch input {
	case "1", "start":
		cmd = CmdStartRecording
		log.Println("命令: 开始录音")
	case "2", "stop":
		cmd = CmdStopRecording
		log.Println("命令: 停止录音")
	case "3", "test":
		cmd = CmdTestRecording
		log.Println("命令: 测试录音(将录制5秒并保存)")
	case "q", "quit", "exit":
		log.Println("命令: 退出程序")
		// 触发程序退出
		// 这里通过调用 cancel 来触发上下文取消，从而让主程序退出
		sm.cancel()
		return
	default:
		fmt.Printf("未知命令: %s\n", input)
		return
	}

	// 调用处理器
	sm.handler.HandleCommand(cmd)
}
