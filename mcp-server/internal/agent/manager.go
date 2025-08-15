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
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are a Senior Engineering Manager responsible for planning and orchestrating software development.

**Current Task:** %s
**Project Type:** %s
**Working Directory:** %s

**Your Responsibilities:**
1. Analyze the requested feature and understand project context
2. Create a detailed implementation plan
3. Identify potential risks and dependencies  
4. Provide clear guidance for the implementation team
5. Make routing decisions for the workflow

**Current Git Status:**
%s

`, req.Description, req.ProjectType, req.WorkingDirectory, context.GitStatus))

	// Add project documentation if available
	if context.ClaudeMd != "" {
		prompt.WriteString(fmt.Sprintf(`**Project Instructions (CLAUDE.md):**
%s

`, context.ClaudeMd))
	}

	if context.AgentsMd != "" {
		prompt.WriteString(fmt.Sprintf(`**Agent Instructions (AGENTS.md):**
%s

`, context.AgentsMd))
	}

	// Add existing project files context
	if len(context.ExistingFiles) > 0 {
		prompt.WriteString("**Existing Project Files:**\n")
		for filename, content := range context.ExistingFiles {
			if len(content) > 500 {
				content = content[:500] + "... (truncated)"
			}
			prompt.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", filename, content))
		}
	}

	prompt.WriteString(`**Available Actions:**
- READ_FILE: Read existing code files to understand structure
- ANALYZE_STRUCTURE: Understand project layout and conventions
- CREATE_PLAN: Generate detailed implementation plan

**Guidelines:**
- Understand existing code patterns and conventions before planning
- Consider dependencies, testing requirements, and integration points
- Plan for quality assurance and code review steps
- Identify potential risks and mitigation strategies
- Create actionable steps for the implementation team

**Response Format:**
Please respond with a structured analysis and plan:

ANALYSIS:
- Project structure assessment
- Existing patterns and conventions identified
- Dependencies and integration points
- Risk assessment

IMPLEMENTATION_PLAN:
- Step-by-step implementation approach
- File changes required
- Testing strategy
- Quality assurance requirements

ACTION: READ_FILE (if needed)
PATH: path/to/file

Begin by analyzing the project and creating a comprehensive implementation plan.`)

	return prompt.String()
}

func (em *EngineeringManager) processManagerResponse(ctx context.Context, req ImplementFeatureRequest, llmResponse string, projectCtx *ProjectContext) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		NextSteps:        "",
	}

	// Parse any READ_FILE actions the manager might have requested
	actions := em.parseActions(llmResponse)

	for _, action := range actions {
		if action.Type == "READ_FILE" {
			content, err := em.tools.ReadFile(action.Path)
			if err != nil {
				// Don't fail the entire process for missing files
				continue
			}
			// Store the file content for potential use by other agents
			projectCtx.ExistingFiles[action.Path] = content
		}
	}

	// Analyze the response to determine success and next steps
	analysis := em.analyzeManagerResponse(llmResponse)
	
	if analysis.HasValidPlan {
		result.Success = true
		result.Message = "Implementation plan created successfully"
		result.NextSteps = analysis.NextSteps
	} else {
		result.Success = false
		result.Error = analysis.Issues
		result.NextSteps = "Plan needs revision"
	}

	return result, nil
}

type ManagerAnalysis struct {
	HasValidPlan bool
	NextSteps    string
	Issues       string
	RiskLevel    string
}

func (em *EngineeringManager) analyzeManagerResponse(response string) ManagerAnalysis {
	analysis := ManagerAnalysis{}
	
	// Look for key sections in the response
	hasAnalysis := strings.Contains(strings.ToLower(response), "analysis:")
	hasPlan := strings.Contains(strings.ToLower(response), "implementation_plan:")
	
	// Check for planning completeness
	planningKeywords := []string{
		"step", "file", "test", "implement", "create", "modify", "build",
	}
	
	keywordCount := 0
	lowerResponse := strings.ToLower(response)
	for _, keyword := range planningKeywords {
		if strings.Contains(lowerResponse, keyword) {
			keywordCount++
		}
	}

	// Check for risk indicators
	riskKeywords := []string{
		"risk", "concern", "dependency", "complex", "difficult", "challenge",
	}
	
	riskCount := 0
	for _, keyword := range riskKeywords {
		if strings.Contains(lowerResponse, keyword) {
			riskCount++
		}
	}

	if riskCount > 2 {
		analysis.RiskLevel = "high"
	} else if riskCount > 0 {
		analysis.RiskLevel = "medium"
	} else {
		analysis.RiskLevel = "low"
	}

	// Determine if plan is valid
	if hasAnalysis && hasPlan && keywordCount >= 3 {
		analysis.HasValidPlan = true
		analysis.NextSteps = "Plan approved. Ready for Senior Engineer implementation."
	} else {
		analysis.HasValidPlan = false
		
		var issues []string
		if !hasAnalysis {
			issues = append(issues, "Missing project analysis")
		}
		if !hasPlan {
			issues = append(issues, "Missing implementation plan")
		}
		if keywordCount < 3 {
			issues = append(issues, "Plan lacks sufficient detail")
		}
		
		analysis.Issues = strings.Join(issues, "; ")
		analysis.NextSteps = "Plan needs more detail and analysis"
	}

	return analysis
}


func (em *EngineeringManager) parseActions(response string) []Action {
	var actions []Action
	lines := strings.Split(response, "\n")
	
	var currentAction *Action
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "ACTION:") {
			// Save previous action
			if currentAction != nil {
				actions = append(actions, *currentAction)
			}
			
			// Start new action
			actionType := strings.TrimSpace(strings.TrimPrefix(line, "ACTION:"))
			currentAction = &Action{Type: actionType}
		} else if currentAction != nil {
			if strings.HasPrefix(line, "PATH:") {
				currentAction.Path = strings.TrimSpace(strings.TrimPrefix(line, "PATH:"))
			} else if strings.HasPrefix(line, "COMMAND:") {
				currentAction.Command = strings.TrimSpace(strings.TrimPrefix(line, "COMMAND:"))
			}
		}
	}
	
	// Save final action
	if currentAction != nil {
		actions = append(actions, *currentAction)
	}
	
	return actions
}