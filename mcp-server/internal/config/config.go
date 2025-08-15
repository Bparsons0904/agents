package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Legacy single agent config - kept for backward compatibility
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
	PerAgentTimeoutMinutes int `toml:"per_agent_timeout_minutes"`
}

// New multi-agent workflow config
type WorkflowConfig struct {
	Workflow     WorkflowSection                `toml:"workflow"`
	Agents       map[string]WorkflowAgentConfig `toml:"agents"`
	Commands     CommandsSection                `toml:"commands"`
	Restrictions RestrictionsSection           `toml:"restrictions"`
}

type WorkflowSection struct {
	MaxTotalIterations int `toml:"max_total_iterations"`
	TimeoutMinutes     int `toml:"timeout_minutes"`
}

type WorkflowAgentConfig struct {
	Role          string   `toml:"role"`
	Model         string   `toml:"model"`
	MaxIterations int      `toml:"max_iterations"`
	PerAgentTimeoutMinutes int `toml:"per_agent_timeout_minutes"`
	Tools         []string `toml:"tools"`
}

type CommandsSection struct {
	Allowed []string `toml:"allowed"`
}

type RestrictionsSection struct {
	BlockedPatterns []string `toml:"blocked_patterns"`
}

// LoadConfig loads the legacy single-agent configuration
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

// LoadWorkflowConfig loads the multi-agent workflow configuration
func LoadWorkflowConfig(path string) (*WorkflowConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return getDefaultWorkflowConfig(), nil
	}

	var cfg WorkflowConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode workflow config file: %w", err)
	}

	// Validate configuration
	if err := cfg.validateWorkflow(); err != nil {
		return nil, fmt.Errorf("invalid workflow configuration: %w", err)
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

	if cfg.Agent.PerAgentTimeoutMinutes <= 0 {
		cfg.Agent.PerAgentTimeoutMinutes = 5 // default
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
			Model:     "qwen3:14b-q4_K_M",
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
		Model: "qwen3:14b-q4_K_M",
	}
}

func getDefaultWorkflowConfig() *WorkflowConfig {
	return &WorkflowConfig{
		Workflow: WorkflowSection{
			MaxTotalIterations: 7,
			TimeoutMinutes:     15,
		},
		Agents: map[string]WorkflowAgentConfig{
			"engineering_manager": {
				Role:          "engineering_manager",
				Model:         "qwen3:14b-q4_K_M",
				MaxIterations: 2,
				Tools:         []string{"read_file", "git_status", "git_log"},
			},
			"senior_engineer": {
				Role:          "senior_engineer",
				Model:         "qwen3:14b-q4_K_M",
				MaxIterations: 3,
				Tools:         []string{"read_file", "write_file", "execute_command", "git_diff"},
			},
			"senior_qa": {
				Role:          "senior_qa",
				Model:         "qwen3:14b-q4_K_M",
				MaxIterations: 2,
				Tools:         []string{"read_file", "write_file", "execute_command", "git_diff"},
			},
			"senior_tech_lead": {
				Role:          "senior_tech_lead",
				Model:         "qwen3:14b-q4_K_M",
				MaxIterations: 2,
				Tools:         []string{"read_file", "write_file", "execute_command", "git_diff"},
			},
		},
		Commands: CommandsSection{
			Allowed: []string{
				"go build", "go test", "go fmt", "go vet", "go mod tidy", "go run",
				"go build ./...", "go test ./...", "go mod download", "go mod init",
				"npm install", "npm run build", "npm test", "npm run dev",
				"npm ci", "yarn install", "yarn build", "yarn test",
				"npm run lint", "npm audit",
				"python -m pytest", "python -m pip install", "python setup.py",
				"pip install", "pytest", "python -m venv",
				"python -m flake8", "python -m black --check",
				"make", "make build", "make test", "make clean",
				"git add", "git status", "git diff", "git log --oneline -10",
				"git log --oneline", "git show", "git branch",
				"ls", "cat", "head", "tail", "find", "grep",
				"mkdir", "touch", "cp", "mv",
			},
		},
		Restrictions: RestrictionsSection{
			BlockedPatterns: []string{
				"sudo", "rm -rf", "chmod +x", "systemctl",
				"iptables", "mount", "cd /", "cat /etc/",
				"passwd", "usermod", "userdel", "groupmod",
				"service", "systemd", "crontab", "at",
				"wget", "curl http", "curl https", "ssh",
				"scp", "rsync", "dd", "fdisk", "mkfs",
				"chown", "chgrp", "umount", "kill -9",
			},
		},
	}
}

func (cfg *WorkflowConfig) validateWorkflow() error {
	if cfg.Workflow.MaxTotalIterations <= 0 {
		cfg.Workflow.MaxTotalIterations = 7 // default
	}

	if cfg.Workflow.TimeoutMinutes <= 0 {
		cfg.Workflow.TimeoutMinutes = 15 // default
	}

	if len(cfg.Agents) == 0 {
		return fmt.Errorf("at least one agent configuration is required")
	}

	// Validate each agent config
	for name, agentCfg := range cfg.Agents {
		if agentCfg.Role == "" {
			return fmt.Errorf("agent %s role is required", name)
		}

		if agentCfg.Model == "" {
			return fmt.Errorf("agent %s model is required", name)
		}

		if agentCfg.MaxIterations <= 0 {
			return fmt.Errorf("agent %s max_iterations must be positive", name)
		}

		if agentCfg.PerAgentTimeoutMinutes <= 0 {
			agentCfg.PerAgentTimeoutMinutes = 5 // default
			cfg.Agents[name] = agentCfg
		}
	}

	if len(cfg.Commands.Allowed) == 0 {
		return fmt.Errorf("at least one allowed command is required")
	}

	return nil
}