package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || (len(rel) > 1 && rel[0] == '.' && rel[1] == '.') || strings.HasPrefix(rel, "../") {
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
	if err != nil || (len(rel) > 1 && rel[0] == '.' && rel[1] == '.') || strings.HasPrefix(rel, "../") {
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

// ListFiles lists all files and directories in the given path
func (fs *FileSystem) ListFiles(path string) ([]string, error) {
	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(fs.workingDir, path)
	}

	// Security check: ensure path is within working directory
	absWorkingDir, err := filepath.Abs(fs.workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	rel, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || (len(rel) > 1 && rel[0] == '.' && rel[1] == '.') || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("access denied: path outside working directory")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name = name + "/"
		}
		files = append(files, name)
	}

	return files, nil
}

// FindFiles recursively searches for files matching the pattern
func (fs *FileSystem) FindFiles(pattern string, searchPath string) ([]string, error) {
	if searchPath == "" {
		searchPath = fs.workingDir
	}
	
	// Make path absolute if relative
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(fs.workingDir, searchPath)
	}

	// Security check: ensure path is within working directory
	absWorkingDir, err := filepath.Abs(fs.workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve search path: %w", err)
	}

	rel, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || (len(rel) > 1 && rel[0] == '.' && rel[1] == '.') || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("access denied: path outside working directory")
	}

	var matches []string
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from working directory
		relPath, err := filepath.Rel(absWorkingDir, path)
		if err != nil {
			return err
		}

		// Check if filename matches pattern
		filename := filepath.Base(path)
		if strings.Contains(strings.ToLower(filename), strings.ToLower(pattern)) {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return matches, nil
}