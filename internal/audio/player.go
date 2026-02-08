package audio

import (
	"context"
	"log"
	"sync"
	"time"

	"websocket_client_chat/internal/config"
	"websocket_client_chat/pkg/buffer"

	"github.com/gordonklaus/portaudio"
)

// Player is the audio player
type Player struct {
	config      *config.AudioConfig
	audioBuffer *buffer.RingBuffer

	// Playback state
	isPlaying     bool
	audioComplete bool
	mutex         sync.RWMutex
	completeMutex sync.RWMutex

	// Playback stream
	stream *portaudio.Stream

	// Context control
	ctx    context.Context
	cancel context.CancelFunc

	// Debug mode
	enableDebug bool
}

// NewPlayer creates a new audio player
func NewPlayer(parentCtx context.Context, cfg *config.AudioConfig, enableDebug bool) *Player {
	ctx, cancel := context.WithCancel(parentCtx)

	return &Player{
		config:      cfg,
		audioBuffer: buffer.New(cfg.BufferSize),
		ctx:         ctx,
		cancel:      cancel,
		enableDebug: enableDebug,
	}
}

// Stop stops the player
func (p *Player) Stop() error {
	p.cancel()

	p.mutex.Lock()
	if p.stream != nil {
		stopErr := p.stream.Stop()
		if stopErr != nil {
			return stopErr
		}
		closeErr := p.stream.Close()
		if closeErr != nil {
			return closeErr
		}
		p.stream = nil
	}
	p.mutex.Unlock()

	p.audioBuffer.Close()
	return nil
}

// WriteAudioData writes audio data
func (p *Player) WriteAudioData(audioData []byte) {
	written := p.audioBuffer.Write(audioData)
	if p.enableDebug {
		log.Printf("Buffer write: %d bytes, Current buffer: %d bytes", written, p.audioBuffer.Length())
	}

	// If not currently playing, start playback
	p.mutex.Lock()
	if !p.isPlaying {
		if p.enableDebug {
			log.Println("Starting playback...")
		}
		p.isPlaying = true
		go p.playAudio()
	}
	p.mutex.Unlock()
}

// SetAudioComplete sets the audio complete flag
func (p *Player) SetAudioComplete(complete bool) {
	p.completeMutex.Lock()
	p.audioComplete = complete
	p.completeMutex.Unlock()

	if complete && p.enableDebug {
		log.Println("Received playback complete signal")
	}
}

// ClearBuffer clears the audio buffer
func (p *Player) ClearBuffer() {
	p.audioBuffer.Clear()
	if p.enableDebug {
		log.Println("Audio buffer cleared")
	}
}

// StopPlayback immediately stops playback (for interruption)
func (p *Player) StopPlayback() {
	p.mutex.Lock()
	if p.stream != nil && p.isPlaying {
		if p.enableDebug {
			log.Println("Interrupting playback, stopping audio stream...")
		}
		// Set flag but don't close stream directly, let playAudio exit naturally
		p.isPlaying = false
	}
	p.mutex.Unlock()

	// Clear buffer
	p.ClearBuffer()
	// Reset complete flag
	p.SetAudioComplete(false)
}

// IsPlaying checks if currently playing
func (p *Player) IsPlaying() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.isPlaying
}

// playAudio plays audio data
func (p *Player) playAudio() {
	defer func() {
		p.mutex.Lock()
		p.isPlaying = false
		if p.stream != nil {
			if err := p.stream.Stop(); err != nil {
				log.Printf("Failed to stop audio stream: %v", err)
			}
			if err := p.stream.Close(); err != nil {
				log.Printf("Failed to close audio stream: %v", err)
			}
			p.stream = nil
		}
		p.mutex.Unlock()

		if p.enableDebug {
			log.Println("Playback ended")
		}
	}()

	// Playback state control
	var shouldStop bool
	emptyCount := 0
	lastDataTime := time.Now()

	// Open stream using callback function mode
	var err error
	p.stream, err = portaudio.OpenDefaultStream(
		0, 1, // 0 input channels, 1 output channel
		float64(p.config.SampleRate),
		0, // Use default buffer size
		func(out []int16) {
			// Prepare byte buffer
			outBytes := make([]byte, len(out)*2)

			// Read from ring buffer
			n, closed := p.audioBuffer.Read(outBytes)

			if n > 0 {
				lastDataTime = time.Now()
				emptyCount = 0
			} else {
				emptyCount++
			}

			// Convert to int16
			for i := 0; i < n/2; i++ {
				out[i] = int16(outBytes[i*2]) | int16(outBytes[i*2+1])<<8
			}

			// Fill remaining with zeros
			if n < len(outBytes) {
				for i := n / 2; i < len(out); i++ {
					out[i] = 0
				}
			}

			// Check stop conditions
			p.completeMutex.RLock()
			complete := p.audioComplete
			p.completeMutex.RUnlock()

			// Stop condition 1: Received complete signal and buffer is empty
			if complete && p.audioBuffer.Length() == 0 {
				shouldStop = true
			}

			// Stop condition 2: No new data for more than 5 seconds
			if time.Since(lastDataTime) > 5*time.Second {
				shouldStop = true
			}

			// Stop condition 3: 10 consecutive callbacks with no data
			if emptyCount >= 10 {
				shouldStop = true
			}

			// Stop condition 4: Buffer is closed
			if closed {
				shouldStop = true
			}
		},
	)

	if err != nil {
		log.Printf("Failed to open audio stream: %v", err)
		return
	}

	// Start stream
	if err := p.stream.Start(); err != nil {
		log.Printf("Failed to start audio stream: %v", err)
		err := p.stream.Close()
		if err != nil {
			log.Printf("Failed to close audio stream: %v", err)
			return
		}
		p.stream = nil
		return
	}

	log.Println("Audio playback started...")

	// Wait for stop signal
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !shouldStop {
		select {
		case <-ticker.C:
			// Continue checking stop conditions
		case <-p.ctx.Done():
			return
		}
	}
}
