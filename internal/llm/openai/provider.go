package openai

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

// OpenAIProvider implements common.Provider for OpenAI/OpenRouter
type OpenAIProvider struct {
	client *Client
}

// NewOpenAIProvider creates a new OpenAI/OpenRouter provider
func NewOpenAIProvider(config common.ProviderConfig) (*OpenAIProvider, error) {
	// Determine base URL
	baseURL := config.BaseURL
	if baseURL == "" && config.Provider == "openrouter" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	// Create client with config
	client, err := NewClientWithConfig(config.APIKey, config.Model, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	return &OpenAIProvider{client: client}, nil
}

// Chat sends a non-streaming chat request
func (p *OpenAIProvider) Chat(messages []common.Message, tools []common.Tool, thinkingEnabled bool) (*common.ChatResponse, error) {
	// Convert our messages to OpenAI format
	openaiMessages := p.convertMessages(messages)
	openaiTools := p.convertTools(tools)

	response, tokenUsage, err := p.client.Chat(openaiMessages, openaiTools)
	if err != nil {
		return nil, err
	}

	return &common.ChatResponse{
		Message:       response.Message,
		FunctionCalls: convertFunctionCalls(response.ToolCalls),
		TokenUsage:    convertTokenUsage(tokenUsage),
	}, nil
}

// ChatStream sends a streaming chat request
func (p *OpenAIProvider) ChatStream(messages []common.Message, tools []common.Tool, thinkingEnabled bool, callback common.StreamCallback) (*common.ChatResponse, error) {
	// Convert our messages to OpenAI format
	openaiMessages := p.convertMessages(messages)
	openaiTools := p.convertTools(tools)

	response, tokenUsage, err := p.client.ChatStream(openaiMessages, openaiTools, thinkingEnabled, func(chunk string, isThought bool) {
		callback(chunk, isThought) // Forward the callback with isThought parameter
	})
	if err != nil {
		return nil, err
	}

	return &common.ChatResponse{
		Message:       response.Message,
		Reasoning:     response.Reasoning,
		FunctionCalls: convertFunctionCalls(response.ToolCalls),
		TokenUsage:    convertTokenUsage(tokenUsage),
	}, nil
}

// Close closes any resources
func (p *OpenAIProvider) Close() error {
	return nil
}

// convertMessages converts common.Messages to OpenAI format
func (p *OpenAIProvider) convertMessages(messages []common.Message) []Message {
	openaiMessages := make([]Message, 0, len(messages))
	for _, msg := range messages {
		openaiMsg := Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert function calls
		for _, fc := range msg.FunctionCalls {
			openaiMsg.FunctionCalls = append(openaiMsg.FunctionCalls, FunctionCallData{
				ID:   fc.ID,
				Name: fc.Name,
				Args: fc.Arguments,
			})
		}

		// Convert function response
		if msg.FunctionResponse != nil {
			openaiMsg.FunctionResponse = &FunctionResponseData{
				ID:       msg.FunctionResponse.ID,
				Name:     msg.FunctionResponse.Name,
				Response: msg.FunctionResponse.Response,
			}
		}

		openaiMessages = append(openaiMessages, openaiMsg)
	}
	return openaiMessages
}

// convertTools converts common.Tools to OpenAI format
func (p *OpenAIProvider) convertTools(tools []common.Tool) []Tool {
	openaiTools := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		openaiTools = append(openaiTools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return openaiTools
}

// convertFunctionCalls converts OpenAI function calls to common format
func convertFunctionCalls(calls []FunctionCallData) []common.FunctionCall {
	result := make([]common.FunctionCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, common.FunctionCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Args,
		})
	}
	return result
}

// convertTokenUsage converts OpenAI token usage to common format
func convertTokenUsage(usage *TokenUsage) *common.TokenUsage {
	if usage == nil {
		return nil
	}
	return &common.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}
