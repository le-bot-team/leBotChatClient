package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/gorilla/websocket"
)

const (
	deviceSN         = "DEV-001"
	sampleRate       = 16000 // 采样率
	audioChannels    = 1
	bitDepth         = 2                                           // 16-bit PCM = 2字节
	bufferSize       = 10 * sampleRate * audioChannels * bitDepth  // 10秒缓冲区
	chunkDuration    = 200                                         // 每个音频块的时长（毫秒）
	chunkSampleCount = sampleRate * chunkDuration / 1000           // 每个块的采样数量
	chunkByteSize    = chunkSampleCount * audioChannels * bitDepth // 每个块的字节数
	wsURL            = "wss://cafuuchino.studio26f.org:10580/api/v1/chat/ws?token=aaaaaa-b-cccccc-dddddd"
	controlFile      = "/tmp/chatctrl"
)

// 更新请求标志位
var g_updateFlag int32 = 0

// 定义请求和响应结构体
type (
	InputAudioStreamRequest struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Data   struct {
			Buffer string `json:"buffer"`
		} `json:"data"`
	}

	InputAudioCompleteRequest struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Data   struct {
			Buffer string `json:"buffer"`
		} `json:"data"`
	}

	OutputAudioStreamResponse struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Data   struct {
			ChatId         string `json:"chatId"`
			ConversationId string `json:"conversationId"`
			Buffer         string `json:"buffer"`
		} `json:"data"`
	}

	OutputAudioCompleteResponse struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}

	GenericServerResponse struct {
		Action string `json:"action"`
	}

	UpdateConfigRequest struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Data   struct {
			ConversationId string `json:"conversationId"`
			SpeechRate     int    `json:"speechRate"`
			VoiceId        string `json:"voiceId"`
			OutputText     bool   `json:"outputText"`
			Location       struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
		} `json:"data"`
	}

	UpdateConfigResponse struct {
		ID      string `json:"id"`
		Action  string `json:"action"`
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ConversationId string `json:"conversationId"`
		} `json:"data"`
	}
)

// 环形缓冲区实现 (原子操作版本)
type RingBuffer struct {
	buf    []byte
	size   int
	r, w   int32
	count  int32
	closed int32
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

func (rb *RingBuffer) Write(data []byte) int {
	if atomic.LoadInt32(&rb.closed) == 1 {
		return 0
	}

	total := 0
	for len(data) > 0 {
		// 原子获取当前状态
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		// 计算可用空间
		avail := rb.size - int(count)
		if avail == 0 {
			break // 缓冲区已满
		}

		var toWrite int
		if w < r {
			// 写入区域在读取区域之前
			toWrite = min(len(data), int(r)-int(w))
		} else {
			// 写入区域在读取区域之后
			toWrite = min(len(data), rb.size-int(w))
			if toWrite == 0 && r > 0 {
				// 如果尾部空间不足，但头部有空间
				atomic.StoreInt32(&rb.w, 0)
				w = 0
				toWrite = min(len(data), int(r))
			}
		}

		if toWrite == 0 {
			break
		}

		copy(rb.buf[w:], data[:toWrite])
		newW := (w + int32(toWrite)) % int32(rb.size)
		atomic.StoreInt32(&rb.w, newW)
		atomic.AddInt32(&rb.count, int32(toWrite))

		data = data[toWrite:]
		total += toWrite
	}
	return total
}

func (rb *RingBuffer) Read(out []byte) (int, bool) {
	if atomic.LoadInt32(&rb.closed) == 1 && atomic.LoadInt32(&rb.count) == 0 {
		return 0, true // 缓冲区已关闭且无数据
	}

	total := 0
	for len(out) > 0 {
		// 原子获取当前状态
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		if count <= 0 {
			break // 无数据可读
		}

		var toRead int
		if r < w {
			// 读取区域在写入区域之前
			toRead = min(len(out), int(w)-int(r))
		} else {
			// 读取区域在写入区域之后
			toRead = min(len(out), rb.size-int(r))
		}

		if toRead == 0 {
			break
		}

		copy(out, rb.buf[r:r+int32(toRead)])
		newR := (r + int32(toRead)) % int32(rb.size)
		atomic.StoreInt32(&rb.r, newR)
		atomic.AddInt32(&rb.count, int32(-toRead))

		out = out[toRead:]
		total += toRead
	}

	closed := atomic.LoadInt32(&rb.closed) == 1 && atomic.LoadInt32(&rb.count) == 0
	return total, closed
}

func (rb *RingBuffer) Length() int {
	return int(atomic.LoadInt32(&rb.count))
}

func (rb *RingBuffer) Close() {
	atomic.StoreInt32(&rb.closed, 1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type AppState struct {
	Recording          bool
	Playing            bool
	WsConn             *websocket.Conn
	Shutdown           chan struct{}
	ControlChan        chan string
	wsMutex            sync.Mutex
	audioBuffer        *RingBuffer
	audioMutex         sync.Mutex
	audioComplete      bool
	audioCompleteMutex sync.Mutex
	// 流式发送相关
	streamingRequestID string
	streamingBuffer    []int16
	streamingMutex     sync.Mutex
}

func main() {
	if err := portaudio.Initialize(); err != nil {
		log.Fatal("PortAudio初始化失败:", err)
	}
	defer portaudio.Terminate()

	if err := initControlFile(); err != nil {
		log.Fatal(err)
	}

	state := &AppState{
		Shutdown:    make(chan struct{}),
		ControlChan: make(chan string, 1),
		audioBuffer: NewRingBuffer(bufferSize * 3), // 30秒总缓冲
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go fileMonitor(state)
	go recordThread(state)
	go websocketThread(state)

	fmt.Println("语音对讲系统启动成功")
	fmt.Println("使用说明:")
	fmt.Println("向/tmp/chatctrl写入:")
	fmt.Println("1 - 开始录音")
	fmt.Println("2 - 停止录音并发送")

	select {
	case <-sigChan:
		fmt.Println("\n收到退出信号")
	case <-state.Shutdown:
	}

	close(state.Shutdown)
	if state.WsConn != nil {
		state.WsConn.Close()
	}
	fmt.Println("系统安全退出")
}

func initControlFile() error {
	return ioutil.WriteFile(controlFile, []byte{}, 0644)
}

func fileMonitor(state *AppState) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastCmd string
	for {
		select {
		case <-state.Shutdown:
			return
		case <-ticker.C:
			content, err := ioutil.ReadFile(controlFile)
			if err != nil {
				log.Printf("读取控制文件失败: %v", err)
				continue
			}

			currentValue := string(bytes.TrimSpace(content))
			if currentValue == "" || currentValue == lastCmd {
				continue
			}

			lastCmd = currentValue
			log.Printf("检测到命令: %s", currentValue)

			select {
			case state.ControlChan <- currentValue:
				if err := ioutil.WriteFile(controlFile, []byte{}, 0644); err != nil {
					log.Printf("清空控制文件失败: %v", err)
				}
			default:
				log.Printf("控制通道已满，丢弃命令: %s", currentValue)
			}
		}
	}
}

func recordThread(state *AppState) {
	var (
		stream      *portaudio.Stream
		audioBuffer []int16
	)

	for {
		select {
		case <-state.Shutdown:
			if stream != nil {
				stream.Close()
			}
			return
		case cmd := <-state.ControlChan:
			switch cmd {
			case "1":
				if !state.Recording {
					log.Println("开始录音...")
					startRecording(state, &stream, &audioBuffer)
				} else {
					log.Println("系统忙，忽略开始录音命令")
				}
			case "2":
				if state.Recording {
					log.Println("停止录音...")
					stopRecording(state, &stream, &audioBuffer)
				} else {
					log.Println("未在录音状态，忽略停止命令")
				}
			}
		}
	}
}

func startRecording(state *AppState, stream **portaudio.Stream, buffer *[]int16) {
	state.Recording = true
	*buffer = make([]int16, 0)

	// 生成流式请求ID并发送更新配置
	state.streamingMutex.Lock()
	state.streamingRequestID = generateRequestID()
	state.streamingBuffer = make([]int16, 0, chunkSampleCount*2) // 预分配容量
	state.streamingMutex.Unlock()

	// 发送配置更新请求
	go sendUpdateConfig(state, state.streamingRequestID)

	var err error
	*stream, err = portaudio.OpenDefaultStream(1, 0, float64(sampleRate), 0, func(in []int16) {
		if state.Recording {
			*buffer = append(*buffer, in...)

			// 流式发送处理
			state.streamingMutex.Lock()
			state.streamingBuffer = append(state.streamingBuffer, in...)

			// 当缓冲区达到200ms的数据量时发送
			for len(state.streamingBuffer) >= chunkSampleCount {
				chunk := make([]int16, chunkSampleCount)
				copy(chunk, state.streamingBuffer[:chunkSampleCount])
				state.streamingBuffer = state.streamingBuffer[chunkSampleCount:]

				// 异步发送避免阻塞录音
				go sendAudioChunk(state, state.streamingRequestID, chunk, false)
			}
			state.streamingMutex.Unlock()
		}
	})
	if err != nil {
		log.Printf("打开音频流失败: %v", err)
		state.Recording = false
		return
	}

	if err := (*stream).Start(); err != nil {
		log.Printf("启动音频流失败: %v", err)
		state.Recording = false
		return
	}
}

func stopRecording(state *AppState, stream **portaudio.Stream, buffer *[]int16) {
	state.Recording = false
	if *stream != nil {
		(*stream).Stop()
		(*stream).Close()
		*stream = nil
	}

	// 发送剩余的音频数据作为完成请求
	state.streamingMutex.Lock()
	remainingBuffer := make([]int16, len(state.streamingBuffer))
	copy(remainingBuffer, state.streamingBuffer)
	requestID := state.streamingRequestID
	state.streamingBuffer = nil
	state.streamingMutex.Unlock()

	if len(remainingBuffer) > 0 {
		log.Printf("发送最后的音频数据: %d 采样点", len(remainingBuffer))
		go sendAudioChunk(state, requestID, remainingBuffer, true) // true表示这是最后一包
	} else {
		// 没有剩余数据，直接发送完成请求
		go sendCompleteRequest(state, requestID, nil)
	}
}

func sendUpdateConfig(state *AppState, requestID string) {
	state.wsMutex.Lock()
	defer state.wsMutex.Unlock()

	if state.WsConn == nil {
		log.Println("WebSocket未连接,无法发送配置更新")
		return
	}

	updateMsg := UpdateConfigRequest{
		ID:     requestID,
		Action: "updateConfig",
		Data: struct {
			ConversationId string `json:"conversationId"`
			SpeechRate     int    `json:"speechRate"`
			VoiceId        string `json:"voiceId"`
			OutputText     bool   `json:"outputText"`
			Location       struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
		}{
			ConversationId: "test",
			SpeechRate:     0,
			VoiceId:        "xiaole",
			OutputText:     false,
			Location: struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}{
				Latitude:  0,
				Longitude: 0,
			},
		},
	}

	updateData, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("JSON编码更新请求失败: %v", err)
		return
	}

	if err := state.WsConn.WriteMessage(websocket.TextMessage, updateData); err != nil {
		log.Printf("发送更新请求失败: %v", err)
		return
	}
	log.Println("更新请求已发送")

	// 等待标志位更新
	for {
		if g_updateFlag == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	g_updateFlag = 0
	log.Println("更新响应成功，开始流式录音发送")
}

func sendAudioChunk(state *AppState, requestID string, audioSamples []int16, isLast bool) {
	// 转换int16采样为字节数据
	audioBytes := make([]byte, len(audioSamples)*2)
	for i, sample := range audioSamples {
		audioBytes[i*2] = byte(sample & 0xFF)
		audioBytes[i*2+1] = byte((sample >> 8) & 0xFF)
	}

	if isLast {
		// 发送最后一包作为完成请求
		go sendCompleteRequest(state, requestID, audioBytes)
		return
	}

	state.wsMutex.Lock()
	defer state.wsMutex.Unlock()

	if state.WsConn == nil {
		log.Println("WebSocket未连接,无法发送音频数据")
		return
	}

	// 发送普通的音频流请求
	msg := InputAudioStreamRequest{
		ID:     requestID,
		Action: "inputAudioStream",
	}
	msg.Data.Buffer = base64.StdEncoding.EncodeToString(audioBytes)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON编码音频流失败: %v", err)
		return
	}

	if err := state.WsConn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("发送音频流失败: %v", err)
		return
	}
	log.Printf("发送音频数据块: %d 字节", len(audioBytes))
}

func sendCompleteRequest(state *AppState, requestID string, lastAudioBytes []byte) {
	state.wsMutex.Lock()
	defer state.wsMutex.Unlock()

	if state.WsConn == nil {
		log.Println("WebSocket未连接,无法发送完成请求")
		return
	}

	completeMsg := InputAudioCompleteRequest{
		ID:     requestID,
		Action: "inputAudioComplete",
	}

	if len(lastAudioBytes) > 0 {
		completeMsg.Data.Buffer = base64.StdEncoding.EncodeToString(lastAudioBytes)
		log.Printf("发送完成请求(包含最后%d字节音频)", len(lastAudioBytes))
	} else {
		completeMsg.Data.Buffer = ""
		log.Println("发送完成请求(无剩余音频)")
	}

	completeData, err := json.Marshal(completeMsg)
	if err != nil {
		log.Printf("JSON编码完成消息失败: %v", err)
		return
	}

	if err := state.WsConn.WriteMessage(websocket.TextMessage, completeData); err != nil {
		log.Printf("发送完成通知失败: %v", err)
		return
	}
	log.Println("完成通知已发送")
}

func generateRequestID() string {
	return fmt.Sprintf("%s-%d", deviceSN, time.Now().UnixNano())
}

func websocketThread(state *AppState) {
	for {
		select {
		case <-state.Shutdown:
			return
		default:
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				log.Printf("WebSocket连接失败: %v (5秒后重试)", err)
				time.Sleep(5 * time.Second)
				continue
			}

			state.wsMutex.Lock()
			state.WsConn = conn
			state.wsMutex.Unlock()

			log.Println("WebSocket连接成功")
			readMessages(state, conn)
		}
	}
}

func readMessages(state *AppState, conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket接收错误: %v", err)
			return
		}

		// 解析通用响应结构获取action类型
		var baseResp GenericServerResponse
		if err := json.Unmarshal(message, &baseResp); err != nil {
			log.Printf("解析消息基础结构失败: %v", err)
			continue
		}

		// 根据action类型处理不同响应
		switch baseResp.Action {
		case "outputAudioStream":
			handleOutputAudioStream(message, state)
		case "outputAudioComplete":
			handleOutputAudioComplete(message, state)
		case "updateConfig":
			handleUpdateConfig(message, state)
		default:
			log.Printf("收到未处理的响应类型: %s", baseResp.Action)
		}
	}
}

func handleUpdateConfig(message []byte, state *AppState) {
	var resp UpdateConfigResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		log.Printf("解析配置更新响应失败: %v", err)
		return
	}

	log.Printf("收到配置更新响应: Success=%v, Message=%s", resp.Success, resp.Message)
	g_updateFlag = 1

}

func handleOutputAudioComplete(message []byte, state *AppState) {
	var resp OutputAudioCompleteResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		log.Printf("解析完成响应失败: %v", err)
		return
	}
	log.Println("收到播放完成指令")

	state.audioCompleteMutex.Lock()
	state.audioComplete = true
	state.audioCompleteMutex.Unlock()
}

func handleOutputAudioStream(message []byte, state *AppState) {
	var resp OutputAudioStreamResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		log.Printf("解析音频流响应失败: %v", err)
		return
	}

	log.Printf("收到音频流响应: ID=%s, 会话ID=%s, 对话ID=%s",
		resp.ID, resp.Data.ConversationId, resp.Data.ChatId)

	audioData, err := base64.StdEncoding.DecodeString(resp.Data.Buffer)
	if err != nil {
		log.Printf("音频解码失败: %v", err)
		return
	}

	log.Printf("音频数据大小: %d 字节", len(audioData))

	// 写入环形缓冲区
	written := state.audioBuffer.Write(audioData)
	log.Printf("写入缓冲区: %d 字节, 当前缓冲: %d 字节", written, state.audioBuffer.Length())

	// 如果当前没有在播放，启动播放
	state.audioMutex.Lock()
	if !state.Playing {
		log.Println("开始播放...")
		state.Playing = true
		go playAudio(state)
	}
	state.audioMutex.Unlock()
}

// 播放音频数据 - 原子操作版本
func playAudio(state *AppState) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("播放崩溃: %v", r)
		}
		state.audioMutex.Lock()
		state.Playing = false
		state.audioMutex.Unlock()
	}()

	// 使用回调函数模式打开流
	var shouldStop bool
	emptyCount := 0
	lastDataTime := time.Now()

	stream, err := portaudio.OpenDefaultStream(0, 1, float64(sampleRate), 0, func(out []int16) {
		// 准备字节缓冲区
		outBytes := make([]byte, len(out)*2)

		// 从环形缓冲区读取
		n, closed := state.audioBuffer.Read(outBytes)

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
		state.audioCompleteMutex.Lock()
		complete := state.audioComplete
		state.audioCompleteMutex.Unlock()

		// 停止条件1: 收到完成指令且缓冲区空
		if complete && state.audioBuffer.Length() == 0 {
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
	})

	if err != nil {
		log.Printf("打开音频流失败: %v", err)
		return
	}
	defer stream.Close()

	// 启动流
	if err := stream.Start(); err != nil {
		log.Printf("启动音频流失败: %v", err)
		return
	}
	defer stream.Stop()

	log.Println("音频播放已启动...")

	// 等待停止信号
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !shouldStop {
		select {
		case <-ticker.C:
			// 继续检查停止条件
		case <-state.Shutdown:
			return
		}
	}

	log.Println("播放结束")
}
