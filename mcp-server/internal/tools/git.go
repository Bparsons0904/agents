package tools

import (
	"os/exec"
)

type GitOperations struct {
	workingDir string
}

func NewGitOperations(workingDir string) *GitOperations {
	return &GitOperations{
		workingDir: workingDir,
	}
}

func (g *GitOperations) GetStatus() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

func (g *GitOperations) GetDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

func (g *GitOperations) GetLog(limit int) (string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-n", "10")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}