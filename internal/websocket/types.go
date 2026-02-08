package websocket

// InputAudioStreamRequest is the input audio stream request
type InputAudioStreamRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		Buffer string `json:"buffer"`
	} `json:"data"`
}

// InputAudioCompleteRequest is the input audio complete request
type InputAudioCompleteRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		Buffer string `json:"buffer"`
	} `json:"data"`
}

// OutputAudioStreamResponse is the output audio stream response
type OutputAudioStreamResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
		Buffer         string `json:"buffer"`
	} `json:"data"`
}

// OutputAudioCompleteResponse is the output audio complete response
type OutputAudioCompleteResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Data   struct {
		ChatId         string `json:"chatId"`
		ConversationId string `json:"conversationId"`
	} `json:"data"`
}

// OutputTextStreamResponse is the output text stream response
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

// OutputTextCompleteResponse is the output text complete response
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

// ChatCompleteResponse is the chat complete response
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

// CancelOutputRequest is the cancel output request
type CancelOutputRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// ClearContextRequest is the clear context request
type ClearContextRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// GenericServerResponse is the generic server response
type GenericServerResponse struct {
	Action string `json:"action"`
}

// UpdateConfigRequest is the update config request
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

// UpdateConfigResponse is the update config response
type UpdateConfigResponse struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ConversationId string `json:"conversationId"`
	} `json:"data"`
}
