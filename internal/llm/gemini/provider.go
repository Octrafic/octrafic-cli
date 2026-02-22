package gemini

import (
	"fmt"

	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

// GeminiProvider implements common.Provider for Google Gemini.
type GeminiProvider struct {
	client *Client
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(config common.ProviderConfig) (*GeminiProvider, error) {
	client, err := NewClientWithConfig(config.APIKey, config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiProvider{client: client}, nil
}

// Chat sends a non-streaming chat request.
func (p *GeminiProvider) Chat(messages []common.Message, tools []common.Tool, thinkingEnabled bool) (*common.ChatResponse, error) {
	geminiMessages := p.convertMessages(messages)
	geminiTools := p.convertTools(tools)

	responseText, thoughtText, functionCalls, tokenUsage, err := p.client.Chat(geminiMessages, thinkingEnabled, geminiTools)
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

// ChatStream sends a streaming chat request.
func (p *GeminiProvider) ChatStream(messages []common.Message, tools []common.Tool, thinkingEnabled bool, callback common.StreamCallback) (*common.ChatResponse, error) {
	geminiMessages := p.convertMessages(messages)
	geminiTools := p.convertTools(tools)

	responseText, functionCalls, tokenUsage, err := p.client.ChatStream(geminiMessages, thinkingEnabled, geminiTools, func(chunk string, isThought bool) {
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

// Close closes any resources.
func (p *GeminiProvider) Close() error {
	return nil
}

// convertMessages converts common.Messages to Gemini format.
func (p *GeminiProvider) convertMessages(messages []common.Message) []Message {
	geminiMessages := make([]Message, 0, len(messages))
	for _, msg := range messages {
		geminiMsg := Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}

		for _, fc := range msg.FunctionCalls {
			geminiMsg.FunctionCalls = append(geminiMsg.FunctionCalls, FunctionCallData{
				ID:               fc.ID,
				Name:             fc.Name,
				Args:             fc.Arguments,
				ThoughtSignature: fc.ThoughtSignature,
			})
		}

		if msg.FunctionResponse != nil {
			geminiMsg.FunctionResponse = &FunctionResponseData{
				ID:       msg.FunctionResponse.ID,
				Name:     msg.FunctionResponse.Name,
				Response: msg.FunctionResponse.Response,
			}
		}

		geminiMessages = append(geminiMessages, geminiMsg)
	}
	return geminiMessages
}

// convertTools converts common.Tools to Gemini format.
func (p *GeminiProvider) convertTools(tools []common.Tool) []Tool {
	geminiTools := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		geminiTools = append(geminiTools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return geminiTools
}

// convertFunctionCalls converts Gemini function calls to common format.
func convertFunctionCalls(calls []FunctionCallResult) []common.FunctionCall {
	result := make([]common.FunctionCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, common.FunctionCall{
			ID:               call.ID,
			Name:             call.Name,
			Arguments:        call.Args,
			ThoughtSignature: call.ThoughtSignature,
		})
	}
	return result
}

// convertTokenUsage converts Gemini token usage to common format.
func convertTokenUsage(usage *TokenUsage) *common.TokenUsage {
	if usage == nil {
		return nil
	}
	return &common.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
}
