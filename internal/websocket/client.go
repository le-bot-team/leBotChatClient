package websocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"websocket_client_chat/internal/config"

	"github.com/gorilla/websocket"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleOutputAudioStream(resp *OutputAudioStreamResponse)
	HandleOutputAudioComplete(resp *OutputAudioCompleteResponse)
	HandleUpdateConfig(resp *UpdateConfigResponse)
}

// Client WebSocket客户端
type Client struct {
	config  *config.WebSocketConfig
	conn    *websocket.Conn
	handler MessageHandler
	mutex   sync.RWMutex

	// 通道和上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 重连控制
	reconnectChan chan struct{}
}

// NewClient 创建新的WebSocket客户端
func NewClient(cfg *config.WebSocketConfig, handler MessageHandler) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:        cfg,
		handler:       handler,
		ctx:           ctx,
		cancel:        cancel,
		reconnectChan: make(chan struct{}, 1),
	}
}

// Start 启动WebSocket客户端
func (c *Client) Start() error {
	go c.connectLoop()
	return nil
}

// Stop 停止WebSocket客户端
func (c *Client) Stop() error {
	c.cancel()

	c.mutex.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mutex.Unlock()

	return nil
}

// SendMessage 发送消息
func (c *Client) SendMessage(message interface{}) error {
	c.mutex.RLock()
	conn := c.conn
	c.mutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return fmt.Errorf("设置写超时失败: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	return nil
}

// SendUpdateConfig 发送更新配置请求
func (c *Client) SendUpdateConfig(requestID string, deviceConfig *config.DeviceConfig) error {
	updateMsg := UpdateConfigRequest{
		ID:     requestID,
		Action: "updateConfig",
	}

	updateMsg.Data.SpeechRate = deviceConfig.SpeechRate
	updateMsg.Data.VoiceId = deviceConfig.VoiceID
	updateMsg.Data.OutputText = deviceConfig.OutputText
	updateMsg.Data.Location.Latitude = deviceConfig.Location.Latitude
	updateMsg.Data.Location.Longitude = deviceConfig.Location.Longitude

	return c.SendMessage(updateMsg)
}

// SendAudioStream 发送音频流数据
func (c *Client) SendAudioStream(requestID string, wavData []byte) error {
	msg := InputAudioStreamRequest{
		ID:     requestID,
		Action: "inputAudioStream",
	}
	msg.Data.Buffer = base64.StdEncoding.EncodeToString(wavData)

	return c.SendMessage(msg)
}

// SendAudioComplete 发送音频完成请求
func (c *Client) SendAudioComplete(requestID string, wavData []byte) error {
	msg := InputAudioCompleteRequest{
		ID:     requestID,
		Action: "inputAudioComplete",
	}

	if len(wavData) > 0 {
		msg.Data.Buffer = base64.StdEncoding.EncodeToString(wavData)
	}

	return c.SendMessage(msg)
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.conn != nil
}

// connectLoop 连接循环
func (c *Client) connectLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.connect(); err != nil {
				log.Printf("WebSocket连接失败: %v (%.1f秒后重试)", err, c.config.ReconnectDelay.Seconds())
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.config.ReconnectDelay):
					continue
				}
			}

			// 连接成功，开始消息循环
			c.messageLoop()
		}
	}
}

// connect 建立连接
func (c *Client) connect() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = c.config.WriteTimeout

	conn, _, err := dialer.Dial(c.config.URL, nil)
	if err != nil {
		return err
	}

	// 设置连接参数
	conn.SetReadLimit(c.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		return nil
	})

	c.mutex.Lock()
	c.conn = conn
	c.mutex.Unlock()

	log.Println("WebSocket连接成功")
	return nil
}

// messageLoop 消息循环
func (c *Client) messageLoop() {
	defer func() {
		c.mutex.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mutex.Unlock()
		log.Println("WebSocket连接已断开")
	}()

	// 启动ping协程
	go c.pingLoop()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mutex.RLock()
			conn := c.conn
			c.mutex.RUnlock()

			if conn == nil {
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket接收错误: %v", err)
				return
			}

			if err := c.handleMessage(message); err != nil {
				log.Printf("处理消息失败: %v", err)
			}
		}
	}
}

// pingLoop ping循环
func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mutex.RLock()
			conn := c.conn
			c.mutex.RUnlock()

			if conn == nil {
				return
			}

			if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
				return
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("发送ping失败: %v", err)
				return
			}
		}
	}
}

// handleMessage 处理收到的消息
func (c *Client) handleMessage(message []byte) error {
	// 解析通用响应结构获取action类型
	var baseResp GenericServerResponse
	if err := json.Unmarshal(message, &baseResp); err != nil {
		return fmt.Errorf("解析消息基础结构失败: %w", err)
	}

	// 根据action类型处理不同响应
	switch baseResp.Action {
	case "outputAudioStream":
		var resp OutputAudioStreamResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("解析音频流响应失败: %w", err)
		}
		c.handler.HandleOutputAudioStream(&resp)

	case "outputAudioComplete":
		var resp OutputAudioCompleteResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("解析完成响应失败: %w", err)
		}
		c.handler.HandleOutputAudioComplete(&resp)

	case "updateConfig":
		var resp UpdateConfigResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("解析配置更新响应失败: %w", err)
		}
		c.handler.HandleUpdateConfig(&resp)

	default:
		log.Printf("收到未处理的响应类型: %s", baseResp.Action)
	}

	return nil
}
