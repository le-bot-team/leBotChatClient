package control

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"websocket_client_chat/internal/config"
)

// GpioHandler is the GPIO event handler interface
type GpioHandler interface {
	OnGpioWake()
}

// GpioMonitor monitors a GPIO pin via sysfs for wake events
type GpioMonitor struct {
	config  *config.GpioConfig
	handler GpioHandler

	ctx    context.Context
	cancel context.CancelFunc
}

// NewGpioMonitor creates a new GPIO monitor
func NewGpioMonitor(parentCtx context.Context, cfg *config.GpioConfig, handler GpioHandler) *GpioMonitor {
	ctx, cancel := context.WithCancel(parentCtx)

	return &GpioMonitor{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start initializes the GPIO pin and starts monitoring
func (gm *GpioMonitor) Start() error {
	if err := gm.initGpio(); err != nil {
		return fmt.Errorf("failed to initialize GPIO %d: %w", gm.config.PinNumber, err)
	}

	go gm.monitorLoop()
	log.Printf("GPIO monitor started on pin %d (poll interval: %v)", gm.config.PinNumber, gm.config.PollInterval)
	return nil
}

// Stop stops the GPIO monitor
func (gm *GpioMonitor) Stop() error {
	gm.cancel()
	return nil
}

// initGpio exports the GPIO pin and sets direction to input
func (gm *GpioMonitor) initGpio() error {
	pinStr := fmt.Sprintf("%d", gm.config.PinNumber)
	gpioDir := fmt.Sprintf("/sys/class/gpio/gpio%d", gm.config.PinNumber)

	// Check if already exported
	if _, err := os.Stat(gpioDir); os.IsNotExist(err) {
		// Export the GPIO pin
		if err := os.WriteFile("/sys/class/gpio/export", []byte(pinStr), 0644); err != nil {
			return fmt.Errorf("failed to export GPIO %d: %w", gm.config.PinNumber, err)
		}
		// Give sysfs a moment to create the directory
		time.Sleep(50 * time.Millisecond)
	}

	// Set direction to input
	directionPath := fmt.Sprintf("%s/direction", gpioDir)
	if err := os.WriteFile(directionPath, []byte("in"), 0644); err != nil {
		return fmt.Errorf("failed to set GPIO %d direction: %w", gm.config.PinNumber, err)
	}

	return nil
}

// readGpioValue reads the current GPIO pin value (0 or 1)
func (gm *GpioMonitor) readGpioValue() (int, error) {
	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gm.config.PinNumber)
	data, err := os.ReadFile(valuePath)
	if err != nil {
		return -1, err
	}

	val := strings.TrimSpace(string(data))
	if val == "0" {
		return 0, nil
	}
	return 1, nil
}

// monitorLoop polls the GPIO pin for falling edge (high -> low transition)
func (gm *GpioMonitor) monitorLoop() {
	ticker := time.NewTicker(gm.config.PollInterval)
	defer ticker.Stop()

	// Read initial state
	prevState, err := gm.readGpioValue()
	if err != nil {
		log.Printf("Failed to read initial GPIO state: %v", err)
		prevState = 1 // Assume high (not pressed)
	}

	for {
		select {
		case <-gm.ctx.Done():
			return
		case <-ticker.C:
			currentState, err := gm.readGpioValue()
			if err != nil {
				log.Printf("Failed to read GPIO value: %v", err)
				continue
			}

			// Detect falling edge: high (1) -> low (0)
			if prevState == 1 && currentState == 0 {
				log.Println("GPIO wake trigger detected (falling edge)")
				gm.handler.OnGpioWake()
			}

			prevState = currentState
		}
	}
}
