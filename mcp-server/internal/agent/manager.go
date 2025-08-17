package agent

import (
	"context"
	"fmt"
	"mcp-server/internal/debug"
	"strings"
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
	// Set working directory if specified
	if req.WorkingDirectory != "" {
		em.tools.SetWorkingDirectory(req.WorkingDirectory)
	}

	// Get minimal project context
	context, err := em.gatherProjectContext()
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to gather project context: %v", err),
		}, nil
	}

	// Generate simple task for engineer
	prompt := em.buildSystemPrompt(req, context)
	response, err := em.llmClient.Generate(ctx, prompt)
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

	// Execute any setup commands and pass task to engineer
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

	// Just get basic project info - let the engineer discover the rest
	if projectStructure, err := em.tools.ReadFile("PROJECT-STRUCTURE.md"); err == nil {
		ctx.ProjectStructure = projectStructure
	} else {
		// List current directory to understand project state
		if files, err := em.tools.ListFiles("."); err == nil {
			ctx.ProjectStructure = fmt.Sprintf("Current directory contents: %v", files)
		} else {
			ctx.ProjectStructure = "Empty or new project directory"
		}
	}

	return ctx, nil
}

func (em *EngineeringManager) buildSystemPrompt(req ImplementFeatureRequest, context *ProjectContext) string {
	// Check if this is a feedback scenario
	isFeedbackScenario := strings.Contains(strings.ToLower(req.Description), "feedback") ||
		strings.Contains(strings.ToLower(req.Description), "failed") ||
		strings.Contains(strings.ToLower(req.Description), "error") ||
		strings.Contains(strings.ToLower(req.Description), "issue")

	if isFeedbackScenario {
		return fmt.Sprintf(`You are the Engineering Manager handling engineer feedback.

**Your Job:** Give the engineer a clearer, simpler task based on the feedback.

**Feedback:** %s

**Response Format:**
TASK: [Tell the engineer exactly what to build in one simple sentence. The engineer will handle any setup needed.]

Keep it simple. The engineer will figure out the implementation details and any setup needed.`, req.Description)
	}

	// Normal scenario - just tell the engineer what to build
	return fmt.Sprintf(`You are the Engineering Manager giving a task to your Senior Engineer.

**User Request:** %s

**Your Job:** Tell the engineer exactly what to build

**Response Format:**
TASK: [Tell the engineer exactly what to build in one simple sentence. The engineer will handle any setup needed.]

Keep it simple. The engineer will figure out the implementation details and any project setup.`, req.Description)
}

func (em *EngineeringManager) processManagerResponse(ctx context.Context, req ImplementFeatureRequest, llmResponse string, projectCtx *ProjectContext) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		Message:          "Task assigned to engineer",
	}

	// Extract the task description and pass it to the engineer
	taskDescription := em.extractTaskDescription(llmResponse)
	result.NextSteps = taskDescription
	return result, nil
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

