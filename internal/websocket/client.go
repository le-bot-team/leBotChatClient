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

// MessageHandler defines the message handler interface
type MessageHandler interface {
	HandleOutputAudioStream(resp *OutputAudioStreamResponse)
	HandleOutputAudioComplete(resp *OutputAudioCompleteResponse)
	HandleOutputTextStream(resp *OutputTextStreamResponse)
	HandleOutputTextComplete(resp *OutputTextCompleteResponse)
	HandleChatComplete(resp *ChatCompleteResponse)
	HandleUpdateConfig(resp *UpdateConfigResponse)
}

// Client is the WebSocket client
type Client struct {
	config  *config.WebSocketConfig
	conn    *websocket.Conn
	handler MessageHandler
	mutex   sync.RWMutex

	// Channels and context
	ctx    context.Context
	cancel context.CancelFunc

	// Reconnection control
	reconnectChan chan struct{}

	// Debug mode
	enableDebug bool
}

// NewClient creates a new WebSocket client
func NewClient(parentCtx context.Context, cfg *config.WebSocketConfig, handler MessageHandler, enableDebug bool) *Client {
	ctx, cancel := context.WithCancel(parentCtx)

	return &Client{
		config:        cfg,
		handler:       handler,
		ctx:           ctx,
		cancel:        cancel,
		reconnectChan: make(chan struct{}, 1),
		enableDebug:   enableDebug,
	}
}

// Start starts the WebSocket client
func (c *Client) Start() error {
	go c.connectLoop()
	return nil
}

// Stop stops the WebSocket client
func (c *Client) Stop() error {
	c.cancel()

	c.mutex.Lock()
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Failed to close WebSocket connection: %v", err)
		}
	}
	c.mutex.Unlock()

	return nil
}

// SendMessage sends a message
func (c *Client) SendMessage(message interface{}) error {
	c.mutex.RLock()
	conn := c.conn
	c.mutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("json encoding failed: %w", err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendUpdateConfig sends an update config request
func (c *Client) SendUpdateConfig(requestID string, deviceConfig *config.DeviceConfig) error {
	updateMsg := UpdateConfigRequest{
		ID:     requestID,
		Action: "updateConfig",
	}

	updateMsg.Data.SpeechRate = deviceConfig.SpeechRate
	updateMsg.Data.VoiceID = deviceConfig.VoiceID
	updateMsg.Data.OutputText = deviceConfig.OutputText
	updateMsg.Data.Location.Latitude = deviceConfig.Location.Latitude
	updateMsg.Data.Location.Longitude = deviceConfig.Location.Longitude
	updateMsg.Data.Timezone = deviceConfig.Timezone

	return c.SendMessage(updateMsg)
}

// SendAudioStream sends audio stream data
func (c *Client) SendAudioStream(requestID string, wavData []byte) error {
	msg := InputAudioStreamRequest{
		ID:     requestID,
		Action: "inputAudioStream",
	}
	msg.Data.Buffer = base64.StdEncoding.EncodeToString(wavData)

	return c.SendMessage(msg)
}

// SendAudioComplete sends audio complete request
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

// SendCancelOutput sends cancel output request
func (c *Client) SendCancelOutput(requestID string) error {
	msg := CancelOutputRequest{
		ID:     requestID,
		Action: "cancelOutput",
	}
	return c.SendMessage(msg)
}

// SendClearContext sends clear context request
func (c *Client) SendClearContext(requestID string) error {
	msg := ClearContextRequest{
		ID:     requestID,
		Action: "clearContext",
	}
	return c.SendMessage(msg)
}

// IsConnected checks connection status
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.conn != nil
}

// connectLoop is the connection loop
func (c *Client) connectLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.connect(); err != nil {
				log.Printf("WebSocket connection failed: %v (retrying in %.1f seconds)", err, c.config.ReconnectDelay.Seconds())
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.config.ReconnectDelay):
					continue
				}
			}

			// Connection successful, start message loop
			c.messageLoop()
		}
	}
}

// connect establishes a connection
func (c *Client) connect() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = c.config.WriteTimeout

	conn, _, err := dialer.Dial(c.config.URL, nil)
	if err != nil {
		return err
	}

	// Set connection parameters
	conn.SetReadLimit(c.config.MaxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	})

	c.mutex.Lock()
	c.conn = conn
	c.mutex.Unlock()

	if c.enableDebug {
		log.Println("WebSocket connected successfully")
	}
	return nil
}

// messageLoop is the message loop
func (c *Client) messageLoop() {
	defer func() {
		c.mutex.Lock()
		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				log.Printf("Failed to close WebSocket connection: %v", err)
			}
			c.conn = nil
		}
		c.mutex.Unlock()
		if c.enableDebug {
			log.Println("WebSocket connection disconnected")
		}
	}()

	// Start ping goroutine
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
				log.Printf("WebSocket receive error: %v", err)
				return
			}

			if err := c.handleMessage(message); err != nil {
				log.Printf("Failed to handle message: %v", err)
			}
		}
	}
}

// pingLoop is the ping loop
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
				log.Printf("Failed to send ping: %v", err)
				return
			}
		}
	}
}

// handleMessage handles received messages
func (c *Client) handleMessage(message []byte) error {
	// Parse generic response structure to get action type
	var baseResp GenericServerResponse
	if err := json.Unmarshal(message, &baseResp); err != nil {
		return fmt.Errorf("failed to parse message base structure: %w", err)
	}

	// Handle different responses based on action type
	switch baseResp.Action {
	case "outputAudioStream":
		var resp OutputAudioStreamResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse audio stream response: %w", err)
		}
		c.handler.HandleOutputAudioStream(&resp)

	case "outputAudioComplete":
		var resp OutputAudioCompleteResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse audio complete response: %w", err)
		}
		c.handler.HandleOutputAudioComplete(&resp)

	case "outputTextStream":
		var resp OutputTextStreamResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse text stream response: %w", err)
		}
		c.handler.HandleOutputTextStream(&resp)

	case "outputTextComplete":
		var resp OutputTextCompleteResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse text complete response: %w", err)
		}
		c.handler.HandleOutputTextComplete(&resp)

	case "chatComplete":
		var resp ChatCompleteResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse chat complete response: %w", err)
		}
		c.handler.HandleChatComplete(&resp)

	case "updateConfig":
		var resp UpdateConfigResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			return fmt.Errorf("failed to parse config update response: %w", err)
		}
		c.handler.HandleUpdateConfig(&resp)

	default:
		log.Printf("Received unhandled response type: %s", baseResp.Action)
	}

	return nil
}
