package websocket

// InputAudioStreamRequest 输入音频流请求
type InputAudioStreamRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		Buffer string `json:"buffer"`
	} `json:"data"`
}

// InputAudioCompleteRequest 输入音频完成请求
type InputAudioCompleteRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		Buffer string `json:"buffer"`
	} `json:"data"`
}

// OutputAudioStreamResponse 输出音频流响应
type OutputAudioStreamResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
		Buffer         string `json:"buffer"`
	} `json:"data"`
}

// OutputAudioCompleteResponse 输出音频完成响应
type OutputAudioCompleteResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// GenericServerResponse 通用服务器响应
type GenericServerResponse struct {
	Action string `json:"action"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
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

// UpdateConfigResponse 更新配置响应
type UpdateConfigResponse struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ConversationId string `json:"conversationId"`
	} `json:"data"`
}
