package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/kakingarae1/picoclaw/pkg/config"
)

type anthropicProvider struct {
	client *anthropic.Client
	model  string
	cfg    config.Provider
}

func newAnthropic(p config.Provider, key string) Provider {
	return &anthropicProvider{client: anthropic.NewClient(option.WithAPIKey(key)), model: p.Model, cfg: p}
}

func (a *anthropicProvider) Name() string { return a.cfg.Name }

func (a *anthropicProvider) Chat(ctx context.Context, msgs []Message, tools []ToolDefinition) (*Response, error) {
	var am []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case "user":
			am = append(am, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			am = append(am, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	var at []anthropic.ToolParam
	for _, t := range tools {
		var s interface{}
		_ = json.Unmarshal([]byte(t.InputSchema), &s)
		if s == nil {
			s = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		at = append(at, anthropic.ToolParam{
			Name: anthropic.String(t.Name), Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: s},
		})
	}
	params := anthropic.MessageNewParams{Model: anthropic.Model(a.model), MaxTokens: anthropic.Int(4096), Messages: am}
	if len(at) > 0 {
		params.Tools = at
	}
	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}
	out := &Response{StopReason: string(resp.StopReason)}
	for _, b := range resp.Content {
		switch v := b.AsAny().(type) {
		case anthropic.TextBlock:
			out.Content += v.Text
		case anthropic.ToolUseBlock:
			raw, _ := json.Marshal(v.Input)
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: v.ID, Name: v.Name, Input: string(raw)})
		}
	}
	return out, nil
}
