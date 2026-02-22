package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Octrafic/octrafic-cli/internal/infra/logger"

	"go.uber.org/zap"
	"google.golang.org/genai"
)

const (
	MaxOutputTokens = 8192
)

// Client wraps the Google GenAI SDK client.
type Client struct {
	client *genai.Client
	model  string
	ctx    context.Context
}

// StreamCallback is called for each chunk during streaming.
type StreamCallback func(chunk string, isThought bool)

// FunctionCallResult holds a parsed tool call from the model response.
type FunctionCallResult struct {
	ID               string
	Name             string
	Args             map[string]interface{}
	ThoughtSignature string
}

// TokenUsage represents token usage information.
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// Message is the internal message format for the Gemini client.
type Message struct {
	Role             string
	Content          string
	ReasoningContent string
	FunctionResponse *FunctionResponseData
	FunctionCalls    []FunctionCallData
}

// FunctionResponseData is the tool result sent back to the model.
type FunctionResponseData struct {
	ID       string
	Name     string
	Response map[string]interface{}
}

// FunctionCallData is a tool call from a prior assistant message (for history).
type FunctionCallData struct {
	ID               string
	Name             string
	Args             map[string]interface{}
	ThoughtSignature string
}

// Tool is the internal tool definition for the Gemini client.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// NewClientWithConfig creates a new Gemini client with explicit API key and model.
func NewClientWithConfig(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY not set")
		}
	}

	if model == "" {
		model = os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-2.5-flash"
		}
	}

	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &Client{
		client: client,
		model:  model,
		ctx:    ctx,
	}, nil
}

// Chat sends a non-streaming chat request.
func (c *Client) Chat(messages []Message, thinkingEnabled bool, tools []Tool) (string, string, []FunctionCallResult, *TokenUsage, error) {
	if len(messages) == 0 {
		return "", "", []FunctionCallResult{}, nil, fmt.Errorf("no messages provided")
	}

	contents, config := c.buildRequest(messages, tools)

	result, err := c.client.Models.GenerateContent(c.ctx, c.model, contents, config)
	if err != nil {
		logger.Error("Gemini API error", logger.Err(err))
		return "", "", []FunctionCallResult{}, nil, fmt.Errorf("gemini error: %w", err)
	}

	responseText, functionCalls := c.extractResponse(result)

	var tokenUsage *TokenUsage
	if result.UsageMetadata != nil {
		tokenUsage = &TokenUsage{
			InputTokens:  int64(result.UsageMetadata.PromptTokenCount),
			OutputTokens: int64(result.UsageMetadata.CandidatesTokenCount),
		}
	}

	return responseText, "", functionCalls, tokenUsage, nil
}

// ChatStream sends a streaming chat request.
func (c *Client) ChatStream(messages []Message, thinkingEnabled bool, tools []Tool, callback StreamCallback) (string, []FunctionCallResult, *TokenUsage, error) {
	if len(messages) == 0 {
		return "", nil, nil, fmt.Errorf("no messages provided")
	}

	contents, config := c.buildRequest(messages, tools)

	var responseText string
	var functionCalls []FunctionCallResult
	var inputTokens, outputTokens int64

	type partialToolCall struct {
		ID               string
		Name             string
		Args             map[string]interface{}
		ThoughtSignature string
	}
	toolCallMap := make(map[int]*partialToolCall)
	toolCallIdx := 0

	for result, err := range c.client.Models.GenerateContentStream(c.ctx, c.model, contents, config) {
		if err != nil {
			logger.Error("Stream error", logger.Err(err))
			return "", nil, nil, fmt.Errorf("stream error: %w", err)
		}

		if result.UsageMetadata != nil {
			inputTokens = int64(result.UsageMetadata.PromptTokenCount)
			outputTokens = int64(result.UsageMetadata.CandidatesTokenCount)
		}

		if len(result.Candidates) > 0 {
			candidate := result.Candidates[0]
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if part.FunctionCall != nil {
						id := part.FunctionCall.ID
						if id == "" {
							id = fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, toolCallIdx)
						}
						var thoughtSig string
						if len(part.ThoughtSignature) > 0 {
							thoughtSig = base64.StdEncoding.EncodeToString(part.ThoughtSignature)
						}
						tc := &partialToolCall{
							ID:               id,
							Name:             part.FunctionCall.Name,
							Args:             part.FunctionCall.Args,
							ThoughtSignature: thoughtSig,
						}
						toolCallMap[toolCallIdx] = tc
						toolCallIdx++

					} else if part.Text != "" {
						isThought := part.Thought
						if !isThought {
							responseText += part.Text
						}
						callback(part.Text, isThought)
					}
				}
			}
		}
	}
	for _, tc := range toolCallMap {
		functionCalls = append(functionCalls, FunctionCallResult{
			ID:               tc.ID,
			Name:             tc.Name,
			Args:             tc.Args,
			ThoughtSignature: tc.ThoughtSignature,
		})
	}

	logger.Debug("Token usage",
		zap.Int64("input_tokens", inputTokens),
		zap.Int64("output_tokens", outputTokens),
		zap.Int64("total", inputTokens+outputTokens),
	)

	tokenUsage := &TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}

	return responseText, functionCalls, tokenUsage, nil
}

// Close is a no-op for the Gemini client.
func (c *Client) Close() {
	// No-op
}

// buildRequest converts internal messages and tools into GenAI SDK types.
func (c *Client) buildRequest(messages []Message, tools []Tool) ([]*genai.Content, *genai.GenerateContentConfig) {
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: MaxOutputTokens,
	}

	var contents []*genai.Content

	for _, msg := range messages {
		if msg.Role == "system" {
			if msg.Content != "" {
				config.SystemInstruction = &genai.Content{
					Parts: []*genai.Part{{Text: msg.Content}},
				}
			}
			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		var parts []*genai.Part

		if msg.Content != "" {
			parts = append(parts, &genai.Part{Text: msg.Content})
		}

		for _, fc := range msg.FunctionCalls {
			var thoughtSig []byte
			if fc.ThoughtSignature != "" {
				thoughtSig, _ = base64.StdEncoding.DecodeString(fc.ThoughtSignature)
			}
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   fc.ID,
					Name: fc.Name,
					Args: fc.Args,
				},
				ThoughtSignature: thoughtSig,
			})
		}

		if msg.FunctionResponse != nil {
			parts = append(parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       msg.FunctionResponse.ID,
					Name:     msg.FunctionResponse.Name,
					Response: msg.FunctionResponse.Response,
				},
			})
		}

		if len(parts) > 0 {
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: parts,
			})
		}
	}

	if len(tools) > 0 {
		var funcDecls []*genai.FunctionDeclaration
		for _, t := range tools {
			funcDecls = append(funcDecls, &genai.FunctionDeclaration{
				Name:                 t.Name,
				Description:          t.Description,
				ParametersJsonSchema: t.InputSchema,
			})
		}
		config.Tools = []*genai.Tool{
			{FunctionDeclarations: funcDecls},
		}
	}

	return contents, config
}

// extractResponse extracts text and function calls from a GenerateContentResponse.
func (c *Client) extractResponse(result *genai.GenerateContentResponse) (string, []FunctionCallResult) {
	var responseText string
	var functionCalls []FunctionCallResult

	if len(result.Candidates) > 0 {
		candidate := result.Candidates[0]
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					responseText += part.Text
				}
				if part.FunctionCall != nil {
					id := part.FunctionCall.ID
					if id == "" {
						id = fmt.Sprintf("call_%s", part.FunctionCall.Name)
					}

					fc := FunctionCallResult{
						ID:   id,
						Name: part.FunctionCall.Name,
						Args: part.FunctionCall.Args,
					}
					functionCalls = append(functionCalls, fc)
				}
			}
		}
	}

	for i := range functionCalls {
		if functionCalls[i].Args == nil {
			argsBytes, _ := json.Marshal(map[string]interface{}{})
			_ = json.Unmarshal(argsBytes, &functionCalls[i].Args)
		}
	}

	return responseText, functionCalls
}
