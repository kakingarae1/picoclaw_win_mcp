package providers

import (
	"context"
	"fmt"

	"github.com/kakingarae1/picoclaw/pkg/config"
	"github.com/kakingarae1/picoclaw/pkg/secrets"
)

type Message        struct{ Role, Content string }
type ToolCall       struct{ ID, Name, Input string }
type ToolDefinition struct{ Name, Description, InputSchema string }

type Response struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
}

type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error)
	Name() string
}

func New(p config.Provider) (Provider, error) {
	apiKey, _ := secrets.Get(p.Name)
	if p.Name == "ollama" {
		apiKey = "ollama"
	}
	switch p.Type {
	case "anthropic":
		if apiKey == "" {
			return nil, fmt.Errorf("no key for %q — run: keytool set %s <key>", p.Name, p.Name)
		}
		return newAnthropic(p, apiKey), nil
	case "openai-compat":
		return newOpenAICompat(p, apiKey), nil
	}
	return nil, fmt.Errorf("unknown provider type %q", p.Type)
}

type Registry struct{ m map[string]Provider }

func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{m: make(map[string]Provider)}
	for _, pc := range cfg.Providers {
		if p, err := New(pc); err == nil {
			r.m[pc.Name] = p
		}
	}
	if len(r.m) == 0 {
		return nil, fmt.Errorf("no providers ready — run keytool set <provider> <key>")
	}
	return r, nil
}

func (r *Registry) Default(cfg *config.Config) (Provider, error) {
	p, ok := r.m[cfg.Agent.DefaultProvider]
	if !ok {
		return nil, fmt.Errorf("default provider %q not configured", cfg.Agent.DefaultProvider)
	}
	return p, nil
}
