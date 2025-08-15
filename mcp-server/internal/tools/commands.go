package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

type CommandValidator struct {
	allowed         []string
	blockedPatterns []string
}

func NewCommandValidator(allowed, blockedPatterns []string) *CommandValidator {
	return &CommandValidator{
		allowed:         allowed,
		blockedPatterns: blockedPatterns,
	}
}

func (cv *CommandValidator) IsAllowed(command string) bool {
	command = strings.TrimSpace(command)
	
	// Check blocked patterns first
	for _, pattern := range cv.blockedPatterns {
		if strings.Contains(command, pattern) {
			return false
		}
	}

	// Check if command starts with any allowed command
	for _, allowedCmd := range cv.allowed {
		if strings.HasPrefix(command, allowedCmd) {
			return true
		}
	}

	return false
}

func (cv *CommandValidator) ValidateCommand(command string) error {
	if !cv.IsAllowed(command) {
		return fmt.Errorf("command not allowed: %s", command)
	}
	return nil
}

func (cv *CommandValidator) ExecuteCommand(command string) (string, error) {
	if err := cv.ValidateCommand(command); err != nil {
		return "", err
	}

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	
	return string(output), err
}