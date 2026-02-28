package models

// --- Anthropic Messages API Models ---

type AnthropicRequest struct {
	Model       string         `json:"model"`
	Messages    []AnthropicMsg `json:"messages"`
	System      interface{}    `json:"system,omitempty"` // Can be string or []map[string]interface{}
	MaxTokens   int            `json:"max_tokens"`
	Temperature *float64       `json:"temperature,omitempty"`
	Stream      bool           `json:"stream"`
}

type AnthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or array of complex content blocks
}

// Anthropic SSE Event representations
type AnthropicEvent struct {
	Type    string            `json:"type"`
	Message *AnthropicMessage `json:"message,omitempty"`
	Index   int               `json:"index,omitempty"`
	Delta   *AnthropicDelta   `json:"delta,omitempty"`
	Usage   *AnthropicUsage   `json:"usage,omitempty"`
}

type AnthropicMessage struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Role  string         `json:"role"`
	Usage AnthropicUsage `json:"usage"`
}

type AnthropicDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Anthropic non-streaming response
type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Model        string             `json:"model"`
	Content      []AnthropicContent `json:"content"`
	StopReason   string             `json:"stop_reason,omitempty"`
	StopSequence *string            `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage     `json:"usage"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- GitHub Copilot / OpenAI Chat Completions API Models ---

type CopilotRequest struct {
	Model       string       `json:"model"`
	Messages    []CopilotMsg `json:"messages"`
	Temperature *float64     `json:"temperature,omitempty"`
	Stream      bool         `json:"stream"`
	// Stop        []string      `json:"stop,omitempty"`
}

type CopilotMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"` // Text only for this simplified proxy
}

// Copilot SSE Event (OpenAI Streaming format)
type CopilotResponseChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int          `json:"index"`
		Delta        CopilotDelta `json:"delta"`
		FinishReason *string      `json:"finish_reason"`
	} `json:"choices"`
}

type CopilotDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
