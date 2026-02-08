package control

import (
	"bytes"
	"context"
	"log"
	"os"
	"time"

	"websocket_client_chat/internal/config"
)

// Command is the control command type
type Command string

const (
	CmdStartRecording Command = "1" // Start recording
	CmdStopRecording  Command = "2" // Stop recording
	CmdTestRecording  Command = "3" // Test recording
	CmdQuit           Command = "q" // Quit program
)

// Handler is the control command handler interface
type Handler interface {
	HandleCommand(cmd Command)
}

// FileMonitor is the file monitor
type FileMonitor struct {
	config  *config.ControlConfig
	handler Handler

	ctx    context.Context
	cancel context.CancelFunc
}

// NewFileMonitor creates a new file monitor
func NewFileMonitor(parentCtx context.Context, cfg *config.ControlConfig, handler Handler) *FileMonitor {
	ctx, cancel := context.WithCancel(parentCtx)

	return &FileMonitor{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts file monitoring
func (fm *FileMonitor) Start() error {
	// Initialize control file
	if err := fm.initControlFile(); err != nil {
		return err
	}

	go fm.monitorLoop()
	return nil
}

// Stop stops file monitoring
func (fm *FileMonitor) Stop() error {
	fm.cancel()
	return nil
}

// initControlFile initializes the control file
func (fm *FileMonitor) initControlFile() error {
	return os.WriteFile(fm.config.FilePath, []byte{}, 0644)
}

// monitorLoop is the monitoring loop
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
				log.Printf("Failed to check control file: %v", err)
			}
		}
	}
}

// checkFile checks file content
func (fm *FileMonitor) checkFile(lastCmd *string) error {
	content, err := os.ReadFile(fm.config.FilePath)
	if err != nil {
		return err
	}

	currentValue := string(bytes.TrimSpace(content))
	if currentValue == "" || currentValue == *lastCmd {
		return nil
	}

	*lastCmd = currentValue
	log.Printf("Command detected: %s", currentValue)

	// Process command
	cmd := Command(currentValue)
	fm.handler.HandleCommand(cmd)

	// Clear control file
	if err := os.WriteFile(fm.config.FilePath, []byte{}, 0644); err != nil {
		log.Printf("Failed to clear control file: %v", err)
	}

	return nil
}
