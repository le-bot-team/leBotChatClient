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

// StdinMonitor is the stdin monitor (for debugging)
type StdinMonitor struct {
	config  *config.ControlConfig
	handler Handler

	ctx    context.Context
	cancel context.CancelFunc
}

// NewStdinMonitor creates a new stdin monitor
func NewStdinMonitor(parentCtx context.Context, cfg *config.ControlConfig, handler Handler) *StdinMonitor {
	ctx, cancel := context.WithCancel(parentCtx)

	return &StdinMonitor{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts stdin monitoring
func (sm *StdinMonitor) Start() error {
	go sm.monitorLoop()
	return nil
}

// Stop stops stdin monitoring
func (sm *StdinMonitor) Stop() error {
	sm.cancel()
	return nil
}

// monitorLoop is the monitoring loop
func (sm *StdinMonitor) monitorLoop() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== Debug Console ===")
	fmt.Println("Enter command:")
	fmt.Println("  1 or start - Start recording")
	fmt.Println("  2 or stop  - Stop recording and send")
	fmt.Println("  3 or test  - Test recording (record 5s and save to file)")
	fmt.Println("  q or quit  - Exit program")
	fmt.Println("==================")

	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
			fmt.Print("> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("Failed to read input: %v", err)
				continue
			}

			// Trim leading and trailing whitespace
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			// Process command
			sm.processCommand(input)
		}
	}
}

// processCommand processes a command
func (sm *StdinMonitor) processCommand(input string) {
	input = strings.ToLower(input)

	var cmd Command
	switch input {
	case "1", "start":
		cmd = CmdStartRecording
		log.Println("Command: Start recording")
	case "2", "stop":
		cmd = CmdStopRecording
		log.Println("Command: Stop recording")
	case "3", "test":
		cmd = CmdTestRecording
		log.Println("Command: Test recording (will record 5s and save)")
	case "q", "quit", "exit":
		log.Println("Command: Exit program")
		sm.handler.HandleCommand(CmdQuit)
		return
	default:
		fmt.Printf("Unknown command: %s\n", input)
		return
	}

	// Call handler
	sm.handler.HandleCommand(cmd)
}
