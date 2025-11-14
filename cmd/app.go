package main

import (
	"context"
	"encoding/base64"
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

// App 应用程序主结构
type App struct {
	config *config.Config

	// 各组件
	recorder     *audio.Recorder
	player       *audio.Player
	wsClient     *websocket.Client
	fileMonitor  *control.FileMonitor
	stdinMonitor *control.StdinMonitor

	// 状态管理
	updateFlag int32 // 更新响应标志位

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewApp 创建新的应用程序实例
func NewApp() *App {
	cfg := config.DefaultConfig()
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// 初始化各组件
	app.recorder = audio.NewRecorder(&cfg.Audio, app)
	app.player = audio.NewPlayer(&cfg.Audio)
	app.wsClient = websocket.NewClient(&cfg.WebSocket, app)

	// 根据配置选择控制方式
	if cfg.Control.UseStdin {
		app.stdinMonitor = control.NewStdinMonitor(&cfg.Control, app)
	} else {
		app.fileMonitor = control.NewFileMonitor(&cfg.Control, app)
	}

	return app
}

// Start 启动应用程序
func (app *App) Start() error {
	// 初始化PortAudio
	if err := app.recorder.Initialize(); err != nil {
		return err
	}

	// 启动WebSocket客户端
	if err := app.wsClient.Start(); err != nil {
		return err
	}

	// 启动相应的控制监控器
	if app.config.Control.UseStdin {
		if err := app.stdinMonitor.Start(); err != nil {
			return err
		}
		log.Println("语音对讲系统启动成功 (标准输入控制模式)")
		log.Println("输入命令:")
		log.Println("  1 或 start - 开始录音")
		log.Println("  2 或 stop  - 停止录音并发送")
		log.Println("  q 或 quit  - 退出程序")
	} else {
		if err := app.fileMonitor.Start(); err != nil {
			return err
		}
		log.Println("语音对讲系统启动成功 (文件控制模式)")
		log.Println("使用说明:")
		log.Println("向/tmp/chat-control写入:")
		log.Println("  1 - 开始录音")
		log.Println("  2 - 停止录音并发送")
	}

	return nil
}

// Stop 停止应用程序
func (app *App) Stop() error {
	app.cancel()

	// 停止各组件
	if app.fileMonitor != nil {
		if err := app.fileMonitor.Stop(); err != nil {
			log.Printf("停止文件监控失败: %v", err)
		}
	}

	if app.stdinMonitor != nil {
		if err := app.stdinMonitor.Stop(); err != nil {
			log.Printf("停止标准输入监控失败: %v", err)
		}
	}

	if err := app.wsClient.Stop(); err != nil {
		log.Printf("停止WebSocket客户端失败: %v", err)
	}

	if err := app.player.Stop(); err != nil {
		log.Printf("停止音频播放器失败: %v", err)
	}

	if err := app.recorder.Terminate(); err != nil {
		log.Printf("终止音频录制器失败: %v", err)
	}

	// 等待所有goroutine结束
	app.wg.Wait()

	log.Println("系统安全退出")
	return nil
}

// Wait 等待应用程序结束
func (app *App) Wait() {
	<-app.ctx.Done()
}

// === 实现 control.Handler 接口 ===

// HandleCommand 处理控制命令
func (app *App) HandleCommand(cmd control.Command) {
	switch cmd {
	case control.CmdStartRecording:
		if !app.recorder.IsRecording() {
			requestID := utils.GenerateRequestID(app.config.Device.SerialNumber)

			// 发送配置更新请求并等待响应
			app.wg.Add(1)
			go func() {
				defer app.wg.Done()
				app.sendUpdateConfigAndWait(requestID)

				// 配置更新成功后开始录音
				if err := app.recorder.StartRecording(requestID); err != nil {
					log.Printf("开始录音失败: %v", err)
				}
			}()
		} else {
			log.Println("系统忙，忽略开始录音命令")
		}

	case control.CmdStopRecording:
		if app.recorder.IsRecording() {
			if err := app.recorder.StopRecording(); err != nil {
				log.Printf("停止录音失败: %v", err)
			}
		} else {
			log.Println("未在录音状态，忽略停止命令")
		}
	}
}

// === 实现 audio.AudioHandler 接口 ===

// OnAudioChunk 处理音频块
func (app *App) OnAudioChunk(requestID string, samples []int16, isLast bool) {
	wavData := app.recorder.ConvertToWAV(samples)

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		var err error
		if isLast {
			err = app.wsClient.SendAudioComplete(requestID, wavData)
			if err == nil {
				log.Printf("发送完成请求(包含最后%d字节WAV音频)", len(wavData))
			}
		} else {
			err = app.wsClient.SendAudioStream(requestID, wavData)
			if err == nil {
				log.Printf("发送WAV音频数据块: %d 字节", len(wavData))
			}
		}

		if err != nil {
			log.Printf("发送音频数据失败: %v", err)
		}
	}()
}

// OnRecordingComplete 录制完成
func (app *App) OnRecordingComplete(requestID string, _ []int16) {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		if err := app.wsClient.SendAudioComplete(requestID, nil); err != nil {
			log.Printf("发送完成通知失败: %v", err)
		} else {
			log.Println("发送完成请求(无剩余音频)")
		}
	}()
}

// === 实现 websocket.MessageHandler 接口 ===

// HandleOutputAudioStream 处理输出音频流
func (app *App) HandleOutputAudioStream(resp *websocket.OutputAudioStreamResponse) {
	log.Printf("收到音频流响应: ID=%s, 会话ID=%s, 对话ID=%s",
		resp.ID, resp.Data.ConversationId, resp.Data.ChatId)

	audioData, err := base64.StdEncoding.DecodeString(resp.Data.Buffer)
	if err != nil {
		log.Printf("音频解码失败: %v", err)
		return
	}

	log.Printf("音频数据大小: %d 字节", len(audioData))

	// 写入播放缓冲区
	app.player.WriteAudioData(audioData)
}

// HandleOutputAudioComplete 处理输出音频完成
func (app *App) HandleOutputAudioComplete(_ *websocket.OutputAudioCompleteResponse) {
	app.player.SetAudioComplete(true)
}

// HandleUpdateConfig 处理更新配置响应
func (app *App) HandleUpdateConfig(resp *websocket.UpdateConfigResponse) {
	log.Printf("收到配置更新响应: Success=%v, Message=%s", resp.Success, resp.Message)
	atomic.StoreInt32(&app.updateFlag, 1)
}

// sendUpdateConfigAndWait 发送配置更新请求并等待响应
func (app *App) sendUpdateConfigAndWait(requestID string) {
	if err := app.wsClient.SendUpdateConfig(requestID, &app.config.Device); err != nil {
		log.Printf("发送配置更新失败: %v", err)
		return
	}

	log.Println("更新请求已发送")

	// 等待标志位更新
	for atomic.LoadInt32(&app.updateFlag) == 0 {
		select {
		case <-app.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// 继续等待
		}
	}

	atomic.StoreInt32(&app.updateFlag, 0)
	log.Println("更新响应成功，开始流式录音发送")
}
