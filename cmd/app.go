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

// App is the main application structure
type App struct {
	config *config.Config

	// Components
	recorder     *audio.Recorder
	player       *audio.Player
	wsClient     *websocket.Client
	fileMonitor  *control.FileMonitor
	stdinMonitor *control.StdinMonitor

	// State management
	updateFlag  int32 // Update response flag
	enableDebug bool  // Debug mode switch

	// Context control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewApp creates a new application instance
func NewApp() *App {
	cfg := config.DefaultConfig()
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:      cfg,
		ctx:         ctx,
		cancel:      cancel,
		enableDebug: cfg.EnableDebug,
	}

	// Initialize components
	app.recorder = audio.NewRecorder(&cfg.Audio, app, cfg.EnableDebug)
	app.player = audio.NewPlayer(&cfg.Audio, cfg.EnableDebug)
	app.wsClient = websocket.NewClient(&cfg.WebSocket, app, cfg.EnableDebug)

	// Select control mode based on configuration
	if cfg.Control.UseStdin {
		app.stdinMonitor = control.NewStdinMonitor(&cfg.Control, app)
	} else {
		app.fileMonitor = control.NewFileMonitor(&cfg.Control, app)
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

	// Start the corresponding control monitor
	if app.config.Control.UseStdin {
		if err := app.stdinMonitor.Start(); err != nil {
			return err
		}
		log.Println("Voice intercom system started successfully (stdin control mode)")
		log.Println("Enter commands:")
		log.Println("  1 or start - start recording")
		log.Println("  2 or stop  - stop recording and send")
		log.Println("  q or quit  - exit program")
	} else {
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

// HandleCommand handles control commands
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
	}
}

// === Implementation of audio.AudioHandler interface ===

// OnAudioChunk handles audio chunks
func (app *App) OnAudioChunk(requestID string, samples []int16, isLast bool) {
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

// OnRecordingComplete handles recording completion
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
			resp.ID, resp.Data.ConversationId, resp.Data.ChatId)
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
			resp.Data.ConversationId, resp.Data.ChatId)
	}
	app.player.SetAudioComplete(true)
}

// HandleOutputTextStream handles output text stream
func (app *App) HandleOutputTextStream(resp *websocket.OutputTextStreamResponse) {
	if app.enableDebug && len(resp.Data.Text) > 0 {
		log.Printf("Received valid text stream: ID=%s, Role=%s, Text=%s",
			resp.Data.ChatId, resp.Data.Role, resp.Data.Text)
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
			resp.Data.ChatId, resp.Data.Role, resp.Data.Text)
	}
}

// HandleChatComplete handles chat completion
func (app *App) HandleChatComplete(resp *websocket.ChatCompleteResponse) {
	if app.enableDebug {
		log.Printf("Chat complete: ID=%s, Success=%v, Message=%s",
			resp.Data.ChatId, resp.Success, resp.Message)
	}

	if !resp.Success {
		for _, err := range resp.Data.Errors {
			log.Printf("Error [%d]: %s", err.Code, err.Message)
		}
	}
}

// HandleUpdateConfig handles update config response
func (app *App) HandleUpdateConfig(resp *websocket.UpdateConfigResponse) {
	if app.enableDebug {
		log.Printf("Received config update response: Success=%v, Message=%s", resp.Success, resp.Message)
	}
	atomic.StoreInt32(&app.updateFlag, 1)
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

	// Wait for flag update
	for atomic.LoadInt32(&app.updateFlag) == 0 {
		select {
		case <-app.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Continue waiting
		}
	}

	atomic.StoreInt32(&app.updateFlag, 0)
	if app.enableDebug {
		log.Println("Update response successful, starting streaming audio transmission")
	}
}
