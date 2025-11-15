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
	Data   struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
	} `json:"data"`
}

// OutputTextStreamResponse 输出文本流响应
type OutputTextStreamResponse struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Data    struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
		Role           string `json:"role"` // "assistant" or "user"
		Text           string `json:"text"`
	} `json:"data"`
}

// OutputTextCompleteResponse 输出文本完成响应
type OutputTextCompleteResponse struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Data    struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
		Role           string `json:"role"` // "assistant" or "user"
		Text           string `json:"text"`
	} `json:"data"`
}

// ChatCompleteResponse 聊天完成响应
type ChatCompleteResponse struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
		CreatedAt      int64  `json:"createdAt"`
		CompletedAt    int64  `json:"completedAt"`
		Errors         []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	} `json:"data"`
}

// CancelOutputRequest 取消输出请求
type CancelOutputRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// ClearContextRequest 清除上下文请求
type ClearContextRequest struct {
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
		Timezone string `json:"timezone,omitempty"`
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
