package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kakingarae1/picoclaw/pkg/config"
)

type Tool struct{ Name, Description, InputSchema, ServerName string }
type CallResult struct{ Content string; IsError bool }

type rpcReq struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type server struct {
	cfg    config.MCPServer
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	httpCl *http.Client
}

type Client struct {
	servers []*server
	tools   []Tool
	mu      sync.RWMutex
}

func NewClient(cfg config.MCPConfig) (*Client, error) {
	c := &Client{}
	for _, sc := range cfg.Servers {
		if cfg.Enabled && sc.Enabled {
			c.servers = append(c.servers, &server{cfg: sc, httpCl: &http.Client{Timeout: 30 * time.Second}})
		}
	}
	return c, nil
}

func (c *Client) Start(ctx context.Context) error {
	for _, s := range c.servers {
		tools, err := s.init(ctx)
		if err != nil {
			fmt.Printf("[mcp] %q init failed: %v\n", s.cfg.Name, err)
			continue
		}
		c.mu.Lock()
		c.tools = append(c.tools, tools...)
		c.mu.Unlock()
	}
	return nil
}

func (c *Client) Tools() []Tool {
	c.mu.RLock(); defer c.mu.RUnlock()
	out := make([]Tool, len(c.tools)); copy(out, c.tools); return out
}

func (c *Client) Call(ctx context.Context, name, input string) (*CallResult, error) {
	c.mu.RLock()
	var tgt *server
	for _, t := range c.tools {
		if t.Name == name {
			for _, s := range c.servers {
				if s.cfg.Name == t.ServerName { tgt = s }
			}
		}
	}
	c.mu.RUnlock()
	if tgt == nil {
		return nil, fmt.Errorf("no server for tool %q", name)
	}
	return tgt.call(ctx, name, input)
}

func (s *server) init(ctx context.Context) ([]Tool, error) {
	if s.cfg.Transport == "stdio" {
		return s.initStdio(ctx)
	}
	return s.initHTTP(ctx)
}

func (s *server) initHTTP(ctx context.Context) ([]Tool, error) {
	raw, err := s.rpcHTTP(ctx, "tools/list", nil)
	if err != nil { return nil, err }
	return parseTools(raw, s.cfg.Name)
}

func (s *server) initStdio(ctx context.Context) ([]Tool, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	cmd := exec.CommandContext(ctx, s.cfg.Command, s.cfg.Args...)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil { return nil, err }
	s.cmd, s.stdin, s.stdout = cmd, stdin, bufio.NewReader(stdout)
	_ = s.write(rpcReq{JSONRPC: "2.0", ID: uuid.NewString(), Method: "initialize",
		Params: map[string]interface{}{"protocolVersion": "2024-11-05", "clientInfo": map[string]string{"name": "picoclaw", "version": "1.0"}, "capabilities": map[string]interface{}{}}})
	_, _ = s.read()
	_ = s.write(rpcReq{JSONRPC: "2.0", ID: uuid.NewString(), Method: "tools/list"})
	resp, err := s.read()
	if err != nil { return nil, err }
	return parseTools(resp.Result, s.cfg.Name)
}

func parseTools(raw json.RawMessage, srv string) ([]Tool, error) {
	var r struct {
		Tools []struct {
			Name, Description string
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &r); err != nil { return nil, err }
	var out []Tool
	for _, t := range r.Tools {
		out = append(out, Tool{Name: t.Name, Description: t.Description, InputSchema: string(t.InputSchema), ServerName: srv})
	}
	return out, nil
}

func (s *server) call(ctx context.Context, name, inputJSON string) (*CallResult, error) {
	var input interface{}
	_ = json.Unmarshal([]byte(inputJSON), &input)
	params := map[string]interface{}{"name": name, "arguments": input}
	var raw json.RawMessage
	var err error
	if s.cfg.Transport == "stdio" {
		s.mu.Lock()
		if e := s.write(rpcReq{JSONRPC: "2.0", ID: uuid.NewString(), Method: "tools/call", Params: params}); e == nil {
			if r, e2 := s.read(); e2 == nil { raw = r.Result } else { err = e2 }
		} else { err = e }
		s.mu.Unlock()
	} else {
		raw, err = s.rpcHTTP(ctx, "tools/call", params)
	}
	if err != nil { return &CallResult{Content: err.Error(), IsError: true}, nil }
	var res struct {
		Content []struct{ Text string `json:"text"` } `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil { return &CallResult{Content: string(raw)}, nil }
	var parts []string
	for _, c := range res.Content { parts = append(parts, c.Text) }
	return &CallResult{Content: strings.Join(parts, "\n"), IsError: res.IsError}, nil
}

func (s *server) rpcHTTP(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	body, _ := json.Marshal(rpcReq{JSONRPC: "2.0", ID: uuid.NewString(), Method: method, Params: params})
	req, _ := http.NewRequestWithContext(ctx, "POST", s.cfg.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.cfg.Headers { req.Header.Set(k, v) }
	resp, err := s.httpCl.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	var r rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil { return nil, err }
	if r.Error != nil { return nil, fmt.Errorf("MCP %d: %s", r.Error.Code, r.Error.Message) }
	return r.Result, nil
}

func (s *server) write(r rpcReq) error {
	d, _ := json.Marshal(r)
	_, err := fmt.Fprintf(s.stdin, "%s\n", d); return err
}

func (s *server) read() (*rpcResp, error) {
	line, err := s.stdout.ReadString('\n')
	if err != nil { return nil, err }
	var r rpcResp
	return &r, json.Unmarshal([]byte(strings.TrimSpace(line)), &r)
}
