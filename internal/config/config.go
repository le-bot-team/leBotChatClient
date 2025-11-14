package config

import "time"

// Config 应用程序配置
type Config struct {
	Audio     AudioConfig     `json:"audio"`
	WebSocket WebSocketConfig `json:"websocket"`
	Control   ControlConfig   `json:"control"`
	Device    DeviceConfig    `json:"device"`
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate       int           `json:"sampleRate"`       // 采样率
	Channels         int           `json:"channels"`         // 声道数
	BitDepth         int           `json:"bitDepth"`         // 位深度
	BufferSize       int           `json:"bufferSize"`       // 缓冲区大小
	ChunkDuration    time.Duration `json:"chunkDuration"`    // 音频块时长
	ChunkSampleCount int           `json:"chunkSampleCount"` // 每块采样数
	ChunkByteSize    int           `json:"chunkByteSize"`    // 每块字节数
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	URL            string        `json:"url"`
	ReconnectDelay time.Duration `json:"reconnectDelay"`
	PingInterval   time.Duration `json:"pingInterval"`
	WriteTimeout   time.Duration `json:"writeTimeout"`
	ReadTimeout    time.Duration `json:"readTimeout"`
	MaxMessageSize int64         `json:"maxMessageSize"`
}

// ControlConfig 控制配置
type ControlConfig struct {
	FilePath      string        `json:"filePath"`
	MonitorDelay  time.Duration `json:"monitorDelay"`
	ChannelBuffer int           `json:"channelBuffer"`
	UseStdin      bool          `json:"useStdin"` // 使用标准输入控制（调试模式）
}

// DeviceConfig 设备配置
type DeviceConfig struct {
	SerialNumber string   `json:"serialNumber"`
	VoiceID      string   `json:"voiceId"`
	SpeechRate   int      `json:"speechRate"`
	OutputText   bool     `json:"outputText"`
	Location     Location `json:"location"`
}

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	const (
		sampleRate    = 16000
		audioChannels = 1
		bitDepth      = 2
		chunkDuration = 200 * time.Millisecond
	)

	chunkSampleCount := int(sampleRate * chunkDuration / time.Second)
	chunkByteSize := chunkSampleCount * audioChannels * bitDepth

	return &Config{
		Audio: AudioConfig{
			SampleRate:       sampleRate,
			Channels:         audioChannels,
			BitDepth:         bitDepth,
			BufferSize:       10 * sampleRate * audioChannels * bitDepth, // 10秒缓冲
			ChunkDuration:    chunkDuration,
			ChunkSampleCount: chunkSampleCount,
			ChunkByteSize:    chunkByteSize,
		},
		WebSocket: WebSocketConfig{
			URL: "wss://cafuuchino.studio26f.org:10543/api/v1/chat/ws?token=019a667c-1231-7000-a662-08221ad75f4a",
			//URL:            "ws://cafuuchino.studio26f.org:10580/api/v1/chat/ws?token=019a667c-1231-7000-a662-08221ad75f4a",
			ReconnectDelay: 5 * time.Second,
			PingInterval:   30 * time.Second,
			WriteTimeout:   10 * time.Second,
			ReadTimeout:    60 * time.Second,
			MaxMessageSize: 1024 * 1024, // 1MB
		},
		Control: ControlConfig{
			FilePath:      "/tmp/chat-control",
			MonitorDelay:  100 * time.Millisecond,
			UseStdin:      true, // 默认使用标准输入（调试模式）
			ChannelBuffer: 1,
		},
		Device: DeviceConfig{
			SerialNumber: "DEV-001",
			VoiceID:      "xiaole",
			SpeechRate:   0,
			OutputText:   false,
			Location: Location{
				Latitude:  0,
				Longitude: 0,
			},
		},
	}
}
