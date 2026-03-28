package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/kakingarae1/picoclaw/pkg/config"
)

type openAICompatProvider struct {
	client *openai.Client
	model  string
	cfg    config.Provider
}

func newOpenAICompat(p config.Provider, key string) Provider {
	opts := []option.RequestOption{option.WithAPIKey(key)}
	if p.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(p.BaseURL))
	}
	return &openAICompatProvider{client: openai.NewClient(opts...), model: p.Model, cfg: p}
}

func (o *openAICompatProvider) Name() string { return o.cfg.Name }

func (o *openAICompatProvider) Chat(ctx context.Context, msgs []Message, tools []ToolDefinition) (*Response, error) {
	var om []openai.ChatCompletionMessageParamUnion
	for _, m := range msgs {
		switch m.Role {
		case "system":
			om = append(om, openai.SystemMessage(m.Content))
		case "user":
			om = append(om, openai.UserMessage(m.Content))
		case "assistant":
			om = append(om, openai.AssistantMessage(m.Content))
		}
	}
	var ot []openai.ChatCompletionToolParam
	for _, t := range tools {
		var s interface{}
		_ = json.Unmarshal([]byte(t.InputSchema), &s)
		if s == nil {
			s = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		ot = append(ot, openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{Name: t.Name, Description: openai.String(t.Description), Parameters: s},
		})
	}
	params := openai.ChatCompletionNewParams{Model: openai.ChatModel(o.model), Messages: om}
	if len(ot) > 0 {
		params.Tools = ot
	}
	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai-compat (%s): %w", o.cfg.Name, err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from %s", o.cfg.Name)
	}
	c := resp.Choices[0]
	out := &Response{Content: c.Message.Content, StopReason: string(c.FinishReason)}
	for _, tc := range c.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: tc.Function.Arguments})
	}
	return out, nil
}
