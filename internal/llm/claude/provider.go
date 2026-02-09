package claude

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

// ClaudeProvider implements common.Provider for Claude
type ClaudeProvider struct {
	client *Client
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(config common.ProviderConfig) (*ClaudeProvider, error) {
	client, err := NewClientWithConfig(config.APIKey, config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude client: %w", err)
	}

	return &ClaudeProvider{client: client}, nil
}

// Chat sends a non-streaming chat request
func (p *ClaudeProvider) Chat(messages []common.Message, tools []common.Tool, thinkingEnabled bool) (*common.ChatResponse, error) {
	// Convert our messages to Claude format
	claudeMessages := p.convertMessages(messages)
	claudeTools := p.convertTools(tools)

	responseText, thoughtText, functionCalls, tokenUsage, err := p.client.Chat(claudeMessages, thinkingEnabled, claudeTools)
	if err != nil {
		return nil, err
	}

	return &common.ChatResponse{
		Message:       responseText,
		Reasoning:     thoughtText,
		FunctionCalls: convertFunctionCalls(functionCalls),
		TokenUsage:    convertTokenUsage(tokenUsage),
	}, nil
}

// ChatStream sends a streaming chat request
func (p *ClaudeProvider) ChatStream(messages []common.Message, tools []common.Tool, thinkingEnabled bool, callback common.StreamCallback) (*common.ChatResponse, error) {
	// Convert our messages to Claude format
	claudeMessages := p.convertMessages(messages)
	claudeTools := p.convertTools(tools)

	responseText, functionCalls, tokenUsage, err := p.client.ChatStream(claudeMessages, thinkingEnabled, claudeTools, func(chunk string, isThought bool) {
		callback(chunk, isThought)
	})
	if err != nil {
		return nil, err
	}

	return &common.ChatResponse{
		Message:       responseText,
		FunctionCalls: convertFunctionCalls(functionCalls),
		TokenUsage:    convertTokenUsage(tokenUsage),
	}, nil
}

// Close closes any resources
func (p *ClaudeProvider) Close() error {
	return nil
}

// convertMessages converts common.Messages to Claude format
func (p *ClaudeProvider) convertMessages(messages []common.Message) []Message {
	claudeMessages := make([]Message, 0, len(messages))
	for _, msg := range messages {
		claudeMsg := Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}

		// Convert function calls
		for _, fc := range msg.FunctionCalls {
			claudeMsg.FunctionCalls = append(claudeMsg.FunctionCalls, FunctionCallData{
				ID:               fc.ID,
				Name:             fc.Name,
				Args:             fc.Arguments,
				ThoughtSignature: []byte{},
			})
		}

		// Convert function response
		if msg.FunctionResponse != nil {
			claudeMsg.FunctionResponse = &FunctionResponseData{
				ID:       msg.FunctionResponse.ID,
				Name:     msg.FunctionResponse.Name,
				Response: msg.FunctionResponse.Response,
			}
		}

		claudeMessages = append(claudeMessages, claudeMsg)
	}
	return claudeMessages
}

// convertTools converts common.Tools to Claude format
func (p *ClaudeProvider) convertTools(tools []common.Tool) []Tool {
	claudeTools := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		claudeTools = append(claudeTools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return claudeTools
}

// convertFunctionCalls converts Claude function calls to llm format
func convertFunctionCalls(calls []FunctionCallResult) []common.FunctionCall {
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

// convertTokenUsage converts Claude token usage to llm format
func convertTokenUsage(usage *TokenUsage) *common.TokenUsage {
	if usage == nil {
		return nil
	}
	return &common.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}
