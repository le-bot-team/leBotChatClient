package config

import (
	"fmt"
	"os"
	"time"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Config is the application configuration
type Config struct {
	Audio       AudioConfig     `json:"audio"`
	WebSocket   WebSocketConfig `json:"websocket"`
	Control     ControlConfig   `json:"control"`
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
	FilePath      string        `json:"filePath"`
	MonitorDelay  time.Duration `json:"monitorDelay"`
	ChannelBuffer int           `json:"channelBuffer"`
	UseStdin      bool          `json:"useStdin"` // Use stdin control (debug mode)
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

	// Read configs from environment variables
	accessToken := getEnv("ACCESS_TOKEN", "019bf218-a79a-7000-a5d4-88f298c5f0ba")
	enableDebug := getEnv("DEBUG", "0") == "1"
	websocketHost := getEnv("WEBSOCKET_URL", "ws://cafuuchino.studio26f.org:10580")

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
			FilePath:      "/tmp/chat-control",
			MonitorDelay:  100 * time.Millisecond,
			UseStdin:      true, // Default to stdin (debug mode)
			ChannelBuffer: 1,
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
