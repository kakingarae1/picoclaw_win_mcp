package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent     AgentConfig    `yaml:"agent"`
	Providers []Provider     `yaml:"providers"`
	MCP       MCPConfig      `yaml:"mcp"`
	Gateway   GatewayConfig  `yaml:"gateway"`
	Security  SecurityConfig `yaml:"security"`
}

type AgentConfig struct {
	DefaultProvider string  `yaml:"default_provider"`
	MaxTokens       int     `yaml:"max_tokens"`
	Temperature     float64 `yaml:"temperature"`
	Workspace       string  `yaml:"workspace"`
	MaxIterations   int     `yaml:"max_iterations"`
}

type Provider struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type MCPConfig struct {
	Enabled bool        `yaml:"enabled"`
	Servers []MCPServer `yaml:"servers"`
}

type MCPServer struct {
	Name      string            `yaml:"name"`
	URL       string            `yaml:"url"`
	Transport string            `yaml:"transport"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	Headers   map[string]string `yaml:"headers"`
	Enabled   bool              `yaml:"enabled"`
}

type GatewayConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	WebUI     bool   `yaml:"webui"`
	WebUIPort int    `yaml:"webui_port"`
}

type SecurityConfig struct {
	RestrictWorkspace    bool     `yaml:"restrict_workspace"`
	AllowedCommands      []string `yaml:"allowed_commands"`
	CronRequiresApproval bool     `yaml:"cron_requires_approval"`
	MCPLocalhostOnly     bool     `yaml:"mcp_localhost_only"`
	SingleInstance       bool     `yaml:"single_instance"`
}

func Defaults() *Config {
	ws := filepath.Join(os.Getenv("APPDATA"), "picoclaw", "workspace")
	return &Config{
		Agent: AgentConfig{DefaultProvider: "claude", MaxTokens: 4096, Temperature: 0.7, Workspace: ws, MaxIterations: 20},
		Providers: []Provider{
			{Name: "claude",     Type: "anthropic",    Model: "claude-sonnet-4-5"},
			{Name: "openai",     Type: "openai-compat", BaseURL: "https://api.openai.com/v1",    Model: "gpt-4o"},
			{Name: "openrouter", Type: "openai-compat", BaseURL: "https://openrouter.ai/api/v1", Model: "anthropic/claude-3.5-sonnet"},
			{Name: "ollama",     Type: "openai-compat", BaseURL: "http://localhost:11434/v1",    Model: "llama3.1:8b"},
		},
		MCP:     MCPConfig{Enabled: true, Servers: []MCPServer{}},
		Gateway: GatewayConfig{Host: "127.0.0.1", Port: 18790, WebUI: true, WebUIPort: 18800},
		Security: SecurityConfig{
			RestrictWorkspace: true, AllowedCommands: []string{"powershell", "cmd", "python", "git", "node"},
			CronRequiresApproval: true, MCPLocalhostOnly: true, SingleInstance: true,
		},
	}
}

func configPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "picoclaw", "config.yaml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("run 'picoclaw onboard' first")
		}
		return nil, err
	}
	cfg := Defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.Gateway.Host = "127.0.0.1"
	for _, s := range cfg.MCP.Servers {
		if s.URL == "" {
			continue
		}
		ok := false
		for _, p := range []string{"http://127.0.0.1", "http://localhost", "ws://127.0.0.1", "ws://localhost"} {
			if len(s.URL) >= len(p) && s.URL[:len(p)] == p {
				ok = true; break
			}
		}
		if !ok {
			return nil, fmt.Errorf("MCP server %q URL must be localhost", s.Name)
		}
	}
	return cfg, nil
}

func Onboard() error {
	path := configPath()
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	cfg := Defaults()
	_ = os.MkdirAll(cfg.Agent.Workspace, 0700)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		data, _ := yaml.Marshal(cfg)
		return os.WriteFile(path, data, 0600)
	}
	return nil
}
