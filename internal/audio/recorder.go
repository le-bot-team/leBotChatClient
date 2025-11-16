package audio

import (
	"context"
	"fmt"
	"log"
	"strings"
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

	// 音频设备状态
	targetDevice      *portaudio.DeviceInfo
	isPortAudioInit   bool
	deviceInitialized bool

	// 录制状态
	isRecording bool
	stream      *portaudio.Stream
	mutex       sync.RWMutex

	// 流式处理
	streamingRequestID string
	streamingBuffer    []int16
	resampleBuffer     []int16 // 用于重采样的缓冲区
	streamingMutex     sync.Mutex

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc

	// 调试模式
	enableDebug bool
}

// NewRecorder 创建新的音频录制器
func NewRecorder(cfg *config.AudioConfig, handler AudioHandler, enableDebug bool) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())

	return &Recorder{
		config:      cfg,
		handler:     handler,
		ctx:         ctx,
		cancel:      cancel,
		enableDebug: enableDebug,
	}
}

// Initialize 初始化音频设备
func (r *Recorder) Initialize() error {
	if r.isPortAudioInit {
		return nil
	}

	// 初始化PortAudio
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("PortAudio初始化失败: %v", err)
	}
	r.isPortAudioInit = true

	// 查找音频设备
	if err := r.findAudioDevice(); err != nil {
		err := portaudio.Terminate()
		if err != nil {
			return err
		}
		r.isPortAudioInit = false
		return err
	}

	r.deviceInitialized = true
	return nil
}

// findAudioDevice 查找合适的音频输入设备
func (r *Recorder) findAudioDevice() error {
	host, err := portaudio.DefaultHostApi()
	if err != nil {
		return fmt.Errorf("获取Host API失败: %v", err)
	}
	if host == nil {
		return fmt.Errorf("获取Host API返回nil")
	}

	// 在debug模式下列出所有可用设备
	if r.enableDebug {
		log.Println("=== 可用的音频输入设备 ===")
		for i, dev := range host.Devices {
			if dev.MaxInputChannels > 0 {
				log.Printf("[%d] %s (输入通道: %d, 采样率: %.0f Hz)",
					i, dev.Name, dev.MaxInputChannels, dev.DefaultSampleRate)
			}
		}
		log.Println("=========================")
	}

	// 优先级匹配逻辑（按优先级从高到低）
	var candidates []*portaudio.DeviceInfo
	var priorities []int

	for _, dev := range host.Devices {
		if dev.MaxInputChannels < r.config.Channels {
			continue
		}

		devNameLower := strings.ToLower(dev.Name)
		priority := 0

		// 优先级1: 明确的麦克风设备
		if strings.Contains(devNameLower, "microphone") ||
			strings.Contains(devNameLower, "mic") {
			priority = 100
		}

		// 优先级2: 数字麦克风(通常质量较好)
		if strings.Contains(devNameLower, "digital") {
			priority += 50
		}

		// 优先级3: sof-hda-dsp 设备
		if strings.Contains(devNameLower, "sof-hda-dsp") {
			priority += 40
		}

		// 优先级4: 嵌入式特定设备
		if strings.Contains(devNameLower, "audiocodec") ||
			strings.Contains(dev.Name, "hw:0,0") {
			priority += 30
		}

		// 排除不需要的设备
		if strings.Contains(devNameLower, "monitor") ||
			strings.Contains(devNameLower, "loopback") {
			continue
		}

		if priority > 0 {
			candidates = append(candidates, dev)
			priorities = append(priorities, priority)
		}
	}

	// 选择优先级最高的设备
	maxPriority := -1
	for i, p := range priorities {
		if p > maxPriority {
			maxPriority = p
			r.targetDevice = candidates[i]
		}
	}

	// 回退到默认设备
	if r.targetDevice == nil {
		if defDev, err := portaudio.DefaultInputDevice(); err == nil {
			r.targetDevice = defDev
			log.Println("警告：未找到匹配的录音设备，使用默认输入设备")
		} else {
			return fmt.Errorf("未找到可用的录音设备")
		}
	}

	log.Printf("选中录音设备: %s (输入通道: %d, 默认采样率: %.0f Hz)",
		r.targetDevice.Name, r.targetDevice.MaxInputChannels, r.targetDevice.DefaultSampleRate)

	if r.enableDebug {
		log.Printf("捕获采样率: %d Hz, 输出采样率: %d Hz",
			r.config.CaptureSampleRate, r.config.SampleRate)
	}

	return nil
}

// Terminate 终止音频系统
func (r *Recorder) Terminate() error {
	r.cancel()

	r.mutex.Lock()
	if r.stream != nil {
		stopErr := r.stream.Stop()
		if stopErr != nil {
			return stopErr
		}
		closeErr := r.stream.Close()
		if closeErr != nil {
			return closeErr
		}
		r.stream = nil
	}
	r.mutex.Unlock()

	if r.isPortAudioInit {
		err := portaudio.Terminate()
		r.isPortAudioInit = false
		r.deviceInitialized = false
		return err
	}
	return nil
}

// StartRecording 开始录音
func (r *Recorder) StartRecording(requestID string) error {
	if !r.deviceInitialized {
		return fmt.Errorf("音频设备未初始化")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.isRecording {
		return nil // 已在录音中
	}

	// 初始化流式缓冲区
	r.streamingMutex.Lock()
	r.streamingRequestID = requestID
	// 捕获缓冲区需要更大（基于捕获采样率）
	captureChunkSize := int(float64(r.config.CaptureSampleRate) * r.config.ChunkDuration.Seconds())
	r.streamingBuffer = make([]int16, 0, captureChunkSize*2)
	r.resampleBuffer = make([]int16, 0, r.config.ChunkSampleCount*2)
	r.streamingMutex.Unlock()

	// 确定实际使用的采样率
	actualSampleRate := r.config.CaptureSampleRate

	// 如果设备不支持配置的采样率，尝试使用设备的默认采样率
	if r.targetDevice.DefaultSampleRate > 0 &&
		r.targetDevice.DefaultSampleRate != float64(r.config.CaptureSampleRate) {
		if r.enableDebug {
			log.Printf("设备默认采样率 %.0f Hz 与配置的 %d Hz 不同，将尝试配置的采样率",
				r.targetDevice.DefaultSampleRate, r.config.CaptureSampleRate)
		}
	}

	// 配置音频流参数
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   r.targetDevice,
			Channels: r.config.Channels,
			Latency:  r.targetDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(actualSampleRate),
		FramesPerBuffer: 1024,
	}

	var err error
	r.stream, err = portaudio.OpenStream(params, r.audioCallback)
	if err != nil {
		// 如果打开失败，尝试使用设备的默认采样率
		if actualSampleRate != int(r.targetDevice.DefaultSampleRate) && r.targetDevice.DefaultSampleRate > 0 {
			log.Printf("使用采样率 %d Hz 打开流失败: %v", actualSampleRate, err)
			actualSampleRate = int(r.targetDevice.DefaultSampleRate)
			log.Printf("尝试使用设备默认采样率: %d Hz", actualSampleRate)

			// 更新采样率并重试
			params.SampleRate = float64(actualSampleRate)
			r.stream, err = portaudio.OpenStream(params, r.audioCallback)
			if err == nil {
				// 成功了，更新配置中的捕获采样率
				r.config.CaptureSampleRate = actualSampleRate
				log.Printf("成功使用默认采样率 %d Hz 打开音频流", actualSampleRate)
			}
		}

		if err != nil {
			return fmt.Errorf("打开音频流失败: %v", err)
		}
	}

	if err := r.stream.Start(); err != nil {
		err := r.stream.Close()
		if err != nil {
			return err
		}
		r.stream = nil
		return fmt.Errorf("启动录音失败: %v", err)
	}

	r.isRecording = true
	if r.enableDebug {
		log.Printf("开始录音 (设备: %s, 捕获采样率: %dHz, 输出采样率: %dHz)",
			r.targetDevice.Name, r.config.CaptureSampleRate, r.config.SampleRate)
	}
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
		stopErr := r.stream.Stop()
		if stopErr != nil {
			return stopErr
		}
		closeErr := r.stream.Close()
		if closeErr != nil {
			return closeErr
		}
		r.stream = nil
	}

	// 发送剩余的音频数据
	r.streamingMutex.Lock()
	remainingBuffer := make([]int16, len(r.streamingBuffer))
	copy(remainingBuffer, r.streamingBuffer)
	resampleBuffer := make([]int16, len(r.resampleBuffer))
	copy(resampleBuffer, r.resampleBuffer)
	requestID := r.streamingRequestID
	r.streamingBuffer = nil
	r.resampleBuffer = nil
	r.streamingMutex.Unlock()

	// 重采样剩余的捕获数据
	if len(remainingBuffer) > 0 {
		resampled := utils.ResampleAudio(remainingBuffer, r.config.CaptureSampleRate, r.config.SampleRate)
		resampleBuffer = append(resampleBuffer, resampled...)
	}

	if len(resampleBuffer) > 0 {
		// 最后一块音频的诊断（仅在debug模式下）
		if r.enableDebug {
			rms := utils.CalculateRMS(resampleBuffer)
			stats := utils.CalculateAudioStats(resampleBuffer, 100)
			isSilent := utils.IsSilent(resampleBuffer, 200.0, 0.95)

			log.Printf("最后音频块诊断 - RMS: %.2f, Peak: %d, 静音比例: %.2f%%, 是否静音: %v, 样本数: %d",
				rms, stats.Peak, stats.SilenceRatio*100, isSilent, len(resampleBuffer))
			log.Printf("发送最后的音频数据: %d 采样点", len(resampleBuffer))
		}
		r.handler.OnAudioChunk(requestID, resampleBuffer, true)
	} else {
		// 没有剩余数据，通知录制完成
		log.Println("警告：录音结束但没有捕获到任何音频数据")
		r.handler.OnRecordingComplete(requestID, nil)
	}

	if r.enableDebug {
		log.Println("停止录音")
	}
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

	// 流式处理 - 累积捕获的数据
	r.streamingMutex.Lock()
	r.streamingBuffer = append(r.streamingBuffer, in...)

	// 计算需要多少捕获样本才能生成一个输出chunk
	captureChunkSize := int(float64(r.config.CaptureSampleRate) * r.config.ChunkDuration.Seconds())

	// 当捕获缓冲区有足够数据时，进行重采样
	for len(r.streamingBuffer) >= captureChunkSize {
		// 取出一个捕获chunk
		captureChunk := r.streamingBuffer[:captureChunkSize]
		r.streamingBuffer = r.streamingBuffer[captureChunkSize:]

		// 重采样到目标采样率
		resampled := utils.ResampleAudio(captureChunk, r.config.CaptureSampleRate, r.config.SampleRate)
		r.resampleBuffer = append(r.resampleBuffer, resampled...)

		// 当重采样缓冲区达到输出chunk大小时发送
		for len(r.resampleBuffer) >= r.config.ChunkSampleCount {
			chunk := make([]int16, r.config.ChunkSampleCount)
			copy(chunk, r.resampleBuffer[:r.config.ChunkSampleCount])
			r.resampleBuffer = r.resampleBuffer[r.config.ChunkSampleCount:]

			// 音频诊断（仅在debug模式下）
			if r.enableDebug {
				rms := utils.CalculateRMS(chunk)
				stats := utils.CalculateAudioStats(chunk, 100)
				isSilent := utils.IsSilent(chunk, 200.0, 0.95)

				log.Printf("音频诊断 - RMS: %.2f, Peak: %d, 静音比例: %.2f%%, 是否静音: %v",
					rms, stats.Peak, stats.SilenceRatio*100, isSilent)
			}

			requestID := r.streamingRequestID
			r.streamingMutex.Unlock()

			// 异步发送避免阻塞录音
			go r.handler.OnAudioChunk(requestID, chunk, false)

			r.streamingMutex.Lock()
		}
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
