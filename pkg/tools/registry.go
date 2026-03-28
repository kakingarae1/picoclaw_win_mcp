package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kakingarae1/picoclaw/pkg/config"
)

type Tool struct {
	Name, Description, InputSchema string
	handler func(context.Context, string) (string, error)
}

type Registry struct {
	tools map[string]*Tool
	cfg   *config.Config
	ws    string
}

func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{tools: make(map[string]*Tool), cfg: cfg, ws: cfg.Agent.Workspace}
	r.add("read_file",   "Read a workspace file",
		`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`, r.read)
	r.add("write_file",  "Write a workspace file",
		`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`, r.write)
	r.add("list_files",  "List workspace files",
		`{"type":"object","properties":{"path":{"type":"string"}}}`, r.list)
	r.add("delete_file", "Delete a workspace file",
		`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`, r.del)
	if len(cfg.Security.AllowedCommands) > 0 {
		r.add("run_command", "Run an allowed shell command",
			`{"type":"object","properties":{"command":{"type":"string"},"args":{"type":"array","items":{"type":"string"}}},"required":["command"]}`, r.run)
	}
	return r
}

func (r *Registry) add(name, desc, schema string, fn func(context.Context, string) (string, error)) {
	r.tools[name] = &Tool{Name: name, Description: desc, InputSchema: schema, handler: fn}
}

func (r *Registry) All() []*Tool {
	out := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools { out = append(out, t) }
	return out
}

func (r *Registry) Execute(ctx context.Context, name, input string) (string, error) {
	t, ok := r.tools[name]
	if !ok { return "", fmt.Errorf("unknown tool: %s", name) }
	return t.handler(ctx, input)
}

func (r *Registry) safe(rel string) (string, error) {
	abs := filepath.Join(r.ws, filepath.Clean("/"+rel))
	if r.cfg.Security.RestrictWorkspace {
		wsAbs, _ := filepath.Abs(r.ws)
		if !strings.HasPrefix(abs, wsAbs+string(filepath.Separator)) && abs != wsAbs {
			return "", fmt.Errorf("path outside workspace: %s", rel)
		}
	}
	return abs, nil
}

func (r *Registry) read(_ context.Context, in string) (string, error) {
	var p struct{ Path string `json:"path"` }
	if err := json.Unmarshal([]byte(in), &p); err != nil { return "", err }
	abs, err := r.safe(p.Path)
	if err != nil { return "", err }
	d, err := os.ReadFile(abs)
	return string(d), err
}

func (r *Registry) write(_ context.Context, in string) (string, error) {
	var p struct{ Path, Content string }
	if err := json.Unmarshal([]byte(in), &p); err != nil { return "", err }
	abs, err := r.safe(p.Path)
	if err != nil { return "", err }
	_ = os.MkdirAll(filepath.Dir(abs), 0700)
	if err := os.WriteFile(abs, []byte(p.Content), 0600); err != nil { return "", err }
	return fmt.Sprintf("wrote %d bytes", len(p.Content)), nil
}

func (r *Registry) list(_ context.Context, in string) (string, error) {
	var p struct{ Path string `json:"path"` }
	_ = json.Unmarshal([]byte(in), &p)
	if p.Path == "" { p.Path = "." }
	abs, err := r.safe(p.Path)
	if err != nil { return "", err }
	entries, err := os.ReadDir(abs)
	if err != nil { return "", err }
	var lines []string
	for _, e := range entries {
		if info, _ := e.Info(); info != nil {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%s", e.Name(), info.Size(), info.ModTime().Format(time.RFC3339)))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (r *Registry) del(_ context.Context, in string) (string, error) {
	var p struct{ Path string `json:"path"` }
	if err := json.Unmarshal([]byte(in), &p); err != nil { return "", err }
	abs, err := r.safe(p.Path)
	if err != nil { return "", err }
	return "deleted", os.Remove(abs)
}

func (r *Registry) run(ctx context.Context, in string) (string, error) {
	var p struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(in), &p); err != nil { return "", err }
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(p.Command), ".exe"))
	for _, a := range r.cfg.Security.AllowedCommands {
		if strings.ToLower(a) == base {
			ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx2, p.Command, p.Args...)
			cmd.Dir = r.ws
			out, err := cmd.CombinedOutput()
			return string(out), err
		}
	}
	return "", fmt.Errorf("command %q not allowed", p.Command)
}
