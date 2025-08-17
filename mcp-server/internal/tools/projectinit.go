package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ProjectInitializer discovers and documents patterns from existing codebases
type ProjectInitializer struct {
	toolSet *ToolSet
}

func NewProjectInitializer(toolSet *ToolSet) *ProjectInitializer {
	return &ProjectInitializer{toolSet: toolSet}
}

// DiscoveredPattern represents a pattern found in the codebase
type DiscoveredPattern struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`        // handler, model, service, etc.
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Files       []string `json:"files"`
	Frequency   int      `json:"frequency"`
}

// ProjectType represents the type of project being analyzed
type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypeJavaScript ProjectType = "javascript" 
	ProjectTypeTypescript ProjectType = "typescript"
	ProjectTypePython     ProjectType = "python"
)

// ProjectAnalysis contains the complete analysis of a project
type ProjectAnalysis struct {
	ProjectType     ProjectType          `json:"project_type"`
	Language        string               `json:"language"`
	Framework       string               `json:"framework"`
	Patterns        []DiscoveredPattern  `json:"patterns"`
	Architecture    ArchitectureAnalysis `json:"architecture"`
	Dependencies    []string             `json:"dependencies"`
	TestingFramework string              `json:"testing_framework"`
}

type ArchitectureAnalysis struct {
	Style       string   `json:"style"`        // mvc, layered, microservice, etc.
	Directories []string `json:"directories"`  // main structural directories
	EntryPoints []string `json:"entry_points"` // main.go, app.js, etc.
}

// AnalyzeProject performs comprehensive project analysis
func (pi *ProjectInitializer) AnalyzeProject(ctx context.Context, projectPath string) (*ProjectAnalysis, error) {
	analysis := &ProjectAnalysis{
		Patterns: []DiscoveredPattern{},
	}

	// Set working directory
	pi.toolSet.SetWorkingDirectory(projectPath)

	// Detect project type and language
	if err := pi.detectProjectType(analysis); err != nil {
		return nil, fmt.Errorf("failed to detect project type: %w", err)
	}

	// Analyze architecture
	if err := pi.analyzeArchitecture(analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze architecture: %w", err)
	}

	// Discover patterns based on project type
	switch analysis.Language {
	case "go":
		if err := pi.discoverGoPatterns(analysis); err != nil {
			return nil, fmt.Errorf("failed to discover Go patterns: %w", err)
		}
	case "javascript", "typescript":
		if err := pi.discoverJSPatterns(analysis); err != nil {
			return nil, fmt.Errorf("failed to discover JS/TS patterns: %w", err)
		}
	case "python":
		if err := pi.discoverPythonPatterns(analysis); err != nil {
			return nil, fmt.Errorf("failed to discover Python patterns: %w", err)
		}
	}

	// Analyze dependencies
	if err := pi.analyzeDependencies(analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	// Detect testing framework
	if err := pi.detectTestingFramework(analysis); err != nil {
		return nil, fmt.Errorf("failed to detect testing framework: %w", err)
	}

	return analysis, nil
}

// detectProjectType identifies the programming language and framework
func (pi *ProjectInitializer) detectProjectType(analysis *ProjectAnalysis) error {
	// Check for Go
	if _, err := pi.toolSet.ReadFile("go.mod"); err == nil {
		analysis.Language = "go"
		analysis.ProjectType = ProjectTypeGo
		
		// Detect Go frameworks
		goMod, _ := pi.toolSet.ReadFile("go.mod")
		if strings.Contains(goMod, "github.com/gofiber/fiber") {
			analysis.Framework = "fiber"
		} else if strings.Contains(goMod, "github.com/gin-gonic/gin") {
			analysis.Framework = "gin"
		} else if strings.Contains(goMod, "github.com/gorilla/mux") {
			analysis.Framework = "gorilla"
		} else if strings.Contains(goMod, "net/http") {
			analysis.Framework = "stdlib"
		}
		return nil
	}

	// Check for Node.js
	if _, err := pi.toolSet.ReadFile("package.json"); err == nil {
		packageJSON, _ := pi.toolSet.ReadFile("package.json")
		if strings.Contains(packageJSON, "typescript") {
			analysis.Language = "typescript"
			analysis.ProjectType = ProjectTypeTypescript
		} else {
			analysis.Language = "javascript"
			analysis.ProjectType = ProjectTypeJavaScript
		}
		
		// Detect JS/TS frameworks
		if strings.Contains(packageJSON, "express") {
			analysis.Framework = "express"
		} else if strings.Contains(packageJSON, "fastify") {
			analysis.Framework = "fastify"
		} else if strings.Contains(packageJSON, "next") {
			analysis.Framework = "nextjs"
		} else if strings.Contains(packageJSON, "react") {
			analysis.Framework = "react"
		}
		return nil
	}

	// Check for Python
	if _, err := pi.toolSet.ReadFile("requirements.txt"); err == nil {
		analysis.Language = "python"
		analysis.ProjectType = ProjectTypePython
		
		requirements, _ := pi.toolSet.ReadFile("requirements.txt")
		if strings.Contains(requirements, "fastapi") {
			analysis.Framework = "fastapi"
		} else if strings.Contains(requirements, "flask") {
			analysis.Framework = "flask"
		} else if strings.Contains(requirements, "django") {
			analysis.Framework = "django"
		}
		return nil
	}

	// Check for Python pyproject.toml
	if _, err := pi.toolSet.ReadFile("pyproject.toml"); err == nil {
		analysis.Language = "python"
		analysis.ProjectType = ProjectTypePython
		return nil
	}

	return fmt.Errorf("unable to detect project type")
}

// analyzeArchitecture determines the project's architectural style
func (pi *ProjectInitializer) analyzeArchitecture(analysis *ProjectAnalysis) error {
	// Get directory structure
	output, err := pi.toolSet.ExecuteCommand("find . -type d -name '.*' -prune -o -type d -print")
	if err != nil {
		return err
	}

	dirs := strings.Split(output, "\n")
	analysis.Architecture.Directories = dirs

	// Detect architectural patterns
	hasHandlers := false
	hasServices := false
	hasModels := false
	hasControllers := false

	for _, dir := range dirs {
		dirLower := strings.ToLower(dir)
		if strings.Contains(dirLower, "handler") || strings.Contains(dirLower, "controller") {
			hasHandlers = true
			hasControllers = true
		}
		if strings.Contains(dirLower, "service") {
			hasServices = true
		}
		if strings.Contains(dirLower, "model") || strings.Contains(dirLower, "entity") {
			hasModels = true
		}
	}

	// Determine architecture style
	if hasHandlers && hasServices && hasModels {
		analysis.Architecture.Style = "layered"
	} else if hasControllers {
		analysis.Architecture.Style = "mvc"
	} else {
		analysis.Architecture.Style = "simple"
	}

	// Find entry points
	entryPoints := []string{}
	if analysis.Language == "go" {
		if _, err := pi.toolSet.ReadFile("main.go"); err == nil {
			entryPoints = append(entryPoints, "main.go")
		}
		if _, err := pi.toolSet.ReadFile("cmd/main.go"); err == nil {
			entryPoints = append(entryPoints, "cmd/main.go")
		}
	}
	analysis.Architecture.EntryPoints = entryPoints

	return nil
}

// discoverGoPatterns analyzes Go-specific patterns
func (pi *ProjectInitializer) discoverGoPatterns(analysis *ProjectAnalysis) error {
	// Find all Go files
	output, err := pi.toolSet.ExecuteCommand("find . -name '*.go' -type f")
	if err != nil {
		return err
	}

	goFiles := strings.Split(strings.TrimSpace(output), "\n")
	if len(goFiles) == 0 || goFiles[0] == "" {
		return nil
	}

	// Analyze patterns in Go files
	patterns := make(map[string]*DiscoveredPattern)

	for _, file := range goFiles {
		if strings.Contains(file, "vendor/") || strings.Contains(file, ".git/") {
			continue
		}

		content, err := pi.toolSet.ReadFile(strings.TrimPrefix(file, "./"))
		if err != nil {
			continue
		}

		// Discover handler patterns
		pi.discoverGoHandlerPatterns(content, file, patterns)
		
		// Discover struct patterns
		pi.discoverGoStructPatterns(content, file, patterns)
		
		// Discover interface patterns
		pi.discoverGoInterfacePatterns(content, file, patterns)
		
		// Discover error handling patterns
		pi.discoverGoErrorPatterns(content, file, patterns)
	}

	// Convert map to slice
	for _, pattern := range patterns {
		analysis.Patterns = append(analysis.Patterns, *pattern)
	}

	return nil
}

// discoverGoHandlerPatterns finds HTTP handler patterns
func (pi *ProjectInitializer) discoverGoHandlerPatterns(content, file string, patterns map[string]*DiscoveredPattern) {
	// Pattern for Fiber handlers
	fiberPattern := regexp.MustCompile(`func\s+(\w+)\s*\([^)]*\*fiber\.Ctx[^)]*\)\s*error`)
	matches := fiberPattern.FindAllStringSubmatch(content, -1)
	
	if len(matches) > 0 {
		patternKey := "fiber_handlers"
		if patterns[patternKey] == nil {
			patterns[patternKey] = &DiscoveredPattern{
				Name:        "Fiber HTTP Handlers",
				Type:        "handler",
				Description: "Standard Fiber HTTP handler functions",
				Examples:    []string{},
				Files:       []string{},
				Frequency:   0,
			}
		}
		
		for _, match := range matches {
			if len(match) > 1 {
				example := fmt.Sprintf("func %s(c *fiber.Ctx) error", match[1])
				patterns[patternKey].Examples = append(patterns[patternKey].Examples, example)
				patterns[patternKey].Frequency++
			}
		}
		patterns[patternKey].Files = append(patterns[patternKey].Files, file)
	}

	// Pattern for standard HTTP handlers
	httpPattern := regexp.MustCompile(`func\s+(\w+)\s*\([^)]*http\.ResponseWriter[^)]*\*http\.Request[^)]*\)`)
	matches = httpPattern.FindAllStringSubmatch(content, -1)
	
	if len(matches) > 0 {
		patternKey := "http_handlers"
		if patterns[patternKey] == nil {
			patterns[patternKey] = &DiscoveredPattern{
				Name:        "Standard HTTP Handlers",
				Type:        "handler",
				Description: "Standard library HTTP handler functions",
				Examples:    []string{},
				Files:       []string{},
				Frequency:   0,
			}
		}
		
		for _, match := range matches {
			if len(match) > 1 {
				example := fmt.Sprintf("func %s(w http.ResponseWriter, r *http.Request)", match[1])
				patterns[patternKey].Examples = append(patterns[patternKey].Examples, example)
				patterns[patternKey].Frequency++
			}
		}
		patterns[patternKey].Files = append(patterns[patternKey].Files, file)
	}
}

// discoverGoStructPatterns finds struct definition patterns
func (pi *ProjectInitializer) discoverGoStructPatterns(content, file string, patterns map[string]*DiscoveredPattern) {
	// Request/Response struct patterns
	structPattern := regexp.MustCompile(`type\s+(\w+(?:Request|Response|DTO|Model))\s+struct\s*{`)
	matches := structPattern.FindAllStringSubmatch(content, -1)
	
	if len(matches) > 0 {
		patternKey := "dto_structs"
		if patterns[patternKey] == nil {
			patterns[patternKey] = &DiscoveredPattern{
				Name:        "Data Transfer Objects",
				Type:        "model",
				Description: "Request/Response/DTO struct definitions",
				Examples:    []string{},
				Files:       []string{},
				Frequency:   0,
			}
		}
		
		for _, match := range matches {
			if len(match) > 1 {
				example := fmt.Sprintf("type %s struct", match[1])
				patterns[patternKey].Examples = append(patterns[patternKey].Examples, example)
				patterns[patternKey].Frequency++
			}
		}
		patterns[patternKey].Files = append(patterns[patternKey].Files, file)
	}
}

// discoverGoInterfacePatterns finds interface patterns
func (pi *ProjectInitializer) discoverGoInterfacePatterns(content, file string, patterns map[string]*DiscoveredPattern) {
	interfacePattern := regexp.MustCompile(`type\s+(\w+(?:Service|Repository|Client|Interface))\s+interface\s*{`)
	matches := interfacePattern.FindAllStringSubmatch(content, -1)
	
	if len(matches) > 0 {
		patternKey := "service_interfaces"
		if patterns[patternKey] == nil {
			patterns[patternKey] = &DiscoveredPattern{
				Name:        "Service Interfaces",
				Type:        "interface",
				Description: "Service and repository interface definitions",
				Examples:    []string{},
				Files:       []string{},
				Frequency:   0,
			}
		}
		
		for _, match := range matches {
			if len(match) > 1 {
				example := fmt.Sprintf("type %s interface", match[1])
				patterns[patternKey].Examples = append(patterns[patternKey].Examples, example)
				patterns[patternKey].Frequency++
			}
		}
		patterns[patternKey].Files = append(patterns[patternKey].Files, file)
	}
}

// discoverGoErrorPatterns finds error handling patterns
func (pi *ProjectInitializer) discoverGoErrorPatterns(content, file string, patterns map[string]*DiscoveredPattern) {
	// Check for error wrapping patterns
	if strings.Contains(content, "fmt.Errorf") || strings.Contains(content, "errors.Wrap") {
		patternKey := "error_wrapping"
		if patterns[patternKey] == nil {
			patterns[patternKey] = &DiscoveredPattern{
				Name:        "Error Wrapping",
				Type:        "error_handling",
				Description: "Error wrapping and context preservation patterns",
				Examples:    []string{"fmt.Errorf(\"failed to process: %w\", err)", "errors.Wrap(err, \"context\")"},
				Files:       []string{},
				Frequency:   0,
			}
		}
		patterns[patternKey].Files = append(patterns[patternKey].Files, file)
		patterns[patternKey].Frequency++
	}
}

// discoverJSPatterns analyzes JavaScript/TypeScript patterns
func (pi *ProjectInitializer) discoverJSPatterns(analysis *ProjectAnalysis) error {
	// Find all JS/TS files
	output, err := pi.toolSet.ExecuteCommand("find . -name '*.js' -o -name '*.ts' -o -name '*.jsx' -o -name '*.tsx' | grep -v node_modules")
	if err != nil {
		return err
	}

	_ = strings.Split(strings.TrimSpace(output), "\n")
	// TODO: Implement JS/TS pattern discovery
	
	return nil
}

// discoverPythonPatterns analyzes Python patterns
func (pi *ProjectInitializer) discoverPythonPatterns(analysis *ProjectAnalysis) error {
	// Find all Python files
	output, err := pi.toolSet.ExecuteCommand("find . -name '*.py' | grep -v __pycache__")
	if err != nil {
		return err
	}

	_ = strings.Split(strings.TrimSpace(output), "\n")
	// TODO: Implement Python pattern discovery
	
	return nil
}

// analyzeDependencies analyzes project dependencies
func (pi *ProjectInitializer) analyzeDependencies(analysis *ProjectAnalysis) error {
	switch analysis.Language {
	case "go":
		if content, err := pi.toolSet.ReadFile("go.mod"); err == nil {
			// Extract dependencies from go.mod
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "github.com/") || strings.HasPrefix(line, "golang.org/") {
					parts := strings.Fields(line)
					if len(parts) >= 1 {
						analysis.Dependencies = append(analysis.Dependencies, parts[0])
					}
				}
			}
		}
	case "javascript", "typescript":
		if content, err := pi.toolSet.ReadFile("package.json"); err == nil {
			// TODO: Parse package.json dependencies
			_ = content
		}
	case "python":
		if content, err := pi.toolSet.ReadFile("requirements.txt"); err == nil {
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					analysis.Dependencies = append(analysis.Dependencies, line)
				}
			}
		}
	}
	
	return nil
}

// detectTestingFramework identifies the testing framework used
func (pi *ProjectInitializer) detectTestingFramework(analysis *ProjectAnalysis) error {
	switch analysis.Language {
	case "go":
		// Check for test files
		output, err := pi.toolSet.ExecuteCommand("find . -name '*_test.go' | head -1")
		if err == nil && strings.TrimSpace(output) != "" {
			analysis.TestingFramework = "go test"
			
			// Check for testing libraries
			if content, err := pi.toolSet.ReadFile("go.mod"); err == nil {
				if strings.Contains(content, "github.com/stretchr/testify") {
					analysis.TestingFramework = "testify"
				} else if strings.Contains(content, "github.com/onsi/ginkgo") {
					analysis.TestingFramework = "ginkgo"
				}
			}
		}
	case "javascript", "typescript":
		if content, err := pi.toolSet.ReadFile("package.json"); err == nil {
			if strings.Contains(content, "jest") {
				analysis.TestingFramework = "jest"
			} else if strings.Contains(content, "mocha") {
				analysis.TestingFramework = "mocha"
			} else if strings.Contains(content, "vitest") {
				analysis.TestingFramework = "vitest"
			}
		}
	case "python":
		if content, err := pi.toolSet.ReadFile("requirements.txt"); err == nil {
			if strings.Contains(content, "pytest") {
				analysis.TestingFramework = "pytest"
			} else if strings.Contains(content, "unittest") {
				analysis.TestingFramework = "unittest"
			}
		}
	}
	
	return nil
}

// GenerateProjectDocumentation creates documentation based on discovered patterns
func (pi *ProjectInitializer) GenerateProjectDocumentation(analysis *ProjectAnalysis, outputPath string) error {
	// Note: We'll create the patterns directory implicitly when writing files

	// Generate project overview
	overview := pi.generateProjectOverview(analysis)
	if err := pi.toolSet.WriteFile(filepath.Join(outputPath, "PROJECT_PATTERNS.md"), overview); err != nil {
		return err
	}

	// Generate pattern-specific documentation
	patternsDir := filepath.Join(outputPath, "patterns")
	for _, pattern := range analysis.Patterns {
		content := pi.generatePatternDocumentation(pattern)
		filename := fmt.Sprintf("%s.md", strings.ToLower(strings.ReplaceAll(pattern.Type, " ", "_")))
		if err := pi.toolSet.WriteFile(filepath.Join(patternsDir, filename), content); err != nil {
			return err
		}
	}

	return nil
}

// generateProjectOverview creates a comprehensive project overview
func (pi *ProjectInitializer) generateProjectOverview(analysis *ProjectAnalysis) string {
	var sb strings.Builder
	
	sb.WriteString("# Project Patterns Documentation\n\n")
	sb.WriteString("## Project Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **Language**: %s\n", analysis.Language))
	sb.WriteString(fmt.Sprintf("- **Framework**: %s\n", analysis.Framework))
	sb.WriteString(fmt.Sprintf("- **Architecture**: %s\n", analysis.Architecture.Style))
	sb.WriteString(fmt.Sprintf("- **Testing Framework**: %s\n\n", analysis.TestingFramework))
	
	sb.WriteString("## Architecture\n\n")
	sb.WriteString(fmt.Sprintf("**Style**: %s\n\n", analysis.Architecture.Style))
	sb.WriteString("**Entry Points**:\n")
	for _, entry := range analysis.Architecture.EntryPoints {
		sb.WriteString(fmt.Sprintf("- %s\n", entry))
	}
	sb.WriteString("\n")
	
	sb.WriteString("## Discovered Patterns\n\n")
	for _, pattern := range analysis.Patterns {
		sb.WriteString(fmt.Sprintf("### %s\n", pattern.Name))
		sb.WriteString(fmt.Sprintf("- **Type**: %s\n", pattern.Type))
		sb.WriteString(fmt.Sprintf("- **Frequency**: %d occurrences\n", pattern.Frequency))
		sb.WriteString(fmt.Sprintf("- **Files**: %d files\n", len(pattern.Files)))
		sb.WriteString(fmt.Sprintf("- **Description**: %s\n\n", pattern.Description))
	}
	
	sb.WriteString("## Dependencies\n\n")
	for _, dep := range analysis.Dependencies {
		sb.WriteString(fmt.Sprintf("- %s\n", dep))
	}
	
	return sb.String()
}

// generatePatternDocumentation creates detailed documentation for a specific pattern
func (pi *ProjectInitializer) generatePatternDocumentation(pattern DiscoveredPattern) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# %s\n\n", pattern.Name))
	sb.WriteString(fmt.Sprintf("**Type**: %s\n\n", pattern.Type))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", pattern.Description))
	sb.WriteString(fmt.Sprintf("**Frequency**: %d occurrences in %d files\n\n", pattern.Frequency, len(pattern.Files)))
	
	if len(pattern.Examples) > 0 {
		sb.WriteString("## Examples\n\n")
		for i, example := range pattern.Examples {
			if i >= 5 { // Limit to first 5 examples
				sb.WriteString("...\n")
				break
			}
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", example))
		}
	}
	
	if len(pattern.Files) > 0 {
		sb.WriteString("## Files\n\n")
		for i, file := range pattern.Files {
			if i >= 10 { // Limit to first 10 files
				sb.WriteString("...\n")
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", file))
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}