package anthropicprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

type (
	ToolCall               = protocoltypes.ToolCall
	FunctionCall           = protocoltypes.FunctionCall
	LLMResponse            = protocoltypes.LLMResponse
	UsageInfo              = protocoltypes.UsageInfo
	Message                = protocoltypes.Message
	ToolDefinition         = protocoltypes.ToolDefinition
	ToolFunctionDefinition = protocoltypes.ToolFunctionDefinition
)

const defaultBaseURL = "https://api.anthropic.com"

type Provider struct {
	client      *anthropic.Client
	tokenSource func() (string, error)
	baseURL     string
}

func NewProvider(token string) *Provider {
	return NewProviderWithBaseURL(token, "")
}

func NewProviderWithBaseURL(token, apiBase string) *Provider {
	baseURL := normalizeBaseURL(apiBase)
	client := anthropic.NewClient(
		option.WithAuthToken(token),
		option.WithBaseURL(baseURL),
	)
	return &Provider{
		client:  &client,
		baseURL: baseURL,
	}
}

func NewProviderWithClient(client *anthropic.Client) *Provider {
	return &Provider{
		client:  client,
		baseURL: defaultBaseURL,
	}
}

func NewProviderWithTokenSource(token string, tokenSource func() (string, error)) *Provider {
	return NewProviderWithTokenSourceAndBaseURL(token, tokenSource, "")
}

func NewProviderWithTokenSourceAndBaseURL(token string, tokenSource func() (string, error), apiBase string) *Provider {
	p := NewProviderWithBaseURL(token, apiBase)
	p.tokenSource = tokenSource
	return p
}

func (p *Provider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	var opts []option.RequestOption
	if p.tokenSource != nil {
		tok, err := p.tokenSource()
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		opts = append(opts, option.WithAuthToken(tok))
	}

	params, err := buildParams(messages, tools, model, options)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Messages.New(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("claude API call: %w", err)
	}

	return parseResponse(resp), nil
}

// ChatStream implements StreamingLLMProvider. It streams text chunks via
// onChunk and returns the complete LLMResponse (including tool calls) at the end.
func (p *Provider) ChatStream(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
	onChunk func(chunk string),
) (*LLMResponse, error) {
	var opts []option.RequestOption
	if p.tokenSource != nil {
		tok, err := p.tokenSource()
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		opts = append(opts, option.WithAuthToken(tok))
	}

	params, err := buildParams(messages, tools, model, options)
	if err != nil {
		return nil, err
	}

	stream := p.client.Messages.NewStreaming(ctx, params, opts...)
	defer stream.Close()

	var content strings.Builder
	var toolCalls []ToolCall
	var usage *UsageInfo
	finishReason := "stop"

	// Track tool_use blocks being assembled from streamed JSON deltas.
	type toolUseState struct {
		id        string
		name      string
		inputJSON strings.Builder
	}
	activeTools := make(map[int64]*toolUseState)

	for stream.Next() {
		evt := stream.Current()

		switch evt.Type {
		case "content_block_start":
			cb := evt.ContentBlock
			if cb.Type == "tool_use" {
				activeTools[evt.Index] = &toolUseState{
					id:   cb.ID,
					name: cb.Name,
				}
			}

		case "content_block_delta":
			delta := evt.Delta
			switch delta.Type {
			case "text_delta":
				if delta.Text != "" {
					content.WriteString(delta.Text)
					if onChunk != nil {
						onChunk(delta.Text)
					}
				}
			case "input_json_delta":
				if ts, ok := activeTools[evt.Index]; ok {
					ts.inputJSON.WriteString(delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if ts, ok := activeTools[evt.Index]; ok {
				var args map[string]any
				if err := json.Unmarshal([]byte(ts.inputJSON.String()), &args); err != nil {
					log.Printf("anthropic: failed to decode streamed tool input for %q: %v", ts.name, err)
					args = map[string]any{"raw": ts.inputJSON.String()}
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:        ts.id,
					Name:      ts.name,
					Arguments: args,
				})
				delete(activeTools, evt.Index)
			}

		case "message_delta":
			md := evt.AsMessageDelta()
			switch md.Delta.StopReason {
			case anthropic.StopReasonToolUse:
				finishReason = "tool_calls"
			case anthropic.StopReasonMaxTokens:
				finishReason = "length"
			case anthropic.StopReasonEndTurn:
				finishReason = "stop"
			}
			usage = &UsageInfo{
				CompletionTokens: int(md.Usage.OutputTokens),
			}

		case "message_start":
			ms := evt.AsMessageStart()
			if usage == nil {
				usage = &UsageInfo{}
			}
			usage.PromptTokens = int(ms.Message.Usage.InputTokens)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("claude streaming API call: %w", err)
	}

	if usage != nil {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return &LLMResponse{
		Content:      content.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func (p *Provider) GetDefaultModel() string {
	return "claude-sonnet-4.6"
}

func (p *Provider) BaseURL() string {
	return p.baseURL
}

func buildParams(
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (anthropic.MessageNewParams, error) {
	var system []anthropic.TextBlockParam
	var anthropicMessages []anthropic.MessageParam

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			system = append(system, anthropic.TextBlockParam{Text: msg.Content})
		case "user":
			if msg.ToolCallID != "" {
				anthropicMessages = append(anthropicMessages,
					anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false)),
				)
			} else {
				anthropicMessages = append(anthropicMessages,
					anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)),
				)
			}
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				if msg.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
				}
				anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(blocks...))
			} else {
				anthropicMessages = append(anthropicMessages,
					anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)),
				)
			}
		case "tool":
			anthropicMessages = append(anthropicMessages,
				anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false)),
			)
		}
	}

	maxTokens := int64(4096)
	if mt, ok := options["max_tokens"].(int); ok {
		maxTokens = int64(mt)
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		Messages:  anthropicMessages,
		MaxTokens: maxTokens,
	}

	if len(system) > 0 {
		params.System = system
	}

	if temp, ok := options["temperature"].(float64); ok {
		params.Temperature = anthropic.Float(temp)
	}

	if len(tools) > 0 {
		params.Tools = translateTools(tools)
	}

	return params, nil
}

func translateTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		tool := anthropic.ToolParam{
			Name: t.Function.Name,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.Function.Parameters["properties"],
			},
		}
		if desc := t.Function.Description; desc != "" {
			tool.Description = anthropic.String(desc)
		}
		if req, ok := t.Function.Parameters["required"].([]any); ok {
			required := make([]string, 0, len(req))
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
			tool.InputSchema.Required = required
		}
		result = append(result, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return result
}

func parseResponse(resp *anthropic.Message) *LLMResponse {
	var content string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			tb := block.AsText()
			content += tb.Text
		case "tool_use":
			tu := block.AsToolUse()
			var args map[string]any
			if err := json.Unmarshal(tu.Input, &args); err != nil {
				log.Printf("anthropic: failed to decode tool call input for %q: %v", tu.Name, err)
				args = map[string]any{"raw": string(tu.Input)}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        tu.ID,
				Name:      tu.Name,
				Arguments: args,
			})
		}
	}

	finishReason := "stop"
	switch resp.StopReason {
	case anthropic.StopReasonToolUse:
		finishReason = "tool_calls"
	case anthropic.StopReasonMaxTokens:
		finishReason = "length"
	case anthropic.StopReasonEndTurn:
		finishReason = "stop"
	}

	return &LLMResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage: &UsageInfo{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}

func normalizeBaseURL(apiBase string) string {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		return defaultBaseURL
	}

	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	if base == "" {
		return defaultBaseURL
	}

	return base
}
