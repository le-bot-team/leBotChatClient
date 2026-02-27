package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"websocket_client_chat/internal/audio"
	"websocket_client_chat/internal/config"
	"websocket_client_chat/internal/control"
	"websocket_client_chat/internal/websocket"
	"websocket_client_chat/pkg/utils"
)

// AppState represents the program state in GPIO mode
type AppState int32

const (
	StateSleeping        AppState = 0 // Default: buffering audio to circular wake buffer
	StateWaitingResponse AppState = 1 // Waiting for wake response audio to finish playing
	StateActive          AppState = 2 // Actively streaming audio to backend (after wake response)
)

// App is the main application structure
type App struct {
	config *config.Config

	// Components
	recorder     *audio.Recorder
	player       *audio.Player
	wsClient     *websocket.Client
	fileMonitor  *control.FileMonitor
	stdinMonitor *control.StdinMonitor
	gpioMonitor  *control.GpioMonitor

	// State management
	updateCh    chan struct{} // Signals config update response received
	enableDebug bool          // Debug mode switch
	controlMode string        // Control mode: "stdin", "file", or "gpio"

	// GPIO mode state
	state              atomic.Int32 // Current state (StateSleeping, StateWaitingResponse, or StateActive)
	currentRequestID   string       // Active session request ID
	requestIDMutex     sync.RWMutex
	wakeBuffer         []int16    // Circular buffer for wake audio
	wakeBufferMutex    sync.Mutex // Protects wakeBuffer
	wakeBufferMaxSize  int        // Max samples in wake buffer
	silenceBuffer      []int16    // Buffer for silence detection
	silenceBufferMutex sync.Mutex // Protects silenceBuffer
	silenceBufferSize  int        // Max samples in silence buffer

	// Context control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewApp creates a new application instance
func NewApp(controlMode string) *App {
	cfg := config.DefaultConfig()
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:      cfg,
		ctx:         ctx,
		cancel:      cancel,
		updateCh:    make(chan struct{}, 1),
		enableDebug: cfg.EnableDebug,
		controlMode: controlMode,
	}

	// Initialize GPIO mode buffers
	if controlMode == "gpio" {
		// Wake buffer: bufferDuration seconds of audio at output sample rate
		app.wakeBufferMaxSize = int(cfg.Wake.BufferDuration.Seconds()) * cfg.Audio.SampleRate
		app.wakeBuffer = make([]int16, 0, app.wakeBufferMaxSize)

		// Silence buffer: configured seconds of audio at output sample rate
		app.silenceBufferSize = cfg.Wake.SilenceBufferSeconds * cfg.Audio.SampleRate
		app.silenceBuffer = make([]int16, 0, app.silenceBufferSize)
	}

	// Initialize components
	app.recorder = audio.NewRecorder(&cfg.Audio, app, cfg.EnableDebug)
	app.player = audio.NewPlayer(ctx, &cfg.Audio, cfg.EnableDebug)
	app.wsClient = websocket.NewClient(ctx, &cfg.WebSocket, app, cfg.EnableDebug)

	// Select control mode based on command-line argument
	switch controlMode {
	case "gpio":
		app.gpioMonitor = control.NewGpioMonitor(ctx, &cfg.Gpio, app)
	case "stdin":
		app.stdinMonitor = control.NewStdinMonitor(ctx, app)
	default:
		app.fileMonitor = control.NewFileMonitor(ctx, &cfg.Control, app)
	}

	return app
}

// Start starts the application
func (app *App) Start() error {
	// Initialize PortAudio
	if err := app.recorder.Initialize(); err != nil {
		return err
	}

	// Start WebSocket client
	if err := app.wsClient.Start(); err != nil {
		return err
	}

	// Start the corresponding control mode
	switch app.controlMode {
	case "gpio":
		// GPIO mode: start continuous recording immediately, then GPIO monitor
		if err := app.gpioMonitor.Start(); err != nil {
			return fmt.Errorf("failed to start GPIO monitor: %w", err)
		}

		// Start continuous recording (state defaults to Sleeping)
		app.state.Store(int32(StateSleeping))
		if err := app.recorder.StartRecording("gpio-continuous"); err != nil {
			return fmt.Errorf("failed to start continuous recording: %w", err)
		}

		// Start silence detection loop
		app.wg.Add(1)
		go app.silenceCheckLoop()

		log.Println("Voice intercom system started successfully (GPIO mode)")
		log.Println("State: SLEEPING - buffering audio to wake buffer")
		log.Printf("Wake buffer: %.1f seconds, Silence check: every %v",
			app.config.Wake.BufferDuration.Seconds(), app.config.Wake.SilenceCheckInterval)

	case "stdin":
		if err := app.stdinMonitor.Start(); err != nil {
			return err
		}
		log.Println("Voice intercom system started successfully (stdin control mode)")
		log.Println("Enter commands:")
		log.Println("  1 or start - start recording")
		log.Println("  2 or stop  - stop recording and send")
		log.Println("  q or quit  - exit program")

	default:
		if err := app.fileMonitor.Start(); err != nil {
			return err
		}
		log.Println("Voice intercom system started successfully (file control mode)")
		log.Println("Usage:")
		log.Println("Write to /tmp/chat-control:")
		log.Println("  1 - start recording")
		log.Println("  2 - stop recording and send")
	}

	return nil
}

// Stop stops the application
func (app *App) Stop() error {
	app.cancel()

	// Stop components
	if app.gpioMonitor != nil {
		if err := app.gpioMonitor.Stop(); err != nil {
			log.Printf("Failed to stop GPIO monitor: %v", err)
		}
	}

	if app.fileMonitor != nil {
		if err := app.fileMonitor.Stop(); err != nil {
			log.Printf("Failed to stop file monitor: %v", err)
		}
	}

	if app.stdinMonitor != nil {
		if err := app.stdinMonitor.Stop(); err != nil {
			log.Printf("Failed to stop stdin monitor: %v", err)
		}
	}

	if err := app.wsClient.Stop(); err != nil {
		log.Printf("Failed to stop WebSocket client: %v", err)
	}

	if err := app.player.Stop(); err != nil {
		log.Printf("Failed to stop audio player: %v", err)
	}

	if err := app.recorder.Terminate(); err != nil {
		log.Printf("Failed to terminate audio recorder: %v", err)
	}

	// Wait for all goroutines to finish
	app.wg.Wait()

	log.Println("System exited safely")
	return nil
}

// Wait waits for the application to finish
func (app *App) Wait() {
	<-app.ctx.Done()
}

// === Implementation of control.Handler interface ===

// HandleCommand handles control commands (for stdin/file modes)
func (app *App) HandleCommand(cmd control.Command) {
	switch cmd {
	case control.CmdStartRecording:
		if !app.recorder.IsRecording() {
			requestID := utils.GenerateRequestID(app.config.Device.SerialNumber)

			// Send config update request and wait for response
			app.wg.Add(1)
			go func() {
				defer app.wg.Done()
				app.sendUpdateConfigAndWait(requestID)

				// Start recording after config update succeeds
				if err := app.recorder.StartRecording(requestID); err != nil {
					log.Printf("Failed to start recording: %v", err)
				}
			}()
		} else {
			log.Println("System busy, ignoring start recording command")
		}

	case control.CmdStopRecording:
		if app.recorder.IsRecording() {
			if err := app.recorder.StopRecording(); err != nil {
				log.Printf("Failed to stop recording: %v", err)
			}
		} else {
			log.Println("Not in recording state, ignoring stop command")
		}

	case control.CmdTestRecording:
		if app.recorder.IsRecording() {
			log.Println("Currently recording, cannot start test recording")
			return
		}

		// Execute test recording asynchronously
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()

			// Generate filename with timestamp
			filename := fmt.Sprintf("test_recording_%s.wav", time.Now().Format("20060102_150405"))

			// Record for 5 seconds
			if err := app.recorder.TestRecording(5, filename); err != nil {
				log.Printf("Test recording failed: %v", err)
			}
		}()

	case control.CmdQuit:
		log.Println("Quit command received, shutting down...")
		app.cancel()
	}
}

// === Implementation of control.GpioHandler interface ===

// OnGpioWake is called when a falling edge is detected on the GPIO pin
func (app *App) OnGpioWake() {
	currentState := AppState(app.state.Load())

	// Check WebSocket connection
	if !app.wsClient.IsConnected() {
		log.Println("GPIO wake ignored: WebSocket not connected")
		return
	}

	// If not in sleeping state, interrupt current session first
	if currentState != StateSleeping {
		log.Printf("GPIO wake during state %d, interrupting current session", currentState)
		app.interruptCurrentSession()
	}

	log.Println("GPIO wake triggered, transitioning to WAITING_RESPONSE state")

	// Generate a new request ID for this session
	requestID := utils.GenerateRequestID(app.config.Device.SerialNumber)
	app.requestIDMutex.Lock()
	app.currentRequestID = requestID
	app.requestIDMutex.Unlock()

	// Get wake buffer contents and convert to WAV
	app.wakeBufferMutex.Lock()
	wakeAudio := make([]int16, len(app.wakeBuffer))
	copy(wakeAudio, app.wakeBuffer)
	app.wakeBufferMutex.Unlock()

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		// Send config update and wait for acknowledgment
		app.sendUpdateConfigAndWait(requestID)

		// Send wake audio buffer to backend
		if len(wakeAudio) > 0 {
			wavData := app.recorder.ConvertToWAV(wakeAudio)
			if err := app.wsClient.SendWakeAudio(requestID, wavData); err != nil {
				log.Printf("Failed to send wake audio: %v", err)
				return
			}
			if app.enableDebug {
				log.Printf("Sent wake audio: %d samples (%.2f seconds)",
					len(wakeAudio), float64(len(wakeAudio))/float64(app.config.Audio.SampleRate))
			}
		} else {
			log.Println("Wake buffer empty, sending wake audio with no data")
			if err := app.wsClient.SendWakeAudio(requestID, nil); err != nil {
				log.Printf("Failed to send empty wake audio: %v", err)
				return
			}
		}

		// Switch to waiting response state (NOT active yet)
		// Audio will be buffered but silence detection won't trigger until playback completes
		app.state.Store(int32(StateWaitingResponse))
		log.Println("State: WAITING_RESPONSE - waiting for wake response audio")
	}()
}

// interruptCurrentSession stops playback and clears buffers for a new session
func (app *App) interruptCurrentSession() {
	// Stop audio playback immediately
	if app.player.IsPlaying() {
		log.Println("Interrupting audio playback")
		app.player.StopPlayback()
	}

	// Clear silence buffer
	app.silenceBufferMutex.Lock()
	app.silenceBuffer = app.silenceBuffer[:0]
	app.silenceBufferMutex.Unlock()

	// Clear wake buffer to start fresh
	app.wakeBufferMutex.Lock()
	app.wakeBuffer = app.wakeBuffer[:0]
	app.wakeBufferMutex.Unlock()

	// Send cancel output to backend to stop any ongoing processing
	app.requestIDMutex.RLock()
	reqID := app.currentRequestID
	app.requestIDMutex.RUnlock()

	if reqID != "" {
		if err := app.wsClient.SendCancelOutput(reqID); err != nil {
			log.Printf("Failed to send cancel output: %v", err)
		} else if app.enableDebug {
			log.Println("Sent cancel output to backend")
		}
	}
}

// === Implementation of audio.Handler interface ===

// OnAudioChunk handles audio chunks from the recorder
func (app *App) OnAudioChunk(requestID string, samples []int16, isLast bool) {
	if app.controlMode == "gpio" {
		app.handleGpioAudioChunk(samples)
		return
	}

	// Original stdin/file mode behavior
	wavData := app.recorder.ConvertToWAV(samples)

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		var err error
		if isLast {
			err = app.wsClient.SendAudioComplete(requestID, wavData)
			if err == nil && app.enableDebug {
				log.Printf("Sent completion request (including last %d bytes of WAV audio)", len(wavData))
			}
		} else {
			err = app.wsClient.SendAudioStream(requestID, wavData)
		}

		if err != nil {
			log.Printf("Failed to send audio data: %v", err)
		}
	}()
}

// handleGpioAudioChunk routes audio based on the current GPIO mode state
func (app *App) handleGpioAudioChunk(samples []int16) {
	state := AppState(app.state.Load())

	switch state {
	case StateSleeping:
		app.appendToWakeBuffer(samples)

	case StateWaitingResponse:
		// While waiting for response, still send audio to backend but don't check for silence
		app.requestIDMutex.RLock()
		reqID := app.currentRequestID
		app.requestIDMutex.RUnlock()

		wavData := app.recorder.ConvertToWAV(samples)

		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			if err := app.wsClient.SendAudioStream(reqID, wavData); err != nil {
				log.Printf("Failed to send audio stream: %v", err)
			}
		}()

	case StateActive:
		// Append to silence detection buffer
		app.appendToSilenceBuffer(samples)

		// Convert to WAV and send to backend
		app.requestIDMutex.RLock()
		reqID := app.currentRequestID
		app.requestIDMutex.RUnlock()

		wavData := app.recorder.ConvertToWAV(samples)

		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			if err := app.wsClient.SendAudioStream(reqID, wavData); err != nil {
				log.Printf("Failed to send audio stream: %v", err)
			}
		}()
	}
}

// appendToWakeBuffer appends samples to the circular wake buffer, trimming old data
func (app *App) appendToWakeBuffer(samples []int16) {
	app.wakeBufferMutex.Lock()
	app.wakeBuffer = append(app.wakeBuffer, samples...)
	if len(app.wakeBuffer) > app.wakeBufferMaxSize {
		excess := len(app.wakeBuffer) - app.wakeBufferMaxSize
		app.wakeBuffer = app.wakeBuffer[excess:]
	}
	app.wakeBufferMutex.Unlock()
}

// appendToSilenceBuffer appends samples to the silence detection buffer, trimming old data
func (app *App) appendToSilenceBuffer(samples []int16) {
	app.silenceBufferMutex.Lock()
	app.silenceBuffer = append(app.silenceBuffer, samples...)
	if len(app.silenceBuffer) > app.silenceBufferSize {
		excess := len(app.silenceBuffer) - app.silenceBufferSize
		app.silenceBuffer = app.silenceBuffer[excess:]
	}
	app.silenceBufferMutex.Unlock()
}

// silenceCheckLoop periodically checks for silence in active state to trigger sleep transition
func (app *App) silenceCheckLoop() {
	defer app.wg.Done()

	ticker := time.NewTicker(app.config.Wake.SilenceCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			if AppState(app.state.Load()) != StateActive {
				continue
			}

			// Copy the silence buffer for checking
			app.silenceBufferMutex.Lock()
			bufLen := len(app.silenceBuffer)
			bufferCopy := make([]int16, bufLen)
			copy(bufferCopy, app.silenceBuffer)
			app.silenceBufferMutex.Unlock()

			// Need at least half the buffer filled before checking
			if bufLen < app.silenceBufferSize/2 {
				continue
			}

			// Check if the entire buffer is silent
			isSilent := utils.IsSilent(
				bufferCopy,
				app.config.Wake.SilenceThresholdRMS,
				app.config.Wake.SilenceRatio,
			)

			if app.enableDebug {
				rms := utils.CalculateRMS(bufferCopy)
				log.Printf("Silence check: RMS=%.2f, threshold=%.2f, silent=%v, samples=%d",
					rms, app.config.Wake.SilenceThresholdRMS, isSilent, bufLen)
			}

			if isSilent {
				app.transitionToSleeping()
			}
		}
	}
}

// transitionToSleeping sends inputAudioComplete and switches state back to sleeping
func (app *App) transitionToSleeping() {
	log.Println("Silence detected, transitioning to SLEEPING state")

	app.requestIDMutex.RLock()
	reqID := app.currentRequestID
	app.requestIDMutex.RUnlock()

	// Send inputAudioComplete to signal end of this session's audio
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		if err := app.wsClient.SendAudioComplete(reqID, nil); err != nil {
			log.Printf("Failed to send audio complete: %v", err)
		} else if app.enableDebug {
			log.Println("Sent audio complete signal")
		}
	}()

	// Clear silence buffer
	app.silenceBufferMutex.Lock()
	app.silenceBuffer = app.silenceBuffer[:0]
	app.silenceBufferMutex.Unlock()

	// Clear wake buffer to start fresh
	app.wakeBufferMutex.Lock()
	app.wakeBuffer = app.wakeBuffer[:0]
	app.wakeBufferMutex.Unlock()

	// Switch state
	app.state.Store(int32(StateSleeping))
	log.Println("State: SLEEPING - buffering audio to wake buffer")
}

// OnRecordingComplete handles recording completion (for stdin/file modes)
func (app *App) OnRecordingComplete(requestID string, _ []int16) {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		if err := app.wsClient.SendAudioComplete(requestID, nil); err != nil {
			log.Printf("Failed to send completion notification: %v", err)
		} else if app.enableDebug {
			log.Println("Sent completion request (no remaining audio)")
		}
	}()
}

// === Implementation of websocket.MessageHandler interface ===

// HandleOutputAudioStream handles output audio stream
func (app *App) HandleOutputAudioStream(resp *websocket.OutputAudioStreamResponse) {
	if app.enableDebug {
		log.Printf("Received audio stream response: ID=%s, ConversationID=%s, ChatID=%s",
			resp.ID, resp.Data.ConversationID, resp.Data.ChatID)
	}

	audioData, err := base64.StdEncoding.DecodeString(resp.Data.Buffer)
	if err != nil {
		log.Printf("Audio decoding failed: %v", err)
		return
	}

	if app.enableDebug {
		log.Printf("Audio data size: %d bytes", len(audioData))
	}

	// Write to playback buffer
	app.player.WriteAudioData(audioData)
}

// HandleOutputAudioComplete handles output audio completion
func (app *App) HandleOutputAudioComplete(resp *websocket.OutputAudioCompleteResponse) {
	if app.enableDebug {
		log.Printf("Audio output complete: ConversationID=%s, ChatID=%s",
			resp.Data.ConversationID, resp.Data.ChatID)
	}
	app.player.SetAudioComplete(true)

	// In GPIO mode, transition from WaitingResponse to Active after audio playback completes
	if app.controlMode == "gpio" && AppState(app.state.Load()) == StateWaitingResponse {
		// Clear silence buffer before starting silence detection
		app.silenceBufferMutex.Lock()
		app.silenceBuffer = app.silenceBuffer[:0]
		app.silenceBufferMutex.Unlock()

		app.state.Store(int32(StateActive))
		log.Println("State: ACTIVE - now listening for user input")
	}
}

// HandleOutputTextStream handles output text stream
func (app *App) HandleOutputTextStream(resp *websocket.OutputTextStreamResponse) {
	if app.enableDebug && len(resp.Data.Text) > 0 {
		log.Printf("Received valid text stream: ID=%s, Role=%s, Text=%s",
			resp.Data.ChatID, resp.Data.Role, resp.Data.Text)
	}

	// If it's a user message with text length >= 2, a new user message was sent; execute interruption logic
	if resp.Data.Role == "user" && len(resp.Data.Text) >= 2 {
		if app.player.IsPlaying() {
			if app.enableDebug {
				log.Println("New user message detected, executing interruption logic")
			}
			app.player.StopPlayback()
		}
	}
}

// HandleOutputTextComplete handles output text completion
func (app *App) HandleOutputTextComplete(resp *websocket.OutputTextCompleteResponse) {
	if app.enableDebug {
		log.Printf("Text output complete: ID=%s, Role=%s, Text=%s",
			resp.Data.ChatID, resp.Data.Role, resp.Data.Text)
	}
}

// HandleChatComplete handles chat completion
func (app *App) HandleChatComplete(resp *websocket.ChatCompleteResponse) {
	if app.enableDebug {
		log.Printf("Chat complete: ID=%s, Success=%v, Message=%s",
			resp.Data.ChatID, resp.Success, resp.Message)
	}

	if !resp.Success {
		for _, err := range resp.Data.Errors {
			log.Printf("Error [%d]: %s", err.Code, err.Message)
		}
	}
}

// HandleCancelOutput handles cancel output from server (voice interrupt)
func (app *App) HandleCancelOutput(resp *websocket.CancelOutputResponse) {
	log.Printf("[App] Received cancelOutput from server (type: %s), stopping playback", resp.Data.CancelType)

	// Stop audio playback immediately
	if app.player.IsPlaying() {
		app.player.StopPlayback()
	}

	// Clear audio buffer to prevent any remaining data from playing
	app.player.ClearBuffer()
}

// HandleUpdateConfig handles update config response
func (app *App) HandleUpdateConfig(resp *websocket.UpdateConfigResponse) {
	if app.enableDebug {
		log.Printf("Received config update response: Success=%v, Message=%s", resp.Success, resp.Message)
	}
	// Non-blocking send; if nobody is waiting, the signal is dropped.
	select {
	case app.updateCh <- struct{}{}:
	default:
	}
}

// sendUpdateConfigAndWait sends a config update request and waits for response
func (app *App) sendUpdateConfigAndWait(requestID string) {
	if err := app.wsClient.SendUpdateConfig(requestID, &app.config.Device); err != nil {
		log.Printf("Failed to send config update: %v", err)
		return
	}

	if app.enableDebug {
		log.Println("Update request sent")
	}

	// Wait for update response or context cancellation
	select {
	case <-app.updateCh:
		// Response received
	case <-app.ctx.Done():
		return
	}

	if app.enableDebug {
		log.Println("Update response successful, starting streaming audio transmission")
	}
}
