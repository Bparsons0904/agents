package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileSystem struct {
	workingDir string
}

func NewFileSystem(workingDir string) *FileSystem {
	return &FileSystem{
		workingDir: workingDir,
	}
}

func (fs *FileSystem) ReadFile(path string) (string, error) {
	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(fs.workingDir, path)
	}

	// Security check: ensure path is within working directory
	absWorkingDir, err := filepath.Abs(fs.workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || len(rel) > 0 && rel[0] == '.' && rel[1] == '.' {
		return "", fmt.Errorf("access denied: path outside working directory")
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

func (fs *FileSystem) WriteFile(path, content string) error {
	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(fs.workingDir, path)
	}

	// Security check: ensure path is within working directory
	absWorkingDir, err := filepath.Abs(fs.workingDir)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || len(rel) > 0 && rel[0] == '.' && rel[1] == '.' {
		return fmt.Errorf("access denied: path outside working directory")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}