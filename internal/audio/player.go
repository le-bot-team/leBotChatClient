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

// Player 音频播放器
type Player struct {
	config      *config.AudioConfig
	audioBuffer *buffer.RingBuffer

	// 播放状态
	isPlaying     bool
	audioComplete bool
	mutex         sync.RWMutex
	completeMutex sync.RWMutex

	// 播放流
	stream *portaudio.Stream

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc

	// 调试模式
	enableDebug bool
}

// NewPlayer 创建新的音频播放器
func NewPlayer(cfg *config.AudioConfig, enableDebug bool) *Player {
	ctx, cancel := context.WithCancel(context.Background())

	return &Player{
		config:      cfg,
		audioBuffer: buffer.New(cfg.BufferSize),
		ctx:         ctx,
		cancel:      cancel,
		enableDebug: enableDebug,
	}
}

// Stop 停止播放器
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

// WriteAudioData 写入音频数据
func (p *Player) WriteAudioData(audioData []byte) {
	written := p.audioBuffer.Write(audioData)
	if p.enableDebug {
		log.Printf("写入缓冲区: %d 字节, 当前缓冲: %d 字节", written, p.audioBuffer.Length())
	}

	// 如果当前没有在播放，启动播放
	p.mutex.Lock()
	if !p.isPlaying {
		if p.enableDebug {
			log.Println("开始播放...")
		}
		p.isPlaying = true
		go p.playAudio()
	}
	p.mutex.Unlock()
}

// SetAudioComplete 设置音频完成标志
func (p *Player) SetAudioComplete(complete bool) {
	p.completeMutex.Lock()
	p.audioComplete = complete
	p.completeMutex.Unlock()

	if complete && p.enableDebug {
		log.Println("收到播放完成指令")
	}
}

// ClearBuffer 清除音频缓冲区
func (p *Player) ClearBuffer() {
	p.audioBuffer.Clear()
	if p.enableDebug {
		log.Println("已清除音频缓冲区")
	}
}

// StopPlayback 立即停止播放（用于打断）
func (p *Player) StopPlayback() {
	p.mutex.Lock()
	if p.stream != nil && p.isPlaying {
		if p.enableDebug {
			log.Println("打断播放，停止音频流...")
		}
		// 设置标志但不直接关闭 stream，让 playAudio 自然退出
		p.isPlaying = false
	}
	p.mutex.Unlock()

	// 清除缓冲区
	p.ClearBuffer()
	// 重置完成标志
	p.SetAudioComplete(false)
}

// IsPlaying 检查是否正在播放
func (p *Player) IsPlaying() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.isPlaying
}

// playAudio 播放音频数据
func (p *Player) playAudio() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("播放崩溃: %v", r)
		}

		p.mutex.Lock()
		p.isPlaying = false
		if p.stream != nil {
			stopErr := p.stream.Stop()
			if stopErr != nil {
				log.Printf("停止音频流失败: %v", stopErr)
				return
			}
			closeErr := p.stream.Close()
			if closeErr != nil {
				log.Printf("关闭音频流失败: %v", closeErr)
				return
			}
			p.stream = nil
		}
		p.mutex.Unlock()

		if p.enableDebug {
			log.Println("播放结束")
		}
	}()

	// 播放状态控制
	var shouldStop bool
	emptyCount := 0
	lastDataTime := time.Now()

	// 使用回调函数模式打开流
	var err error
	p.stream, err = portaudio.OpenDefaultStream(
		0, 1, // 输入0通道，输出1通道
		float64(p.config.SampleRate),
		0, // 使用默认缓冲区大小
		func(out []int16) {
			// 准备字节缓冲区
			outBytes := make([]byte, len(out)*2)

			// 从环形缓冲区读取
			n, closed := p.audioBuffer.Read(outBytes)

			if n > 0 {
				lastDataTime = time.Now()
				emptyCount = 0
			} else {
				emptyCount++
			}

			// 转换为int16
			for i := 0; i < n/2; i++ {
				out[i] = int16(outBytes[i*2]) | int16(outBytes[i*2+1])<<8
			}

			// 填充剩余部分为0
			if n < len(outBytes) {
				for i := n / 2; i < len(out); i++ {
					out[i] = 0
				}
			}

			// 检查停止条件
			p.completeMutex.RLock()
			complete := p.audioComplete
			p.completeMutex.RUnlock()

			// 停止条件1: 收到完成指令且缓冲区空
			if complete && p.audioBuffer.Length() == 0 {
				shouldStop = true
			}

			// 停止条件2: 超过5秒没有新数据
			if time.Since(lastDataTime) > 5*time.Second {
				shouldStop = true
			}

			// 停止条件3: 连续10次回调没有数据
			if emptyCount >= 10 {
				shouldStop = true
			}

			// 停止条件4: 缓冲区已关闭
			if closed {
				shouldStop = true
			}
		},
	)

	if err != nil {
		log.Printf("打开音频流失败: %v", err)
		return
	}

	// 启动流
	if err := p.stream.Start(); err != nil {
		log.Printf("启动音频流失败: %v", err)
		err := p.stream.Close()
		if err != nil {
			log.Printf("关闭音频流失败: %v", err)
			return
		}
		p.stream = nil
		return
	}

	log.Println("音频播放已启动...")

	// 等待停止信号
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !shouldStop {
		select {
		case <-ticker.C:
			// 继续检查停止条件
		case <-p.ctx.Done():
			return
		}
	}
}
