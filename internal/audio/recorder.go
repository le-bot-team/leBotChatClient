package audio

import (
	"context"
	"log"
	"sync"

	"websocket_client_chat/internal/config"
	"websocket_client_chat/pkg/utils"

	"github.com/gordonklaus/portaudio"
)

// AudioHandler 音频数据处理器接口
type AudioHandler interface {
	OnAudioChunk(requestID string, samples []int16, isLast bool)
	OnRecordingComplete(requestID string, samples []int16)
}

// Recorder 音频录制器
type Recorder struct {
	config  *config.AudioConfig
	handler AudioHandler

	// 录制状态
	isRecording bool
	stream      *portaudio.Stream
	mutex       sync.RWMutex

	// 流式处理
	streamingRequestID string
	streamingBuffer    []int16
	streamingMutex     sync.Mutex

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRecorder 创建新的音频录制器
func NewRecorder(cfg *config.AudioConfig, handler AudioHandler) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())

	return &Recorder{
		config:  cfg,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Initialize 初始化PortAudio
func (r *Recorder) Initialize() error {
	return portaudio.Initialize()
}

// Terminate 终止PortAudio
func (r *Recorder) Terminate() error {
	r.cancel()

	r.mutex.Lock()
	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
	}
	r.mutex.Unlock()

	return portaudio.Terminate()
}

// StartRecording 开始录音
func (r *Recorder) StartRecording(requestID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.isRecording {
		return nil // 已在录音中
	}

	// 初始化流式缓冲区
	r.streamingMutex.Lock()
	r.streamingRequestID = requestID
	r.streamingBuffer = make([]int16, 0, r.config.ChunkSampleCount*2)
	r.streamingMutex.Unlock()

	var err error
	r.stream, err = portaudio.OpenDefaultStream(
		1, 0, // 输入1通道，输出0通道
		float64(r.config.SampleRate),
		0, // 使用默认缓冲区大小
		r.audioCallback,
	)
	if err != nil {
		return err
	}

	if err := r.stream.Start(); err != nil {
		r.stream.Close()
		r.stream = nil
		return err
	}

	r.isRecording = true
	log.Println("开始录音...")
	return nil
}

// StopRecording 停止录音
func (r *Recorder) StopRecording() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.isRecording {
		return nil // 未在录音中
	}

	r.isRecording = false

	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
	}

	// 发送剩余的音频数据
	r.streamingMutex.Lock()
	remainingBuffer := make([]int16, len(r.streamingBuffer))
	copy(remainingBuffer, r.streamingBuffer)
	requestID := r.streamingRequestID
	r.streamingBuffer = nil
	r.streamingMutex.Unlock()

	if len(remainingBuffer) > 0 {
		log.Printf("发送最后的音频数据: %d 采样点", len(remainingBuffer))
		r.handler.OnAudioChunk(requestID, remainingBuffer, true)
	} else {
		// 没有剩余数据，通知录制完成
		r.handler.OnRecordingComplete(requestID, nil)
	}

	log.Println("停止录音")
	return nil
}

// IsRecording 检查是否正在录音
func (r *Recorder) IsRecording() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.isRecording
}

// audioCallback 音频回调函数
func (r *Recorder) audioCallback(in []int16) {
	if !r.isRecording {
		return
	}

	// 流式处理
	r.streamingMutex.Lock()
	r.streamingBuffer = append(r.streamingBuffer, in...)

	// 当缓冲区达到指定数据量时发送
	for len(r.streamingBuffer) >= r.config.ChunkSampleCount {
		chunk := make([]int16, r.config.ChunkSampleCount)
		copy(chunk, r.streamingBuffer[:r.config.ChunkSampleCount])
		r.streamingBuffer = r.streamingBuffer[r.config.ChunkSampleCount:]

		requestID := r.streamingRequestID
		r.streamingMutex.Unlock()

		// 异步发送避免阻塞录音
		go r.handler.OnAudioChunk(requestID, chunk, false)

		r.streamingMutex.Lock()
	}
	r.streamingMutex.Unlock()
}

// ConvertToWAV 将采样数据转换为WAV格式
func (r *Recorder) ConvertToWAV(samples []int16) []byte {
	return utils.ConvertSamplesToWAV(
		samples,
		r.config.SampleRate,
		r.config.Channels,
		r.config.BitDepth,
	)
}
