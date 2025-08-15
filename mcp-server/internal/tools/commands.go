package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

type CommandValidator struct {
	allowed         []string
	blockedPatterns []string
	workingDir      string
}

func NewCommandValidator(allowed, blockedPatterns []string, workingDir string) *CommandValidator {
	return &CommandValidator{
		allowed:         allowed,
		blockedPatterns: blockedPatterns,
		workingDir:      workingDir,
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

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cv.workingDir
	output, err := cmd.CombinedOutput()

	return string(output), err
}