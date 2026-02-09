package agent

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

type BaseAgent struct {
	provider common.Provider
	messages []common.Message
}

func NewBaseAgent(provider common.Provider) *BaseAgent {
	return &BaseAgent{
		provider: provider,
		messages: []common.Message{},
	}
}

func (a *BaseAgent) Chat(systemPrompt string, tools []common.Tool, inputMessages []ChatMessage, thinkingEnabled bool) (*ChatResponse, error) {
	messages := []common.Message{
		{Role: "system", Content: systemPrompt},
	}

	for _, msg := range inputMessages {
		commonMsg := common.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}

		for _, fc := range msg.FunctionCalls {
			commonMsg.FunctionCalls = append(commonMsg.FunctionCalls, common.FunctionCall{
				ID:        fc.ID,
				Name:      fc.Name,
				Arguments: fc.Arguments,
			})
		}

		if msg.FunctionResponse != nil {
			commonMsg.FunctionResponse = &common.FunctionResponseData{
				ID:       msg.FunctionResponse.ID,
				Name:     msg.FunctionResponse.Name,
				Response: msg.FunctionResponse.Response,
			}
		}

		messages = append(messages, commonMsg)
	}

	response, err := a.provider.Chat(messages, tools, thinkingEnabled)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %w", err)
	}

	var toolCalls []ToolCall
	for _, fc := range response.FunctionCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        fc.ID,
			Name:      fc.Name,
			Arguments: fc.Arguments,
		})
	}

	chatResp := &ChatResponse{
		Message:      response.Message,
		Reasoning:    response.Reasoning,
		ToolCalls:    toolCalls,
		TokensUsed:   0,
		InputTokens:  0,
		OutputTokens: 0,
	}

	if response.TokenUsage != nil {
		chatResp.TokensUsed = response.TokenUsage.InputTokens + response.TokenUsage.OutputTokens
		chatResp.InputTokens = response.TokenUsage.InputTokens
		chatResp.OutputTokens = response.TokenUsage.OutputTokens
	}

	return chatResp, nil
}

func (a *BaseAgent) ChatStream(systemPrompt string, tools []common.Tool, inputMessages []ChatMessage, thinkingEnabled bool, callback ReasoningCallback) (*ChatResponse, error) {
	messages := []common.Message{
		{Role: "system", Content: systemPrompt},
	}

	for _, msg := range inputMessages {
		commonMsg := common.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}

		for _, fc := range msg.FunctionCalls {
			commonMsg.FunctionCalls = append(commonMsg.FunctionCalls, common.FunctionCall{
				ID:        fc.ID,
				Name:      fc.Name,
				Arguments: fc.Arguments,
			})
		}

		if msg.FunctionResponse != nil {
			commonMsg.FunctionResponse = &common.FunctionResponseData{
				ID:       msg.FunctionResponse.ID,
				Name:     msg.FunctionResponse.Name,
				Response: msg.FunctionResponse.Response,
			}
		}

		messages = append(messages, commonMsg)
	}

	response, err := a.provider.ChatStream(messages, tools, thinkingEnabled, func(chunk string, isThought bool) {
		if chunk != "" {
			callback(chunk, isThought)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("chat stream failed: %w", err)
	}

	var toolCalls []ToolCall
	for _, fc := range response.FunctionCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        fc.ID,
			Name:      fc.Name,
			Arguments: fc.Arguments,
		})
	}

	chatResp := &ChatResponse{
		Message:      response.Message,
		Reasoning:    response.Reasoning,
		ToolCalls:    toolCalls,
		TokensUsed:   0,
		InputTokens:  0,
		OutputTokens: 0,
	}

	if response.TokenUsage != nil {
		chatResp.TokensUsed = response.TokenUsage.InputTokens + response.TokenUsage.OutputTokens
		chatResp.InputTokens = response.TokenUsage.InputTokens
		chatResp.OutputTokens = response.TokenUsage.OutputTokens
	}

	return chatResp, nil
}
