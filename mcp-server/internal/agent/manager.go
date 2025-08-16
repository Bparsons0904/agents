package agent

import (
	"context"
	"fmt"
	"mcp-server/internal/debug"
	"strings"
	"time"
)

type EngineeringManager struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
	debugLogger  *debug.DebugLogger
}

func NewEngineeringManager(llmClient LLMClient, tools ToolSet, restrictions CommandRestrictions, debugLogger *debug.DebugLogger) *EngineeringManager {
	return &EngineeringManager{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
		debugLogger:  debugLogger,
	}
}

func (em *EngineeringManager) ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error) {
	// Debug: Log what the EM is thinking about
	em.debugLogger.LogThought(debug.AgentThought{
		Timestamp:   time.Now(),
		Agent:       "EngineeringManager",
		Phase:       "feature_analysis",
		Task:        req.Description,
		Context:     fmt.Sprintf("Working Directory: %s, Project Type: %s", req.WorkingDirectory, req.ProjectType),
		Thinking:    "I need to analyze this feature request, understand the project context, and create a detailed implementation plan for the Senior Engineer. Let me gather project information and examine existing patterns.",
		PlanOfAction: "1. Set working directory\n2. Gather project context (files, git status, patterns)\n3. Analyze feature requirements\n4. Create structured briefing for engineer\n5. Execute any necessary setup commands",
	})

	// Step 1: Set working directory if specified
	if req.WorkingDirectory != "" {
		em.tools.SetWorkingDirectory(req.WorkingDirectory)
		em.debugLogger.LogAction(debug.AgentAction{
			Timestamp:  time.Now(),
			Agent:      "EngineeringManager",
			ActionType: "set_working_directory",
			FilePath:   req.WorkingDirectory,
			Result:     "Working directory set",
			Success:    true,
		})
	}

	// Step 2: Gather comprehensive project context
	em.debugLogger.LogThought(debug.AgentThought{
		Timestamp:   time.Now(),
		Agent:       "EngineeringManager",
		Phase:       "context_gathering",
		Task:        "Project Analysis",
		Thinking:    "Now I need to understand the current project structure, examine existing files, check git status, and identify patterns the engineer should follow.",
		PlanOfAction: "Scan project files, analyze existing code patterns, check git history for context",
	})

	context, err := em.gatherProjectContext()
	if err != nil {
		em.debugLogger.LogError("EngineeringManager", "context_gathering", err, "Failed to gather project context")
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to gather project context: %v", err),
		}, nil
	}

	contextSize := len(context.GitStatus) + len(context.GitLog) + len(context.ClaudeMd) + len(context.AgentsMd) + len(context.ProjectStructure)
	for _, content := range context.ExistingFiles {
		contextSize += len(content)
	}
	em.debugLogger.LogAction(debug.AgentAction{
		Timestamp:  time.Now(),
		Agent:      "EngineeringManager",
		ActionType: "gather_context",
		Result:     fmt.Sprintf("Gathered project context: %d characters, %d files", contextSize, len(context.ExistingFiles)),
		Success:    true,
	})

	// Step 3: Build system prompt with all context
	em.debugLogger.LogThought(debug.AgentThought{
		Timestamp:   time.Now(),
		Agent:       "EngineeringManager",
		Phase:       "plan_creation",
		Task:        "Creating Implementation Plan",
		Thinking:    "Based on the project context and feature requirements, I need to create a detailed, structured plan that guides the Senior Engineer. This should include specific files to examine, implementation approach, potential issues, and success criteria.",
		PlanOfAction: "Generate comprehensive briefing with task breakdown, context, file guidance, and implementation strategy",
	})

	prompt := em.buildSystemPrompt(req, context)

	// Step 4: Generate implementation plan from LLM
	response, err := em.llmClient.Generate(ctx, prompt)
	if err != nil {
		em.debugLogger.LogError("EngineeringManager", "llm_generation", err, "LLM generation failed")
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

	em.debugLogger.LogAction(debug.AgentAction{
		Timestamp:  time.Now(),
		Agent:      "EngineeringManager",
		ActionType: "llm_generation",
		Content:    truncateString(response, 500),
		Result:     "Generated implementation plan",
		Success:    true,
	})

	// Step 5: Parse and validate the plan
	return em.processManagerResponse(ctx, req, response, context)
}

type ProjectContext struct {
	GitStatus        string
	GitLog           string
	ClaudeMd         string
	AgentsMd         string
	ProjectStructure string
	ExistingFiles    map[string]string
}

func (em *EngineeringManager) gatherProjectContext() (*ProjectContext, error) {
	ctx := &ProjectContext{
		ExistingFiles: make(map[string]string),
	}

	// Get git status
	if gitStatus, err := em.tools.GetGitStatus(); err == nil {
		ctx.GitStatus = gitStatus
	} else {
		ctx.GitStatus = "No git repository detected"
	}

	// Get git diff to understand current changes
	if gitDiff, err := em.tools.GetGitDiff(); err == nil {
		ctx.GitLog = gitDiff
	}

	// Read project documentation
	if claudeMd, err := em.tools.ReadFile("CLAUDE.md"); err == nil {
		ctx.ClaudeMd = claudeMd
	}

	// Try both locations for AGENTS.md (new structure first, then fallback)
	if agentsMd, err := em.tools.ReadFile("agents/AGENTS.md"); err == nil {
		ctx.AgentsMd = agentsMd
	} else if agentsMd, err := em.tools.ReadFile("AGENTS.md"); err == nil {
		ctx.AgentsMd = agentsMd
	}

	if projectStructure, err := em.tools.ReadFile("PROJECT-STRUCTURE.md"); err == nil {
		ctx.ProjectStructure = projectStructure
	}

	// Read common project files for context
	commonFiles := []string{
		"README.md", "go.mod", "package.json", "requirements.txt",
		"Dockerfile", "docker-compose.yml", ".gitignore",
	}

	for _, file := range commonFiles {
		if content, err := em.tools.ReadFile(file); err == nil {
			ctx.ExistingFiles[file] = content
		}
	}

	return ctx, nil
}

func (em *EngineeringManager) buildSystemPrompt(req ImplementFeatureRequest, context *ProjectContext) string {
	agentsMdContent := context.AgentsMd
	if agentsMdContent == "" {
		agentsMdContent = "No AGENTS.md file found. This file should be created to document project standards and learnings."
	}

	// Check if this is a feedback/replanning scenario
	isFeedbackScenario := strings.Contains(strings.ToLower(req.Description), "feedback") ||
		strings.Contains(strings.ToLower(req.Description), "failed") ||
		strings.Contains(strings.ToLower(req.Description), "error") ||
		strings.Contains(strings.ToLower(req.Description), "issue")

	if isFeedbackScenario {
		return fmt.Sprintf(`You are the Engineering Manager handling feedback from the development workflow.

**Your Role:**
1. **Problem Analysis:** Review the feedback/issue and determine root cause
2. **Project Cleanup:** Clean up any project conflicts or organizational issues
3. **Replanning:** Provide a revised, more specific task for the Senior Engineer
4. **Context Integration:** Use project knowledge to avoid known pitfalls

**Available Tools for Project Management:**
- EXECUTE_COMMAND: Clean up files, create directories, manage project structure
- WRITE_FILE: Create project setup files (go.mod, package.json, etc.)
- READ_FILE: Analyze existing project structure

**Common Project Issues & Solutions:**
- Package conflicts: Clean up conflicting files, create isolated directories
- Build failures: Initialize proper project structure (go mod init, npm init)
- File conflicts: Remove conflicting files, organize into proper structure
- Directory issues: Create proper project hierarchy

**Feedback/Issue:** %s

**Project Knowledge (from AGENTS.md):**
%s

**Project Structure:**
%s

**Current Git Status:**
%s

**Your Task:**
1. **FIRST:** Analyze if project cleanup is needed (package conflicts, file organization)
2. **THEN:** Create a revised structured brief for the Senior Engineer

If project cleanup is needed, use EXECUTE_COMMAND actions to:
- Remove conflicting files
- Create proper directory structure  
- Initialize project files (go.mod, etc.)
- Organize files into appropriate locations

**REVISED IMPLEMENTATION BRIEF FORMAT:**
Your TASK response must follow this structured format to provide clear guidance to the Senior Engineer:

TASK: [One clear sentence describing what to implement]
CONTEXT: [Key project patterns/architecture the engineer should know]
FILES_TO_EXAMINE: [Specific files to read for patterns/examples, comma-separated]
IMPLEMENTATION_APPROACH: [Revised technical approach based on feedback analysis]
POTENTIAL_ISSUES: [Known pitfalls or challenges to watch for, including lessons from the failure, comma-separated]
SUCCESS_CRITERIA: [How to verify the implementation works]

**Response Format:**
If cleanup needed:
ACTION: EXECUTE_COMMAND
COMMAND: [cleanup command]

ACTION: WRITE_FILE  
PATH: [project setup file]
CONTENT: [file content]

TASK: [Your revised structured implementation brief addressing the feedback]

If no cleanup needed:
TASK: [Your revised structured implementation brief addressing the feedback]`, req.Description, agentsMdContent, context.ProjectStructure, context.GitStatus)
	}

	// Initial briefing scenario
	return fmt.Sprintf(`You are the Engineering Manager briefing the development team.

**Your Role:**
1. **Project Setup:** Ensure proper project structure and organization
2. **Requirements Analysis:** Break down the user request into clear, actionable tasks
3. **Context Integration:** Incorporate relevant project knowledge and patterns
4. **Technical Guidance:** Provide high-level direction without micromanaging
5. **Documentation Maintenance:** Create/update PROJECT-STRUCTURE.md as projects evolve

**Available Tools for Project Management:**
- EXECUTE_COMMAND: Set up project structure, create directories, initialize projects
- WRITE_FILE: Create project setup files (go.mod, package.json, README, etc.)
- READ_FILE: Analyze existing project structure

**User Request:** %s

**Project Knowledge (from AGENTS.md):**
%s

**Project Structure:**
%s

**Current Project State:**
%s

**Your Task:**
1. **FIRST:** Determine if project setup is needed for this task
2. **THEN:** Create a structured implementation brief for the Senior Engineer

For new features, consider:
- Does this need a new subdirectory to avoid conflicts?
- Does this require project initialization (go mod init, npm init)?
- Should this be organized in a specific way?
- Should PROJECT-STRUCTURE.md be created/updated to help other agents understand the layout?

**IMPLEMENTATION BRIEF FORMAT:**
Your TASK response must follow this structured format to provide clear guidance to the Senior Engineer:

TASK: [One clear sentence describing what to implement]
CONTEXT: [Key project patterns/architecture the engineer should know]
FILES_TO_EXAMINE: [Specific files to read for patterns/examples, comma-separated]
IMPLEMENTATION_APPROACH: [Suggested technical approach based on project analysis]
POTENTIAL_ISSUES: [Known pitfalls or challenges to watch for, comma-separated]
SUCCESS_CRITERIA: [How to verify the implementation works]

**Response Format:**
If project setup needed:
ACTION: EXECUTE_COMMAND
COMMAND: [setup command - mkdir, go mod init, etc.]

ACTION: WRITE_FILE
PATH: [project file if needed]
CONTENT: [file content]

TASK: [Your structured implementation brief following the format above]

If no setup needed:
TASK: [Your structured implementation brief following the format above]

**Example Brief Format:**
TASK: Create a REST API endpoint for user authentication
CONTEXT: This project uses Go Fiber framework with middleware-based routing
FILES_TO_EXAMINE: handlers/auth.go, middleware/cors.go, main.go
IMPLEMENTATION_APPROACH: Create new handler in handlers/ directory, add route to main.go, follow existing error handling patterns
POTENTIAL_ISSUES: Ensure proper CORS configuration, validate input parameters, handle database connection errors
SUCCESS_CRITERIA: Endpoint responds to POST /auth/login with proper JSON response and status codes`, req.Description, agentsMdContent, context.ProjectStructure, context.GitStatus)
}

func (em *EngineeringManager) processManagerResponse(ctx context.Context, req ImplementFeatureRequest, llmResponse string, projectCtx *ProjectContext) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		Message:          "Project setup and briefing completed",
	}

	// If AGENTS.md doesn't exist, create it in the new location
	if projectCtx.AgentsMd == "" {
		initialContent := "# Agent Knowledge Base\n\nThis file is managed by the Engineering Manager agent to maintain context and learnings between tasks.\n"
		err := em.tools.WriteFile("agents/AGENTS.md", initialContent)
		if err != nil {
			// Log the error but don't fail the whole process
			fmt.Printf("Error creating agents/AGENTS.md: %v\n", err)
		} else {
			result.FilesModified = append(result.FilesModified, "agents/AGENTS.md")
		}
	}

	// Parse and execute any actions from the LLM response
	actions := em.parseActions(llmResponse)
	for _, action := range actions {
		switch action.Type {
		case "EXECUTE_COMMAND":
			if err := em.restrictions.ValidateCommand(action.Command); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command validation failed: %v", err)
				return result, nil
			}

			output, err := em.tools.ExecuteCommand(action.Command)
			result.CommandsExecuted = append(result.CommandsExecuted, action.Command)
			result.BuildOutput += fmt.Sprintf("EM Command: %s\nOutput: %s\n", action.Command, output)
			
			if err != nil {
				// Log but don't fail - some setup commands may fail if already done
				result.BuildOutput += fmt.Sprintf("Command warning: %v\n", err)
			}

		case "WRITE_FILE":
			err := em.tools.WriteFile(action.Path, action.Content)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to write setup file %s: %v", action.Path, err)
				return result, nil
			}
			result.FilesModified = append(result.FilesModified, action.Path)
		}
	}

	// Extract the task description from the LLM response
	taskDescription := em.extractTaskDescription(llmResponse)
	
	result.NextSteps = taskDescription // The orchestrator will use this as the input for the next agent.
	return result, nil
}

// Helper method to parse actions from LLM response (similar to engineer's parseActions)
func (em *EngineeringManager) parseActions(response string) []Action {
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

// Helper method to extract task description, handling both action-based and simple responses
func (em *EngineeringManager) extractTaskDescription(llmResponse string) string {
	lines := strings.Split(llmResponse, "\n")
	
	// Look for TASK: line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TASK:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "TASK:"))
		}
	}
	
	// If no TASK: found, treat the whole response as task description
	taskDescription := strings.TrimSpace(llmResponse)
	if strings.HasPrefix(taskDescription, "TASK:") {
		taskDescription = strings.TrimSpace(strings.TrimPrefix(taskDescription, "TASK:"))
	}
	
	return taskDescription
}

// DocumentTask is called at the end of a successful workflow to update the knowledge base.
func (em *EngineeringManager) DocumentTask(ctx context.Context, result *WorkflowResult) error {
	// 1. Read existing AGENTS.md (try new location first, then fallback)
	var currentKnowledge string
	var agentsFile string
	
	if knowledge, err := em.tools.ReadFile("agents/AGENTS.md"); err == nil {
		currentKnowledge = knowledge
		agentsFile = "agents/AGENTS.md"
	} else if knowledge, err := em.tools.ReadFile("AGENTS.md"); err == nil {
		currentKnowledge = knowledge
		agentsFile = "AGENTS.md"
	} else {
		// If it doesn't exist, start with a fresh slate in the new location
		currentKnowledge = "# Agent Knowledge Base\n\nThis file is managed by the Engineering Manager agent to maintain context and learnings between tasks.\n"
		agentsFile = "agents/AGENTS.md"
	}

	// 2. Build a prompt to ask the LLM to summarize and update the knowledge base
	prompt := em.buildDocumentationPrompt(result, currentKnowledge)

	// 3. Generate the updated knowledge base from the LLM
	updatedKnowledge, err := em.llmClient.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate documentation from LLM: %w", err)
	}

	// 4. Write the new content back to AGENTS.md
	return em.tools.WriteFile(agentsFile, updatedKnowledge)
}

// buildReplanningPrompt creates an enhanced replanning prompt based on specific engineer feedback
func (em *EngineeringManager) buildReplanningPrompt(engineerFeedback string, originalBrief string, context *ProjectContext) string {
	return fmt.Sprintf(`You are the Engineering Manager receiving feedback from your Senior Engineer.

**ORIGINAL BRIEF:** %s

**ENGINEER FEEDBACK/ERROR:** %s

**PROJECT CONTEXT:**
%s

**YOUR REPLANNING TASK:**
1. Analyze what went wrong with your original brief
2. Identify if this was a project setup issue, approach issue, or missing context
3. Provide a REVISED brief with better guidance

**If the issue was:**
- Missing files/setup: Use EXECUTE_COMMAND to fix project structure
- Wrong approach: Revise IMPLEMENTATION_APPROACH with better strategy  
- Missing context: Add more specific FILES_TO_EXAMINE and CONTEXT
- Pattern mismatch: Update approach to match existing project patterns
- Structure issue: Create necessary directories and organization

**REVISED IMPLEMENTATION BRIEF FORMAT:**
Your response must follow this structured format to provide clear guidance to the Senior Engineer:

TASK: [One clear sentence describing what to implement]
CONTEXT: [Enhanced project patterns/architecture the engineer should know]
FILES_TO_EXAMINE: [Specific files to read for patterns/examples, comma-separated]
IMPLEMENTATION_APPROACH: [Revised technical approach addressing the feedback]
POTENTIAL_ISSUES: [Known pitfalls including lessons from the failure, comma-separated]
SUCCESS_CRITERIA: [How to verify the implementation works]

**Response Format:**
If cleanup/setup needed:
ACTION: EXECUTE_COMMAND
COMMAND: [cleanup command]

ACTION: WRITE_FILE  
PATH: [project setup file]
CONTENT: [file content]

TASK: [Your revised structured implementation brief addressing the feedback]

If no cleanup needed:
TASK: [Your revised structured implementation brief addressing the feedback]

Provide a complete revised brief that addresses the specific feedback received.`, originalBrief, engineerFeedback, context.ProjectStructure)
}

func (em *EngineeringManager) buildDocumentationPrompt(result *WorkflowResult, currentKnowledge string) string {
	var summary strings.Builder
	summary.WriteString("**Workflow Summary:**\n")
	summary.WriteString(fmt.Sprintf("- Success: %v\n", result.Success))
	if !result.Success {
		summary.WriteString(fmt.Sprintf("- Failure Reason: %s\n", result.FailureReason))
	}
	summary.WriteString(fmt.Sprintf("- Files Modified: %s\n", strings.Join(result.FilesModified, ", ")))
	summary.WriteString("\n**Agent Contributions:**\n")
	for role, agentSummary := range result.AgentSummaries {
		summary.WriteString(fmt.Sprintf("- **%s**: %s (Success: %v)\n", role, agentSummary.TaskCompleted, agentSummary.Success))
	}

	return fmt.Sprintf(`You are the Engineering Manager, responsible for maintaining the team's collective knowledge.\n\n**Your Task:**\nUpdate the Agent Knowledge Base (` + "`AGENTS.md`" + `) with the results of the last workflow. \n- Integrate new learnings, architectural decisions, or coding patterns.\n- Do NOT remove existing valuable information unless it is explicitly replaced by a new standard.\n- Keep the document concise and well-organized.\n\n**Summary of Completed Workflow:**\n%s\n\n**Current Knowledge Base (AGENTS.md):**\n--- (start of file) ---\n%s\n--- (end of file) ---\n\n**Your Response:**\nRespond with ONLY the complete, updated content for ` + "`AGENTS.md`" + `.\n`,
		summary.String(), currentKnowledge)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

