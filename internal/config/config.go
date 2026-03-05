package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// fileConfig holds values loaded from config.toml
type fileConfig struct {
	AccessToken  string `toml:"access_token"`
	Debug        bool   `toml:"debug"`
	WebsocketURL string `toml:"websocket_url"`
}

// loadFileConfig reads config.toml from the executable's directory or CWD.
// If neither location has a config.toml, defaults are returned.
func loadFileConfig() fileConfig {
	cfg := fileConfig{
		AccessToken:  "019cb9c7-4e91-7000-aa0b-a06b6f9b475a",
		Debug:        false,
		WebsocketURL: "ws://cafuuchino.studio26f.org:10580",
	}

	// Try config.toml next to the executable
	if exePath, err := os.Executable(); err == nil {
		tomlPath := filepath.Join(filepath.Dir(exePath), "config.toml")
		if _, err := toml.DecodeFile(tomlPath, &cfg); err == nil {
			log.Printf("[Config] Loaded config from %s", tomlPath)
			return cfg
		}
	}

	// Try config.toml in the current working directory
	if _, err := toml.DecodeFile("config.toml", &cfg); err == nil {
		log.Printf("[Config] Loaded config from config.toml (CWD)")
		return cfg
	}

	log.Println("[Config] No config.toml found, using defaults")
	return cfg
}

// Config is the application configuration
type Config struct {
	Audio       AudioConfig     `json:"audio"`
	WebSocket   WebSocketConfig `json:"websocket"`
	Control     ControlConfig   `json:"control"`
	Gpio        GpioConfig      `json:"gpio"`
	Wake        WakeConfig      `json:"wake"`
	Device      DeviceConfig    `json:"device"`
	EnableDebug bool            `json:"enableDebug"` // Global debug switch
}

// AudioConfig is the audio configuration
type AudioConfig struct {
	SampleRate        int           `json:"sampleRate"`        // Output sample rate (sent to server)
	CaptureSampleRate int           `json:"captureSampleRate"` // Hardware capture sample rate
	Channels          int           `json:"channels"`          // Number of channels
	BitDepth          int           `json:"bitDepth"`          // Bit depth
	BufferSize        int           `json:"bufferSize"`        // Buffer size
	ChunkDuration     time.Duration `json:"chunkDuration"`     // Audio chunk duration
	ChunkSampleCount  int           `json:"chunkSampleCount"`  // Samples per chunk (output)
	ChunkByteSize     int           `json:"chunkByteSize"`     // Bytes per chunk (output)
}

// WebSocketConfig is the WebSocket configuration
type WebSocketConfig struct {
	URL            string        `json:"url"`
	ReconnectDelay time.Duration `json:"reconnectDelay"`
	PingInterval   time.Duration `json:"pingInterval"`
	WriteTimeout   time.Duration `json:"writeTimeout"`
	ReadTimeout    time.Duration `json:"readTimeout"`
	MaxMessageSize int64         `json:"maxMessageSize"`
}

// ControlConfig is the control configuration
type ControlConfig struct {
	FilePath     string        `json:"filePath"`
	MonitorDelay time.Duration `json:"monitorDelay"`
}

// GpioConfig is the GPIO configuration
type GpioConfig struct {
	PinNumber    int           `json:"pinNumber"`    // GPIO pin number (e.g. 200 = PG8)
	PollInterval time.Duration `json:"pollInterval"` // Polling interval for GPIO value
}

// WakeConfig is the wake/sleep state configuration
type WakeConfig struct {
	BufferDuration       time.Duration `json:"bufferDuration"`       // Circular wake buffer duration (e.g. 8s)
	SilenceCheckInterval time.Duration `json:"silenceCheckInterval"` // How often to check for silence in active state
	SilenceThresholdRMS  float64       `json:"silenceThresholdRms"`  // RMS threshold for silence detection
	SilenceRatio         float64       `json:"silenceRatio"`         // Ratio of silent samples to consider as silence
	SilenceBufferSeconds int           `json:"silenceBufferSeconds"` // Seconds of audio to keep for silence checking
}

// DeviceConfig is the device configuration
type DeviceConfig struct {
	SerialNumber string   `json:"serialNumber"`
	VoiceID      string   `json:"voiceId"`
	SpeechRate   int      `json:"speechRate"`
	OutputText   bool     `json:"outputText"`
	Location     Location `json:"location"`
	Timezone     string   `json:"timezone,omitempty"` // Timezone, e.g. "Asia/Shanghai"
}

// Location is the location information
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	const (
		outputSampleRate  = 16000 // Sample rate required by server
		captureSampleRate = 48000 // Native hardware sample rate
		audioChannels     = 1
		bitDepth          = 2
		chunkDuration     = 200 * time.Millisecond
	)

	// Calculate chunk size based on output sample rate
	chunkSampleCount := int(outputSampleRate * chunkDuration / time.Second)
	chunkByteSize := chunkSampleCount * audioChannels * bitDepth

	// Read configs from config.toml
	fileCfg := loadFileConfig()
	accessToken := fileCfg.AccessToken
	enableDebug := fileCfg.Debug
	websocketHost := fileCfg.WebsocketURL

	return &Config{
		EnableDebug: enableDebug, // Global debug switch
		Audio: AudioConfig{
			SampleRate:        outputSampleRate,
			CaptureSampleRate: captureSampleRate,
			Channels:          audioChannels,
			BitDepth:          bitDepth,
			BufferSize:        16 * outputSampleRate * audioChannels * bitDepth,
			ChunkDuration:     chunkDuration,
			ChunkSampleCount:  chunkSampleCount,
			ChunkByteSize:     chunkByteSize,
		},
		WebSocket: WebSocketConfig{
			URL:            fmt.Sprintf("%s/api/v1/chat/ws?token=%s", websocketHost, accessToken),
			ReconnectDelay: 5 * time.Second,
			PingInterval:   30 * time.Second,
			WriteTimeout:   10 * time.Second,
			ReadTimeout:    60 * time.Second,
			MaxMessageSize: 1024 * 1024, // 1MB
		},
		Control: ControlConfig{
			FilePath:     "/tmp/chat-control",
			MonitorDelay: 100 * time.Millisecond,
		},
		Gpio: GpioConfig{
			PinNumber:    200,
			PollInterval: 100 * time.Millisecond,
		},
		Wake: WakeConfig{
			BufferDuration:       8 * time.Second,
			SilenceCheckInterval: 2 * time.Second,
			SilenceThresholdRMS:  200.0,
			SilenceRatio:         0.95,
			SilenceBufferSeconds: 3,
		},
		Device: DeviceConfig{
			SerialNumber: "DEV-001",
			VoiceID:      "xiaole",
			SpeechRate:   0,
			OutputText:   true, // Enable text output to support interruption logic
			Location: Location{
				Latitude:  0,
				Longitude: 0,
			},
			Timezone: "Asia/Shanghai", // Default timezone
		},
	}
}
