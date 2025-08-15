package agent

import (
	"context"
	"fmt"
	"log"
	"mcp-server/internal/config"
	"strings"
	"time"
)

type SeniorEngineer struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
	config       config.WorkflowAgentConfig // Agent-specific config
}

func NewSeniorEngineer(
	llmClient LLMClient,
	tools ToolSet,
	restrictions CommandRestrictions,
	cfg config.WorkflowAgentConfig,
) *SeniorEngineer {
	return &SeniorEngineer{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
		config:       cfg,
	}
}

func (se *SeniorEngineer) ImplementFeature(
	ctx context.Context,
	req ImplementFeatureRequest,
) (*ImplementFeatureResponse, error) {
	// Set per-agent timeout
	ctx, cancel := context.WithTimeout(
		ctx,
		time.Duration(se.config.PerAgentTimeoutMinutes)*time.Minute,
	)
	defer cancel()

	// Set working directory if specified
	if req.WorkingDirectory != "" {
		se.tools.SetWorkingDirectory(req.WorkingDirectory)
	}

	var lastError string
	var result *ImplementFeatureResponse
	var attempts int
	var currentErrorCategory string
	var sameErrorAttempts int
	maxAttempts := 8          // Increased for 14B model
	maxSameErrorAttempts := 3 // Reset when error type changes

	for attempts < maxAttempts {
		select {
		case <-ctx.Done():
			return &ImplementFeatureResponse{Success: false, Error: "Agent timed out"}, nil
		default:
		}

		attempts++

		// Analyze current project state
		gitStatus, err := se.tools.GetGitStatus()
		if err != nil {
			gitStatus = "No git repository detected or git error occurred"
		}

		// Build system prompt with context and last error
		prompt := se.buildSystemPrompt(req, gitStatus, lastError)

		// Generate implementation plan from LLM
		llmResponse, err := se.llmClient.Generate(ctx, prompt)
		if err != nil {
			return &ImplementFeatureResponse{
				Success: false,
				Error:   fmt.Sprintf("LLM generation failed: %v", err),
			}, nil
		}

		// Parse and execute implementation
		result, err = se.executeImplementation(ctx, req, llmResponse)
		if err != nil {
			return &ImplementFeatureResponse{
				Success: false,
				Error:   fmt.Sprintf("Error during implementation execution: %v", err),
			}, nil
		}

		// If successful, we are done
		if result.Success {
			return result, nil
		}

		// --- Handle Failure ---
		// Categorize the current error
		newErrorCategory := se.categorizeError(result.Error)
		
		// Check if we're dealing with a new type of error
		if newErrorCategory != currentErrorCategory {
			// New error type - reset same-error attempt counter
			currentErrorCategory = newErrorCategory
			sameErrorAttempts = 1
			log.Printf("Engineer: New error category '%s', resetting same-error counter", newErrorCategory)
		} else {
			// Same error type - increment counter
			sameErrorAttempts++
		}
		
		// Check if we're stuck on the same error type
		if sameErrorAttempts >= maxSameErrorAttempts {
			result.Error = fmt.Sprintf("Agent stuck on '%s' error after %d attempts (total attempts: %d): %s", 
				currentErrorCategory, sameErrorAttempts, attempts, result.Error)
			return result, nil
		}
		
		lastError = result.Error
		log.Printf("Engineer: Attempt %d/%d failed (%s error #%d): %s", 
			attempts, maxAttempts, currentErrorCategory, sameErrorAttempts, result.Error)
	}

	// If we've exhausted all attempts
	result.Error = fmt.Sprintf("Agent failed after %d attempts. Last error: %s", maxAttempts, result.Error)
	return result, nil
}

func (se *SeniorEngineer) buildSystemPrompt(
	req ImplementFeatureRequest,
	gitStatus, lastError string,
) string {
	correctionPrompt := ""
	if lastError != "" {
		correctionPrompt = fmt.Sprintf(`
**Previous Attempt Failed!**
Your last attempt failed with the following error. Analyze the error and the code you produced, then generate a new plan to fix it.

**Error:**
%s
`, lastError)
	}

	return fmt.Sprintf(
		`You are a Senior Software Engineer focused on implementing high-quality code.

**Current Task:** %s
**Project Type:** %s
**Working Directory:** %s
%s
**Current Git Status:**
%s

**Your Responsibilities:**
1. Analyze the requested feature and determine implementation approach
2. Create or modify files to implement the feature
3. Follow language-specific best practices and conventions
4. Ensure code builds successfully
5. Run basic tests to validate functionality

**Available Actions:**
- READ_FILE: Read existing code files
- WRITE_FILE: Create or modify files
- EXECUTE_COMMAND: Run build, test, and git commands
- GET_GIT_DIFF: Check current changes
- LIST_FILES: List files and directories in a path
- FIND_FILES: Search for files by name pattern

**Guidelines:**
- Write clean, maintainable code
- Follow existing code patterns and conventions
- Include proper error handling
- Add minimal comments only for complex logic
- Ensure changes build without errors
- **For Go projects: Remove unused imports, handle all declared variables**
- **If you get "imported and not used" errors, remove the unused import**
- **If you are unable to fix a build error after an attempt, or if you believe you cannot complete the task, respond with a single line: ACTION: GIVE_UP**

**Response Format:**
Please respond with a structured plan using these action markers:

ACTION: READ_FILE
PATH: path/to/file

ACTION: WRITE_FILE
PATH: path/to/new/file
CONTENT:
`+"```"+`
file content here
`+"```"+`

ACTION: EXECUTE_COMMAND
COMMAND: build command here

ACTION: LIST_FILES
PATH: directory/path

ACTION: FIND_FILES
PATTERN: filename_pattern
SEARCH_PATH: directory/to/search (optional)

IMPORTANT: You MUST start by exploring the project structure before attempting to read any files. 

First, use ACTION: LIST_FILES with PATH: . to see the project root, then explore subdirectories.
Use ACTION: FIND_FILES to locate specific file types (e.g., PATTERN: .go, PATTERN: handler, etc.).
Only after understanding the structure should you read existing files and implement changes.

Begin by analyzing the current project structure and implementing the requested feature.`,
		req.Description,
		req.ProjectType,
		req.WorkingDirectory,
		correctionPrompt,
		gitStatus,
	)
}

func (se *SeniorEngineer) executeImplementation(
	ctx context.Context,
	req ImplementFeatureRequest,
	llmResponse string,
) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		NextSteps:        "Ready for review and testing",
	}

	// Parse LLM response for actions
	actions := se.parseActions(llmResponse)

	for _, action := range actions {
		if action.Type == "GIVE_UP" {
			result.Success = false
			result.Error = "Agent decided to give up."
			return result, nil
		}

		switch action.Type {
		case "READ_FILE":
			// Just for context, don't need to store result
			_, err := se.tools.ReadFile(action.Path)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to read file %s: %v", action.Path, err)
				return result, nil
			}

		case "WRITE_FILE":
			err := se.tools.WriteFile(action.Path, action.Content)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to write file %s: %v", action.Path, err)
				return result, nil
			}
			result.FilesModified = append(result.FilesModified, action.Path)

		case "LIST_FILES":
			files, err := se.tools.ListFiles(action.Path)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to list files in %s: %v", action.Path, err)
				return result, nil
			}
			// Add results to build output for the engineer to see
			result.BuildOutput += fmt.Sprintf("Files in %s:\n", action.Path)
			for _, file := range files {
				result.BuildOutput += fmt.Sprintf("  %s\n", file)
			}

		case "FIND_FILES":
			searchPath := action.SearchPath
			if searchPath == "" {
				searchPath = "." // Default to current directory
			}
			files, err := se.tools.FindFiles(action.Pattern, searchPath)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to find files with pattern '%s' in %s: %v", action.Pattern, searchPath, err)
				return result, nil
			}
			// Add results to build output for the engineer to see
			result.BuildOutput += fmt.Sprintf("Files matching '%s' in %s:\n", action.Pattern, searchPath)
			for _, file := range files {
				result.BuildOutput += fmt.Sprintf("  %s\n", file)
			}

		case "EXECUTE_COMMAND":
			if err := se.restrictions.ValidateCommand(action.Command); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command validation failed: %v", err)
				return result, nil
			}

			output, err := se.tools.ExecuteCommand(action.Command)
			log.Printf("Engineer: EXECUTE_COMMAND result - Error: %v, Output: %s", err, output)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf(
					"Command execution failed: %v",
					err,
				)
				result.BuildOutput = output
				return result, nil
			}
			result.CommandsExecuted = append(result.CommandsExecuted, action.Command)
			result.BuildOutput += output + "\n"
		}
	}

	// Try to run a build command based on project type
	buildCommand := se.getBuildCommand(req.ProjectType)
	if buildCommand != "" {
		if err := se.restrictions.ValidateCommand(buildCommand); err == nil {
			output, err := se.tools.ExecuteCommand(buildCommand)
			log.Printf(
				"Engineer: Build Command (%s) result - Error: %v, Output: %s",
				buildCommand,
				err,
				output,
			)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Build failed: %v", err)
				result.BuildOutput += "\nBuild Output:\n" + output
				return result, nil
			}
			result.CommandsExecuted = append(result.CommandsExecuted, buildCommand)
			result.BuildOutput += "\nBuild Output:\n" + output
		}
	}

	if result.Success {
		result.Message = "Feature implemented successfully"
	}

	log.Printf(
		"Engineer: Final executeImplementation result - Success: %v, Error: %v",
		result.Success,
		result.Error,
	)
	return result, nil
}

func (se *SeniorEngineer) parseActions(response string) []Action {
	var actions []Action
	lines := strings.Split(response, "\n")

	var currentAction *Action
	var inContent bool
	var contentBuilder strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ACTION:") {
			// Save previous action
			if currentAction != nil {
				if inContent {
					currentAction.Content = strings.TrimSpace(contentBuilder.String())
				}
				actions = append(actions, *currentAction)
			}

			// Start new action
			actionType := strings.TrimSpace(strings.TrimPrefix(line, "ACTION:"))
			currentAction = &Action{Type: actionType}
			inContent = false
			contentBuilder.Reset()
		} else if currentAction != nil {
			if strings.HasPrefix(line, "PATH:") {
				currentAction.Path = strings.TrimSpace(strings.TrimPrefix(line, "PATH:"))
			} else if strings.HasPrefix(line, "COMMAND:") {
				currentAction.Command = strings.TrimSpace(strings.TrimPrefix(line, "COMMAND:"))
			} else if strings.HasPrefix(line, "PATTERN:") {
				currentAction.Pattern = strings.TrimSpace(strings.TrimPrefix(line, "PATTERN:"))
			} else if strings.HasPrefix(line, "SEARCH_PATH:") {
				currentAction.SearchPath = strings.TrimSpace(strings.TrimPrefix(line, "SEARCH_PATH:"))
			} else if strings.HasPrefix(line, "CONTENT:") {
				inContent = true
				contentBuilder.Reset()
			} else if inContent && !strings.HasPrefix(line, "```") {
				if contentBuilder.Len() > 0 {
					contentBuilder.WriteString("\n")
				}
				contentBuilder.WriteString(line)
			}
		}
	}

	// Save final action
	if currentAction != nil {
		if inContent {
			currentAction.Content = strings.TrimSpace(contentBuilder.String())
		}
		actions = append(actions, *currentAction)
	}

	return actions
}

func (se *SeniorEngineer) getBuildCommand(projectType ProjectType) string {
	switch projectType {
	case ProjectTypeGo:
		return "go build ."
	case ProjectTypeTypeScript:
		return "npm run build"
	case ProjectTypePython:
		return "python -m py_compile *.py"
	default:
		return ""
	}
}

// categorizeError categorizes errors into broad types to detect progress vs stuck patterns
func (se *SeniorEngineer) categorizeError(errorMsg string) string {
	errorLower := strings.ToLower(errorMsg)
	
	// Go-specific compilation errors
	if strings.Contains(errorLower, "imported and not used") {
		return "unused_import"
	}
	if strings.Contains(errorLower, "undefined:") || strings.Contains(errorLower, "not declared") {
		return "undefined_symbol"
	}
	if strings.Contains(errorLower, "syntax error") || strings.Contains(errorLower, "expected") {
		return "syntax_error"
	}
	if strings.Contains(errorLower, "type") && (strings.Contains(errorLower, "mismatch") || strings.Contains(errorLower, "cannot")) {
		return "type_error"
	}
	
	// Build and compilation
	if strings.Contains(errorLower, "build failed") || strings.Contains(errorLower, "compilation") {
		return "build_failure"
	}
	if strings.Contains(errorLower, "package") && strings.Contains(errorLower, "not found") {
		return "missing_package"
	}
	
	// File system and tooling
	if strings.Contains(errorLower, "no such file") || strings.Contains(errorLower, "file not found") {
		return "file_not_found"
	}
	if strings.Contains(errorLower, "permission") || strings.Contains(errorLower, "access denied") {
		return "permission_error"
	}
	if strings.Contains(errorLower, "command") && strings.Contains(errorLower, "not found") {
		return "command_error"
	}
	
	// Network and external
	if strings.Contains(errorLower, "connection") || strings.Contains(errorLower, "network") {
		return "network_error"
	}
	if strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "deadline") {
		return "timeout_error"
	}
	
	// Generic categories
	if strings.Contains(errorLower, "test") && strings.Contains(errorLower, "failed") {
		return "test_failure"
	}
	
	return "unknown_error"
}

// isStuckOnSameError checks if the engineer is stuck on the same or similar error (legacy method, kept for compatibility)
func (se *SeniorEngineer) isStuckOnSameError(currentError, lastError string) bool {
	if lastError == "" {
		return false
	}
	
	// Use the new categorization system
	return se.categorizeError(currentError) == se.categorizeError(lastError)
}

// DocumentTask for SeniorEngineer is a no-op
func (se *SeniorEngineer) DocumentTask(ctx context.Context, result *WorkflowResult) error {
	return nil
}

