package control

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"time"

	"websocket_client_chat/internal/config"
)

// Command 控制命令类型
type Command string

const (
	CmdStartRecording Command = "1" // 开始录音
	CmdStopRecording  Command = "2" // 停止录音
)

// Handler 控制命令处理器接口
type Handler interface {
	HandleCommand(cmd Command)
}

// FileMonitor 文件监控器
type FileMonitor struct {
	config  *config.ControlConfig
	handler Handler

	ctx    context.Context
	cancel context.CancelFunc
}

// NewFileMonitor 创建新的文件监控器
func NewFileMonitor(cfg *config.ControlConfig, handler Handler) *FileMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &FileMonitor{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动文件监控
func (fm *FileMonitor) Start() error {
	// 初始化控制文件
	if err := fm.initControlFile(); err != nil {
		return err
	}

	go fm.monitorLoop()
	return nil
}

// Stop 停止文件监控
func (fm *FileMonitor) Stop() error {
	fm.cancel()
	return nil
}

// initControlFile 初始化控制文件
func (fm *FileMonitor) initControlFile() error {
	return ioutil.WriteFile(fm.config.FilePath, []byte{}, 0644)
}

// monitorLoop 监控循环
func (fm *FileMonitor) monitorLoop() {
	ticker := time.NewTicker(fm.config.MonitorDelay)
	defer ticker.Stop()

	var lastCmd string

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			if err := fm.checkFile(&lastCmd); err != nil {
				log.Printf("检查控制文件失败: %v", err)
			}
		}
	}
}

// checkFile 检查文件内容
func (fm *FileMonitor) checkFile(lastCmd *string) error {
	content, err := ioutil.ReadFile(fm.config.FilePath)
	if err != nil {
		return err
	}

	currentValue := string(bytes.TrimSpace(content))
	if currentValue == "" || currentValue == *lastCmd {
		return nil
	}

	*lastCmd = currentValue
	log.Printf("检测到命令: %s", currentValue)

	// 处理命令
	cmd := Command(currentValue)
	fm.handler.HandleCommand(cmd)

	// 清空控制文件
	if err := ioutil.WriteFile(fm.config.FilePath, []byte{}, 0644); err != nil {
		log.Printf("清空控制文件失败: %v", err)
	}

	return nil
}
