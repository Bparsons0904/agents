package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type AgentConfig struct {
	Agent        AgentSection        `toml:"agent"`
	Commands     CommandsSection     `toml:"commands"`
	Restrictions RestrictionsSection `toml:"restrictions"`
	Model        string              `toml:"-"` // Set from Agent.Model
}

type AgentSection struct {
	Role      string `toml:"role"`
	Model     string `toml:"model"`
	MaxTokens int    `toml:"max_tokens"`
}

type CommandsSection struct {
	Allowed []string `toml:"allowed"`
}

type RestrictionsSection struct {
	BlockedPatterns []string `toml:"blocked_patterns"`
}

func LoadConfig(path string) (*AgentConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return getDefaultConfig(), nil
	}

	var cfg AgentConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Set model from agent section
	cfg.Model = cfg.Agent.Model

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (cfg *AgentConfig) validate() error {
	if cfg.Agent.Role == "" {
		return fmt.Errorf("agent role is required")
	}

	if cfg.Agent.Model == "" {
		return fmt.Errorf("agent model is required")
	}

	if cfg.Agent.MaxTokens <= 0 {
		cfg.Agent.MaxTokens = 4000 // default
	}

	if len(cfg.Commands.Allowed) == 0 {
		return fmt.Errorf("at least one allowed command is required")
	}

	return nil
}

func getDefaultConfig() *AgentConfig {
	return &AgentConfig{
		Agent: AgentSection{
			Role:      "senior_engineer",
			Model:     "qwen3:14b",
			MaxTokens: 4000,
		},
		Commands: CommandsSection{
			Allowed: []string{
				"go build", "go test", "go fmt", "go mod tidy", "go run",
				"npm install", "npm run build", "npm test", "npm run dev",
				"python -m pytest", "python -m pip install", "python setup.py",
				"make", "make build", "make test", "make clean",
				"git add", "git status", "git diff", "git log --oneline -10",
			},
		},
		Restrictions: RestrictionsSection{
			BlockedPatterns: []string{
				"sudo", "rm -rf", "chmod +x", "systemctl",
				"iptables", "mount", "cd /", "cat /etc/",
			},
		},
		Model: "qwen3:14b",
	}
}