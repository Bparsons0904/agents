package config

import (
	"os"
	"strconv"
)

// DebugConfig holds debug configuration
type DebugConfig struct {
	Enabled   bool   `toml:"enabled"`
	LogDir    string `toml:"log_dir"`
	Verbose   bool   `toml:"verbose"`
	MaxLogMB  int    `toml:"max_log_mb"`
}

// GetDebugConfig returns debug configuration from environment and defaults
func GetDebugConfig() DebugConfig {
	config := DebugConfig{
		Enabled:  false,
		LogDir:   "/tmp/agent-debug",
		Verbose:  false,
		MaxLogMB: 10,
	}
	
	// Check environment variables
	if enabled := os.Getenv("AGENT_DEBUG"); enabled == "true" || enabled == "1" {
		config.Enabled = true
	}
	
	if logDir := os.Getenv("AGENT_DEBUG_DIR"); logDir != "" {
		config.LogDir = logDir
	}
	
	if verbose := os.Getenv("AGENT_DEBUG_VERBOSE"); verbose == "true" || verbose == "1" {
		config.Verbose = true
	}
	
	if maxMB := os.Getenv("AGENT_DEBUG_MAX_MB"); maxMB != "" {
		if mb, err := strconv.Atoi(maxMB); err == nil {
			config.MaxLogMB = mb
		}
	}
	
	return config
}