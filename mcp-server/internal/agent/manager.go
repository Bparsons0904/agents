package agent

import (
	"context"
	"fmt"
	"strings"
)

type EngineeringManager struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
}

func NewEngineeringManager(llmClient LLMClient, tools ToolSet, restrictions CommandRestrictions) *EngineeringManager {
	return &EngineeringManager{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
	}
}

func (em *EngineeringManager) ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error) {
	// Step 1: Set working directory if specified
	if req.WorkingDirectory != "" {
		em.tools.SetWorkingDirectory(req.WorkingDirectory)
	}

	// Step 2: Gather comprehensive project context
	context, err := em.gatherProjectContext()
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to gather project context: %v", err),
		}, nil
	}

	// Step 3: Build system prompt with all context
	prompt := em.buildSystemPrompt(req, context)

	// Step 4: Generate implementation plan from LLM
	response, err := em.llmClient.Generate(ctx, prompt)
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

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

	if agentsMd, err := em.tools.ReadFile("AGENTS.md"); err == nil {
		ctx.AgentsMd = agentsMd
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
	// This function will now generate a prompt focused on briefing, not deep planning.
	// The orchestrator will provide the final summary for the documentation phase.

	agentsMdContent := context.AgentsMd
	if agentsMdContent == "" {
		agentsMdContent = "No AGENTS.md file found. This file should be created to document project standards and learnings."
	}

	return fmt.Sprintf(`You are the Engineering Manager, a facilitator for the software development team.

**Your Role:**
1.  **Briefing:** Synthesize the user's request with existing project knowledge into a clear task for the Senior Engineer.
2.  **Organizing:** Ensure the engineer has all relevant context from past work.

**User Request:** %s

**Project Knowledge (from AGENTS.md):**
%s

**Your Task:**
Based on the user request and the project knowledge, write a concise task description for the Senior Engineer. 
Focus on clarity and providing actionable context. Do not create a technical plan. 

**Response Format:**
Respond with only the task description for the engineer. Start with "TASK:".

TASK: [Your concise task description for the engineer]`, req.Description, agentsMdContent)
}

func (em *EngineeringManager) processManagerResponse(ctx context.Context, req ImplementFeatureRequest, llmResponse string, projectCtx *ProjectContext) (*ImplementFeatureResponse, error) {
	// The EM's job is to produce the next prompt for the engineer.
	// We extract the task description and place it in the 'NextSteps' field for the orchestrator.

	// If AGENTS.md doesn't exist, create it.
	if projectCtx.AgentsMd == "" {
		initialContent := "# Agent Knowledge Base\n\nThis file is managed by the Engineering Manager agent to maintain context and learnings between tasks.\n"
		err := em.tools.WriteFile("AGENTS.md", initialContent)
		if err != nil {
			// Log the error but don't fail the whole process
			fmt.Printf("Error creating AGENTS.md: %v\n", err)
		}
	}

	// Extract the task description from the LLM response.
	taskDescription := strings.TrimSpace(llmResponse)
	if strings.HasPrefix(taskDescription, "TASK:") {
		taskDescription = strings.TrimSpace(strings.TrimPrefix(taskDescription, "TASK:"))
	}

	return &ImplementFeatureResponse{
		Success:   true,
		Message:   "Briefing for engineer created.",
		NextSteps: taskDescription, // The orchestrator will use this as the input for the next agent.
	}, nil
}

// DocumentTask is called at the end of a successful workflow to update the knowledge base.
func (em *EngineeringManager) DocumentTask(ctx context.Context, result *WorkflowResult) error {
	// 1. Read existing AGENTS.md
	currentKnowledge, err := em.tools.ReadFile("AGENTS.md")
	if err != nil {
		// If it doesn't exist, start with a fresh slate.
		currentKnowledge = "# Agent Knowledge Base\n\nThis file is managed by the Engineering Manager agent to maintain context and learnings between tasks.\n"
	}

	// 2. Build a prompt to ask the LLM to summarize and update the knowledge base
	prompt := em.buildDocumentationPrompt(result, currentKnowledge)

	// 3. Generate the updated knowledge base from the LLM
	updatedKnowledge, err := em.llmClient.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate documentation from LLM: %w", err)
	}

	// 4. Write the new content back to AGENTS.md
	return em.tools.WriteFile("AGENTS.md", updatedKnowledge)
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

