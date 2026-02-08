package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Octrafic/octrafic-cli/internal/infra/logger"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/invopop/jsonschema"
	"go.uber.org/zap"
)

const (
	// Claude API limits
	MaxTokensNonStreaming = 4096 // Max tokens for regular Chat()
	MaxTokensStreaming    = 8192 // Max tokens for ChatStream()
	ThinkingBudget        = 5000 // Extended thinking budget (tokens)

	// Prompt caching notes:
	// - Both Chat() and ChatStream() use prompt caching via CacheControl
	// - Minimum cacheable: 1024 tokens (Sonnet), 4096 tokens (Haiku)
	// - System messages with cache_control are automatically cached
	// - If prompt is too small, API won't cache (no error thrown)
)

type Client struct {
	client anthropic.Client
	model  string
	ctx    context.Context
}

type Message struct {
	Role             string
	Content          string
	ReasoningContent string
	FunctionResponse *FunctionResponseData
	FunctionCalls    []FunctionCallData
}

type FunctionResponseData struct {
	ID       string // tool_use_id from the original tool use block
	Name     string
	Response map[string]interface{}
}

type FunctionCallData struct {
	ID               string
	Name             string
	Args             map[string]interface{}
	ThoughtSignature []byte
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// NewClient creates a new Claude client (uses env vars)
func NewClient() (*Client, error) {
	return NewClientWithConfig("", "")
}

// NewClientWithConfig creates a new Claude client with explicit API key and model
func NewClientWithConfig(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
	}

	if model == "" {
		model = os.Getenv("ANTHROPIC_MODEL")
		if model == "" {
			model = string(anthropic.ModelClaudeSonnet4_20250514)
		}
	}

	ctx := context.Background()

	// Build client options
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	// Add custom base URL if provided (for proxies)
	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)

	return &Client{
		client: client,
		model:  model,
		ctx:    ctx,
	}, nil
}

// generateToolInputSchema creates a ToolInputSchemaParam from a map
func generateToolInputSchema(inputSchema map[string]interface{}) anthropic.ToolInputSchemaParam {
	properties := make(map[string]jsonschema.Schema)

	// Extract the actual properties from the schema (inputSchema has structure: {type: "object", properties: {...}, required: [...]})
	propertiesRaw, ok := inputSchema["properties"].(map[string]interface{})
	if !ok {
		return anthropic.ToolInputSchemaParam{}
	}

	for propName, propDef := range propertiesRaw {
		propMap, ok := propDef.(map[string]interface{})
		if !ok {
			continue
		}

		// Create schema for this property - Type is a plain string
		schema := jsonschema.Schema{}
		if typ, ok := propMap["type"].(string); ok {
			schema.Type = typ
		}

		if desc, ok := propMap["description"].(string); ok {
			schema.Description = desc
		}

		properties[propName] = schema
	}

	return anthropic.ToolInputSchemaParam{
		Properties: properties,
	}
}

type StreamCallback func(reasoning string, isThought bool)

type FunctionCallResult struct {
	ID               string
	Name             string
	Args             map[string]interface{}
	ThoughtSignature []byte
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// Chat sends a non-streaming chat request
func (c *Client) Chat(messages []Message, thinkingEnabled bool, tools []Tool) (string, string, []FunctionCallResult, *TokenUsage, error) {
	if len(messages) == 0 {
		return "", "", []FunctionCallResult{}, nil, fmt.Errorf("no messages provided")
	}

	// Extract all system instructions (for prompt caching)
	// We may have multiple system messages: base instructions + TOON spec
	var systemBlocks []anthropic.TextBlockParam

	// Convert tools to anthropic format
	var anthropicTools []anthropic.ToolUnionParam
	for _, t := range tools {
		inputSchema := generateToolInputSchema(t.InputSchema)

		toolParam := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: inputSchema,
		}
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{OfTool: &toolParam})
	}

	// Convert messages to anthropic format
	var anthropicMessages []anthropic.MessageParam
	for _, msg := range messages {
		// Collect system messages for prompt caching
		if msg.Role == "system" {
			if msg.Content != "" {
				systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
					Text:         msg.Content,
					CacheControl: anthropic.CacheControlEphemeralParam{},
				})
			}
			continue
		}

		var contentBlocks []anthropic.ContentBlockParamUnion

		// Add text content
		if msg.Content != "" {
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfText: &anthropic.TextBlockParam{Text: msg.Content},
			})
		}

		// Add tool use blocks (for assistant messages with function calls)
		for _, fc := range msg.FunctionCalls {
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    fc.ID,
					Name:  fc.Name,
					Input: fc.Args,
				},
			})
		}

		// Add tool result blocks (for user messages with function responses)
		if msg.FunctionResponse != nil {
			responseJSON, _ := json.Marshal(msg.FunctionResponse.Response)
			contentBlocks = append(contentBlocks, anthropic.NewToolResultBlock(
				msg.FunctionResponse.ID, // tool_use_id from original tool use block
				string(responseJSON),
				false,
			))
		}

		// Create message based on role
		if len(contentBlocks) > 0 {
			var role anthropic.MessageParamRole
			if msg.Role == "assistant" || msg.Role == "model" {
				role = anthropic.MessageParamRoleAssistant
			} else {
				role = anthropic.MessageParamRoleUser
			}

			anthropicMessages = append(anthropicMessages, anthropic.MessageParam{
				Role:    role,
				Content: contentBlocks,
			})
		}
	}

	// Build params
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: MaxTokensNonStreaming,
		Messages:  anthropicMessages,
	}

	// Add all system blocks with cache_control for prompt caching
	// Minimum cacheable: 1024 tokens for Sonnet, 4096 for Haiku
	// If prompt is too small, API just won't cache it (no error)
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}

	// Add tools
	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}



	// Send request
	message, err := c.client.Messages.New(c.ctx, params)
	if err != nil {
		logger.Error("Anthropic error", logger.Err(err))
		return "", "", []FunctionCallResult{}, nil, fmt.Errorf("anthropic error: %w", err)
	}

	// Extract response and tool calls
	var responseText string
	var thoughtText string
	var functionCalls []FunctionCallResult

	for _, block := range message.Content {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			responseText += block.Text
		case anthropic.ToolUseBlock:
			// Convert json.RawMessage to map[string]interface{}
			var args map[string]interface{}
			if err := json.Unmarshal(block.Input, &args); err != nil {
				logger.Warn("Failed to unmarshal tool args", logger.Err(err))
				continue
			}

			fc := FunctionCallResult{
				ID:               block.ID,
				Name:             block.Name,
				Args:             args,
				ThoughtSignature: []byte{},
			}
			functionCalls = append(functionCalls, fc)
		}
	}

	// Log usage


	// Return token usage
	tokenUsage := &TokenUsage{
		InputTokens:  message.Usage.InputTokens,
		OutputTokens: message.Usage.OutputTokens,
	}

	return responseText, thoughtText, functionCalls, tokenUsage, nil
}

// ChatStream sends a streaming chat request
func (c *Client) ChatStream(messages []Message, thinkingEnabled bool, tools []Tool, callback StreamCallback) (string, []FunctionCallResult, *TokenUsage, error) {
	if len(messages) == 0 {
		return "", nil, nil, fmt.Errorf("no messages provided")
	}

	// Extract all system instructions (for prompt caching)
	// We may have multiple system messages: base instructions + TOON spec
	var systemBlocks []anthropic.TextBlockParam

	// Convert tools to anthropic format
	var anthropicTools []anthropic.ToolUnionParam
	for _, t := range tools {
		inputSchema := generateToolInputSchema(t.InputSchema)

		toolParam := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: inputSchema,
		}
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{OfTool: &toolParam})
	}

	// Convert messages to anthropic format
	var anthropicMessages []anthropic.MessageParam
	for _, msg := range messages {
		// Collect system messages for prompt caching
		if msg.Role == "system" {
			if msg.Content != "" {
				systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
					Text:         msg.Content,
					CacheControl: anthropic.CacheControlEphemeralParam{},
				})
			}
			continue
		}

		var contentBlocks []anthropic.ContentBlockParamUnion

		// Add text content
		if msg.Content != "" {
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfText: &anthropic.TextBlockParam{Text: msg.Content},
			})
		}

		// Add tool use blocks
		for _, fc := range msg.FunctionCalls {
			contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    fc.ID,
					Name:  fc.Name,
					Input: fc.Args,
				},
			})
		}

		// Add tool result blocks
		if msg.FunctionResponse != nil {
			responseJSON, _ := json.Marshal(msg.FunctionResponse.Response)
			contentBlocks = append(contentBlocks, anthropic.NewToolResultBlock(
				msg.FunctionResponse.ID, // CRITICAL: tool_use_id, not Name!
				string(responseJSON),
				false,
			))
		}

		// Create message based on role
		if len(contentBlocks) > 0 {
			var role anthropic.MessageParamRole
			if msg.Role == "assistant" || msg.Role == "model" {
				role = anthropic.MessageParamRoleAssistant
			} else {
				role = anthropic.MessageParamRoleUser
			}

			anthropicMessages = append(anthropicMessages, anthropic.MessageParam{
				Role:    role,
				Content: contentBlocks,
			})
		}
	}

	// Build params
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: MaxTokensStreaming,
		Messages:  anthropicMessages,
	}

	if len(systemBlocks) > 0 {
		// Add all system blocks with cache_control for prompt caching
		// Minimum cacheable: 1024 tokens for Sonnet, 4096 for Haiku
		// If prompt is too small, API just won't cache it (no error)
		params.System = systemBlocks
	}

	if len(anthropicTools) > 0 {
		params.Tools = anthropicTools
	}

	// Enable extended thinking if requested
	if thinkingEnabled {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(ThinkingBudget)
		params.MaxTokens = MaxTokensStreaming // Must be greater than thinking budget
	}

	// Create stream
	stream := c.client.Messages.NewStreaming(c.ctx, params)
	accumulatedMessage := anthropic.Message{}
	var inputTokens, outputTokens int64 // Track tokens from MessageDeltaEvent

	for stream.Next() {
		event := stream.Current()

		// Extract token usage BEFORE Accumulate (since Accumulate may not preserve it)
		if deltaEvent, ok := event.AsAny().(anthropic.MessageDeltaEvent); ok {
			// MessageDeltaEvent contains usage info
			inputTokens = int64(deltaEvent.Usage.InputTokens)
			outputTokens = int64(deltaEvent.Usage.OutputTokens)
			logger.Debug("MessageDelta token usage extracted",
				zap.Int64("input", inputTokens),
				zap.Int64("output", outputTokens),
			)
		}

		_ = accumulatedMessage.Accumulate(event)

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.ThinkingDelta:
				// Extended thinking reasoning - stream immediately
				callback(deltaVariant.Thinking, true)
			case anthropic.TextDelta:
				// Regular response - stream immediately
				callback(deltaVariant.Text, false)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return "", nil, nil, fmt.Errorf("stream error: %w", err)
	}

	// Debug: check accumulated message structure
	logger.Debug("After stream",
		zap.Int64("accumulated_input", int64(accumulatedMessage.Usage.InputTokens)),
		zap.Int64("accumulated_output", int64(accumulatedMessage.Usage.OutputTokens)),
	)

	// Extract final response and tool calls
	var responseText string
	var functionCalls []FunctionCallResult

	for _, block := range accumulatedMessage.Content {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			responseText += block.Text
		case anthropic.ToolUseBlock:
			// Convert json.RawMessage to map[string]interface{}
			var args map[string]interface{}
			if err := json.Unmarshal(block.Input, &args); err != nil {
				logger.Warn("Failed to unmarshal tool args", logger.Err(err))
				continue
			}

			fc := FunctionCallResult{
				ID:               block.ID,
				Name:             block.Name,
				Args:             args,
				ThoughtSignature: []byte{},
			}
			functionCalls = append(functionCalls, fc)
		}
	}

	// Log usage
	logger.Debug("Token usage",
		zap.Int64("input_tokens", inputTokens),
		zap.Int64("output_tokens", outputTokens),
		zap.Int64("total", inputTokens+outputTokens),
	)

	// Return token usage
	tokenUsage := &TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}

	return responseText, functionCalls, tokenUsage, nil
}

// Close is a no-op for the Claude client
func (c *Client) Close() {
	// No-op
}
