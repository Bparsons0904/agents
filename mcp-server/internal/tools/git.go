package tools

import (
	"fmt"
	"os/exec"
	"strconv"
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

// GetDiff returns the full diff of current changes
func (g *GitOperations) GetDiff() (string, error) {
	cmd := exec.Command("git", "diff")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetDiffNameOnly returns only the names of changed files
func (g *GitOperations) GetDiffNameOnly() (string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetDiffCached returns staged changes
func (g *GitOperations) GetDiffCached() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetLog returns recent commit history
func (g *GitOperations) GetLog(limit int) (string, error) {
	if limit <= 0 {
		limit = 10 // default
	}
	
	cmd := exec.Command("git", "log", "--oneline", "-n", strconv.Itoa(limit))
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetLogOneFile returns commit history for a specific file
func (g *GitOperations) GetLogOneFile(filepath string, limit int) (string, error) {
	if limit <= 0 {
		limit = 10 // default
	}
	
	cmd := exec.Command("git", "log", "--oneline", "-n", strconv.Itoa(limit), "--", filepath)
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetShow returns details of a specific commit
func (g *GitOperations) GetShow(commitHash string) (string, error) {
	if commitHash == "" {
		return "", fmt.Errorf("commit hash is required")
	}
	
	cmd := exec.Command("git", "show", commitHash)
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// GetBranch returns current branch name
func (g *GitOperations) GetBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = g.workingDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// IsGitRepo checks if the directory is a git repository
func (g *GitOperations) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = g.workingDir
	
	err := cmd.Run()
	return err == nil
}