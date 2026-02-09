package common

import "time"

// Message represents a chat message
type Message struct {
	Role             string                `json:"role"`
	Content          string                `json:"content"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	FunctionCalls    []FunctionCall        `json:"function_calls,omitempty"`
	FunctionResponse *FunctionResponseData `json:"function_response,omitempty"`
}

// FunctionCall represents a tool/function call
type FunctionCall struct {
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// FunctionResponseData represents the response to a function call
type FunctionResponseData struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// Tool represents a tool/function definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// ChatResponse represents the response from a chat request
type ChatResponse struct {
	Message       string         `json:"message"`
	Reasoning     string         `json:"reasoning,omitempty"`
	FunctionCalls []FunctionCall `json:"function_calls,omitempty"`
	TokenUsage    *TokenUsage    `json:"token_usage,omitempty"`
}

// StreamCallback is called for each chunk during streaming
type StreamCallback func(chunk string, isThought bool)

// Provider represents an LLM provider interface
type Provider interface {
	// Chat sends a non-streaming chat request
	Chat(messages []Message, tools []Tool, thinkingEnabled bool) (*ChatResponse, error)

	// ChatStream sends a streaming chat request
	ChatStream(messages []Message, tools []Tool, thinkingEnabled bool, callback StreamCallback) (*ChatResponse, error)

	// Close closes any resources
	Close() error
}

// ProviderConfig holds configuration for creating a provider
type ProviderConfig struct {
	Provider string // "claude", "openai", "openrouter", "ollama", "llamacpp"
	APIKey   string
	BaseURL  string // optional override
	Model    string // model name
	Timeout  time.Duration
}
