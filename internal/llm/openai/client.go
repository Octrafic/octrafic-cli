package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"regexp"

	"github.com/tidwall/gjson"
)

// thinkTagFilter separates <think>...</think> content from regular content in streaming responses.
// Local models (Ollama/llama.cpp) embed reasoning inside <think> tags in the content field
// instead of using a dedicated reasoning field.
type thinkTagFilter struct {
	insideThink bool
	trimNext    bool // trim leading whitespace from next content emission after </think>
	buffer      string
	callback    StreamCallback
}

func newThinkTagFilter(callback StreamCallback) *thinkTagFilter {
	return &thinkTagFilter{callback: callback}
}

func (f *thinkTagFilter) Process(chunk string) {
	f.buffer += chunk

	for {
		if f.insideThink {
			idx := strings.Index(f.buffer, "</think>")
			if idx == -1 {
				// Check for partial closing tag at the end
				for i := 1; i < len("</think>") && i <= len(f.buffer); i++ {
					if strings.HasSuffix(f.buffer, "</think>"[:i]) {
						// Flush safe part as reasoning
						safe := f.buffer[:len(f.buffer)-i]
						if safe != "" {
							f.callback(safe, true)
						}
						f.buffer = f.buffer[len(f.buffer)-i:]
						return
					}
				}
				// No partial tag, flush all as reasoning
				if f.buffer != "" {
					f.callback(f.buffer, true)
					f.buffer = ""
				}
				return
			}
			// Send everything before </think> as reasoning
			if idx > 0 {
				f.callback(f.buffer[:idx], true)
			}
			f.buffer = f.buffer[idx+len("</think>"):]
			f.insideThink = false
			f.trimNext = true
		} else {
			// Trim leading whitespace from content right after </think>
			if f.trimNext {
				f.buffer = strings.TrimLeft(f.buffer, " \t\n\r")
				if f.buffer == "" {
					return
				}
				f.trimNext = false
			}

			idx := strings.Index(f.buffer, "<think>")
			if idx == -1 {
				// Check for partial opening tag at the end
				for i := 1; i < len("<think>") && i <= len(f.buffer); i++ {
					if strings.HasSuffix(f.buffer, "<think>"[:i]) {
						safe := f.buffer[:len(f.buffer)-i]
						if safe != "" {
							f.callback(safe, false)
						}
						f.buffer = f.buffer[len(f.buffer)-i:]
						return
					}
				}
				// No partial tag, flush all as content
				if f.buffer != "" {
					f.callback(f.buffer, false)
					f.buffer = ""
				}
				return
			}
			// Send everything before <think> as content
			if idx > 0 {
				f.callback(f.buffer[:idx], false)
			}
			f.buffer = f.buffer[idx+len("<think>"):]
			f.insideThink = true
		}
	}
}

func (f *thinkTagFilter) Flush() {
	if f.buffer != "" {
		if f.insideThink {
			f.callback(f.buffer, true)
		} else {
			f.callback(f.buffer, false)
		}
		f.buffer = ""
	}
}

var thinkTagRegex = regexp.MustCompile(`(?s)<think>.*?</think>\s*`)

// stripThinkTags removes <think>...</think> blocks from non-streaming responses
func stripThinkTags(content string) (cleaned string, reasoning string) {
	var reasoningParts []string
	matches := thinkTagRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, ""
	}
	last := 0
	var cleanedParts []string
	for _, match := range matches {
		cleanedParts = append(cleanedParts, content[last:match[0]])
		thinkContent := content[match[0]+len("<think>") : match[1]-len("</think>")]
		thinkContent = strings.TrimSuffix(thinkContent, "\n")
		reasoningParts = append(reasoningParts, strings.TrimSpace(thinkContent))
		last = match[1]
	}
	cleanedParts = append(cleanedParts, content[last:])
	return strings.TrimSpace(strings.Join(cleanedParts, "")), strings.Join(reasoningParts, "\n")
}

// Message represents a chat message
type Message struct {
	Role             string
	Content          string
	FunctionResponse *FunctionResponseData
	FunctionCalls    []FunctionCallData
}

// FunctionResponseData represents a function/tool response
type FunctionResponseData struct {
	ID       string // tool_call_id from the original tool call block
	Name     string
	Response map[string]interface{}
}

// FunctionCallData represents a function/tool call
type FunctionCallData struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// Tool represents a tool/function definition
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// StreamCallback is called for each chunk as it's streamed
// isThought indicates if this chunk is reasoning/thinking (true) or regular content (false)
type StreamCallback func(chunk string, isThought bool)

// ChatResponse represents the response from a chat completion
type ChatResponse struct {
	Message   string
	Reasoning string // Reasoning/thinking content
	ToolCalls []FunctionCallData
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// Client is a custom OpenAI/OpenRouter client
type Client struct {
	httpClient *http.Client
	apiKey     string
	model      string
	baseURL    string
	ctx        context.Context
}

// NewClient creates a new client from environment variables
func NewClient() (*Client, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	model := os.Getenv("SEARCH_SPEC_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return NewClientWithConfig(apiKey, model, baseURL)
}

// NewClientWithConfig creates a new client with explicit configuration
func NewClientWithConfig(apiKey, model, baseURL string) (*Client, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		model:      model,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		ctx:        context.Background(),
	}, nil
}

// ChatStream sends a streaming chat request
func (c *Client) ChatStream(messages []Message, tools []Tool, thinkingEnabled bool, callback StreamCallback) (*ChatResponse, *TokenUsage, error) {
	return c.chatStream(messages, tools, thinkingEnabled, callback)
}

// chatStream uses /v1/chat/completions
func (c *Client) chatStream(messages []Message, tools []Tool, thinkingEnabled bool, callback StreamCallback) (*ChatResponse, *TokenUsage, error) {
	reqBody := c.buildRequestPayload(messages, tools, true)
	
	if strings.Contains(c.baseURL, "openrouter.ai") {
		reqBody["include_reasoning"] = true
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, err
	}

	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	var accumulatedContent, accumulatedReasoning string
	var toolCalls []FunctionCallData
	var usage *TokenUsage

	// Filter that separates <think>...</think> tags from content (for Ollama/llama.cpp)
	filter := newThinkTagFilter(func(chunk string, isThought bool) {
		if isThought {
			accumulatedReasoning += chunk
			callback(chunk, true)
		} else {
			accumulatedContent += chunk
			callback(chunk, false)
		}
	})

	type pendingToolCall struct {
		id   string
		name string
		args strings.Builder
	}
	pendingTools := make(map[int]*pendingToolCall)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF { break }
			return nil, nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") { continue }

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" { break }

		res := gjson.Parse(data)

		reasoning := ""
		if r := res.Get("choices.0.delta.reasoning_content"); r.Exists() {
			reasoning = r.String()
		} else if r := res.Get("choices.0.delta.reasoning"); r.Exists() {
			reasoning = r.String()
		} else if r := res.Get("choices.0.delta.thought"); r.Exists() {
			reasoning = r.String()
		}

		if reasoning != "" {
			accumulatedReasoning += reasoning
			callback(reasoning, true)
		}

		content := res.Get("choices.0.delta.content").String()
		if content != "" {
			filter.Process(content)
		}

		tcs := res.Get("choices.0.delta.tool_calls").Array()
		for _, tc := range tcs {
			idx := int(tc.Get("index").Int())
			if pt, ok := pendingTools[idx]; ok {
				pt.args.WriteString(tc.Get("function.arguments").String())
			} else {
				pendingTools[idx] = &pendingToolCall{
					id:   tc.Get("id").String(),
					name: tc.Get("function.name").String(),
				}
				pendingTools[idx].args.WriteString(tc.Get("function.arguments").String())
			}
		}

		if res.Get("usage").Exists() {
			usage = &TokenUsage{
				InputTokens:  res.Get("usage.prompt_tokens").Int(),
				OutputTokens: res.Get("usage.completion_tokens").Int(),
			}
		}
	}

	filter.Flush()

	for _, pt := range pendingTools {
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(pt.args.String()), &args)
		toolCalls = append(toolCalls, FunctionCallData{ID: pt.id, Name: pt.name, Args: args})
	}

	return &ChatResponse{
		Message:   accumulatedContent,
		Reasoning: accumulatedReasoning,
		ToolCalls: toolCalls,
	}, usage, nil
}

// Chat sends a non-streaming chat request
func (c *Client) Chat(messages []Message, tools []Tool) (*ChatResponse, *TokenUsage, error) {
	return c.chat(messages, tools)
}

func (c *Client) chat(messages []Message, tools []Tool) (*ChatResponse, *TokenUsage, error) {
	reqBody := c.buildRequestPayload(messages, tools, false)
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil { return nil, nil, err }

	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, nil, err }
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	res := gjson.ParseBytes(body)
	msg := res.Get("choices.0.message")

	reasoning := msg.Get("reasoning_content").String()
	if reasoning == "" { reasoning = msg.Get("reasoning").String() }

	content := msg.Get("content").String()

	// Strip <think>...</think> tags from content (Ollama/llama.cpp embed reasoning inline)
	if strings.Contains(content, "<think>") {
		cleaned, inlineReasoning := stripThinkTags(content)
		content = cleaned
		if reasoning == "" {
			reasoning = inlineReasoning
		}
	}

	var toolCalls []FunctionCallData
	for _, tc := range msg.Get("tool_calls").Array() {
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(tc.Get("function.arguments").String()), &args)
		toolCalls = append(toolCalls, FunctionCallData{ID: tc.Get("id").String(), Name: tc.Get("function.name").String(), Args: args})
	}

	usage := &TokenUsage{InputTokens: res.Get("usage.prompt_tokens").Int(), OutputTokens: res.Get("usage.completion_tokens").Int()}
	return &ChatResponse{Message: content, Reasoning: reasoning, ToolCalls: toolCalls}, usage, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if strings.Contains(c.baseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", "https://octrafic.com")
		req.Header.Set("X-Title", "Octrafic")
	}
}

func (c *Client) buildRequestPayload(messages []Message, tools []Tool, stream bool) map[string]interface{} {
	payload := map[string]interface{}{
		"model":    c.model,
		"messages": c.convertMessages(messages),
		"stream":   stream,
	}

	if len(tools) > 0 {
		payload["tools"] = c.convertTools(tools)
	}

	if strings.HasPrefix(c.model, "o1") || strings.HasPrefix(c.model, "o3") {
		payload["reasoning_effort"] = "medium"
		payload["max_completion_tokens"] = 10000 
	}

	if stream {
		payload["stream_options"] = map[string]interface{}{"include_usage": true}
	}

	return payload
}

func (c *Client) convertMessages(messages []Message) []map[string]interface{} {
	var result []map[string]interface{}
	for _, m := range messages {
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}

		if m.FunctionResponse != nil {
			msg["role"] = "tool"
			msg["tool_call_id"] = m.FunctionResponse.ID
			resp, _ := json.Marshal(m.FunctionResponse.Response)
			msg["content"] = string(resp)
		}

		if len(m.FunctionCalls) > 0 {
			var tcs []map[string]interface{}
			for _, fc := range m.FunctionCalls {
				args, _ := json.Marshal(fc.Args)
				tcs = append(tcs, map[string]interface{}{
					"id":   fc.ID,
					"type": "function",
					"function": map[string]interface{}{"name": fc.Name, "arguments": string(args)},
				})
			}
			msg["tool_calls"] = tcs
		}

		result = append(result, msg)
	}
	return result
}

func (c *Client) convertTools(tools []Tool) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		tool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		}
		result = append(result, tool)
	}
	return result
}
