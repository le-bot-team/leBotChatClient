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

// Config 应用程序配置
type Config struct {
	Audio       AudioConfig     `json:"audio"`
	WebSocket   WebSocketConfig `json:"websocket"`
	Control     ControlConfig   `json:"control"`
	Device      DeviceConfig    `json:"device"`
	EnableDebug bool            `json:"enableDebug"` // 全局调试开关
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate        int           `json:"sampleRate"`        // 输出采样率（发送到服务器）
	CaptureSampleRate int           `json:"captureSampleRate"` // 硬件捕获采样率
	Channels          int           `json:"channels"`          // 声道数
	BitDepth          int           `json:"bitDepth"`          // 位深度
	BufferSize        int           `json:"bufferSize"`        // 缓冲区大小
	ChunkDuration     time.Duration `json:"chunkDuration"`     // 音频块时长
	ChunkSampleCount  int           `json:"chunkSampleCount"`  // 每块采样数（输出）
	ChunkByteSize     int           `json:"chunkByteSize"`     // 每块字节数（输出）
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
	Timezone     string   `json:"timezone,omitempty"` // 时区，例如 "Asia/Shanghai"
}

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	const (
		outputSampleRate  = 16000 // 服务器要求的采样率
		captureSampleRate = 48000 // 硬件原生采样率
		audioChannels     = 1
		bitDepth          = 2
		chunkDuration     = 200 * time.Millisecond
	)

	// 基于输出采样率计算chunk大小
	chunkSampleCount := int(outputSampleRate * chunkDuration / time.Second)
	chunkByteSize := chunkSampleCount * audioChannels * bitDepth

	// Read configs from environment variables
	accessToken := getEnv("ACCESS_TOKEN", "019bf218-a79a-7000-a5d4-88f298c5f0ba")
	enableDebug := getEnv("DEBUG", "0") == "1"
	websocketHost := getEnv("WEBSOCKET_URL", "ws://cafuuchino.studio26f.org:10580")

	return &Config{
		EnableDebug: enableDebug, // 全局调试开关
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
			UseStdin:      true, // 默认使用标准输入（调试模式）
			ChannelBuffer: 1,
		},
		Device: DeviceConfig{
			SerialNumber: "DEV-001",
			VoiceID:      "xiaole",
			SpeechRate:   0,
			OutputText:   true, // 启用文本输出以支持打断逻辑
			Location: Location{
				Latitude:  0,
				Longitude: 0,
			},
			Timezone: "Asia/Shanghai", // 默认时区
		},
	}
}
