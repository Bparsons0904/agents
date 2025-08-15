package agent

import (
	"context"
	"fmt"
	"strings"
)

type SeniorEngineer struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
}

func NewSeniorEngineer(llmClient LLMClient, tools ToolSet, restrictions CommandRestrictions) *SeniorEngineer {
	return &SeniorEngineer{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
	}
}

func (se *SeniorEngineer) ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error) {
	// Step 1: Set working directory if specified
	if req.WorkingDirectory != "" {
		se.tools.SetWorkingDirectory(req.WorkingDirectory)
	}
	
	// Step 2: Analyze current project state
	gitStatus, err := se.tools.GetGitStatus()
	if err != nil {
		// Git status is optional - continue without it
		gitStatus = "No git repository detected or git error occurred"
	}

	// Step 3: Build system prompt with context
	prompt := se.buildSystemPrompt(req, gitStatus)

	// Step 4: Generate implementation plan from LLM
	response, err := se.llmClient.Generate(ctx, prompt)
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

	// Step 5: Parse and execute implementation
	return se.executeImplementation(ctx, req, response)
}

func (se *SeniorEngineer) buildSystemPrompt(req ImplementFeatureRequest, gitStatus string) string {
	return fmt.Sprintf(`You are a Senior Software Engineer focused on implementing high-quality code.

**Current Task:** %s
**Project Type:** %s
**Working Directory:** %s

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

**Guidelines:**
- Write clean, maintainable code
- Follow existing code patterns and conventions
- Include proper error handling
- Add minimal comments only for complex logic
- Ensure changes build without errors
- Use fail-fast approach - attempt once, report clearly on failure

**Response Format:**
Please respond with a structured plan using these action markers:

ACTION: READ_FILE
PATH: path/to/file

ACTION: WRITE_FILE
PATH: path/to/new/file
CONTENT:
` + "```" + `
file content here
` + "```" + `

ACTION: EXECUTE_COMMAND
COMMAND: build command here

Begin by analyzing the current project structure and implementing the requested feature.`, 
		req.Description, req.ProjectType, req.WorkingDirectory, gitStatus)
}

func (se *SeniorEngineer) executeImplementation(ctx context.Context, req ImplementFeatureRequest, llmResponse string) (*ImplementFeatureResponse, error) {
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

		case "EXECUTE_COMMAND":
			if err := se.restrictions.ValidateCommand(action.Command); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command validation failed: %v", err)
				return result, nil
			}

			output, err := se.tools.ExecuteCommand(action.Command)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command execution failed: %v", err)
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
		return "go build ./..."
	case ProjectTypeTypeScript:
		return "npm run build"
	case ProjectTypePython:
		return "python -m py_compile *.py"
	default:
		return ""
	}
}