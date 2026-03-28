package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kakingarae1/picoclaw/pkg/config"
	"github.com/kakingarae1/picoclaw/pkg/mcp"
	"github.com/kakingarae1/picoclaw/pkg/providers"
	"github.com/kakingarae1/picoclaw/pkg/tools"
)

const sysPrompt = `You are PicoClaw, a helpful AI assistant running on Windows.
Use tools to perform actions — never pretend. Stay in the workspace. Be concise.`

type Agent struct {
	cfg      *config.Config
	provider providers.Provider
	mcp      *mcp.Client
	builtin  *tools.Registry
	history  []providers.Message
}

func New(cfg *config.Config) *Agent { return &Agent{cfg: cfg} }

func (a *Agent) init(ctx context.Context) error {
	if a.provider != nil { return nil }
	reg, err := providers.NewRegistry(a.cfg)
	if err != nil { return err }
	p, err := reg.Default(a.cfg)
	if err != nil { return err }
	a.provider = p
	mc, err := mcp.NewClient(a.cfg.MCP)
	if err != nil { return err }
	_ = mc.Start(ctx)
	a.mcp = mc
	a.builtin = tools.NewRegistry(a.cfg)
	return nil
}

func (a *Agent) Chat(ctx context.Context, msg string) (string, error) {
	if err := a.init(ctx); err != nil { return "", err }
	a.history = append(a.history, providers.Message{Role: "user", Content: msg})
	for i := 0; i < a.cfg.Agent.MaxIterations; i++ {
		resp, err := a.provider.Chat(ctx, a.buildMsgs(), a.allTools())
		if err != nil { return "", err }
		if len(resp.ToolCalls) == 0 {
			a.history = append(a.history, providers.Message{Role: "assistant", Content: resp.Content})
			return resp.Content, nil
		}
		tcJSON, _ := json.Marshal(resp.ToolCalls)
		a.history = append(a.history, providers.Message{Role: "assistant", Content: resp.Content + "\n[tool_calls]:" + string(tcJSON)})
		for _, tc := range resp.ToolCalls {
			res, err := a.exec(ctx, tc)
			if err != nil { res = "error: " + err.Error() }
			a.history = append(a.history, providers.Message{Role: "user", Content: fmt.Sprintf("[tool_result id=%s]: %s", tc.ID, res)})
		}
	}
	return "", fmt.Errorf("max iterations reached")
}

func (a *Agent) Interactive(ctx context.Context) {
	if err := a.init(ctx); err != nil { fmt.Fprintln(os.Stderr, err); return }
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("PicoClaw ready. Type 'exit' to quit.")
	for {
		fmt.Print("> ")
		if !sc.Scan() { break }
		line := strings.TrimSpace(sc.Text())
		if line == "" { continue }
		if line == "exit" || line == "quit" { break }
		resp, err := a.Chat(ctx, line)
		if err != nil { fmt.Fprintln(os.Stderr, "Error:", err); continue }
		fmt.Println(resp)
	}
}

func (a *Agent) Reset() { a.history = nil }

func (a *Agent) buildMsgs() []providers.Message {
	return append([]providers.Message{{Role: "system", Content: sysPrompt}}, a.history...)
}

func (a *Agent) allTools() []providers.ToolDefinition {
	var defs []providers.ToolDefinition
	for _, t := range a.builtin.All() {
		defs = append(defs, providers.ToolDefinition{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	for _, t := range a.mcp.Tools() {
		defs = append(defs, providers.ToolDefinition{
			Name:        "mcp__" + t.ServerName + "__" + t.Name,
			Description: fmt.Sprintf("[MCP:%s] %s", t.ServerName, t.Description),
			InputSchema: t.InputSchema,
		})
	}
	return defs
}

func (a *Agent) exec(ctx context.Context, tc providers.ToolCall) (string, error) {
	if strings.HasPrefix(tc.Name, "mcp__") {
		parts := strings.SplitN(tc.Name, "__", 3)
		if len(parts) != 3 { return "", fmt.Errorf("bad MCP tool name: %s", tc.Name) }
		r, err := a.mcp.Call(ctx, parts[2], tc.Input)
		if err != nil { return "", err }
		if r.IsError { return "", fmt.Errorf("MCP: %s", r.Content) }
		return r.Content, nil
	}
	return a.builtin.Execute(ctx, tc.Name, tc.Input)
}
