package agent

import (
	"context"
	"fmt"
	"strings"
)

type SeniorTechLead struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
}

func NewSeniorTechLead(llmClient LLMClient, tools ToolSet, restrictions CommandRestrictions) *SeniorTechLead {
	return &SeniorTechLead{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
	}
}

func (tl *SeniorTechLead) ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error) {
	// Step 1: Set working directory if specified
	if req.WorkingDirectory != "" {
		tl.tools.SetWorkingDirectory(req.WorkingDirectory)
	}

	// Step 2: Analyze the complete implementation and test coverage
	reviewContext, err := tl.analyzeCompleteWork()
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to analyze work for review: %v", err),
		}, nil
	}

	// Step 3: Build system prompt with context
	prompt := tl.buildSystemPrompt(req, reviewContext)

	// Step 4: Generate quality review from LLM
	response, err := tl.llmClient.Generate(ctx, prompt)
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

	// Step 5: Execute quality assurance actions
	return tl.executeQualityReview(ctx, req, response)
}

type ReviewContext struct {
	GitDiff         string
	AllChangedFiles []string
	FileContents    map[string]string
	TestFiles       []string
	ProjectType     ProjectType
	QualityTools    []string
}

func (tl *SeniorTechLead) analyzeCompleteWork() (*ReviewContext, error) {
	ctx := &ReviewContext{
		FileContents: make(map[string]string),
	}

	// Get git diff to see all changes
	gitDiff, err := tl.tools.GetGitDiff()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %v", err)
	}
	ctx.GitDiff = gitDiff

	// Parse all changed files
	ctx.AllChangedFiles = tl.parseAllChangedFiles(gitDiff)

	// Read all changed files for review
	for _, file := range ctx.AllChangedFiles {
		if content, err := tl.tools.ReadFile(file); err == nil {
			ctx.FileContents[file] = content
		}
	}

	// Identify test files
	ctx.TestFiles = tl.identifyTestFiles(ctx.AllChangedFiles)

	// Determine available quality tools
	ctx.QualityTools = tl.detectQualityTools()

	return ctx, nil
}

func (tl *SeniorTechLead) parseAllChangedFiles(gitDiff string) []string {
	var files []string
	lines := strings.Split(gitDiff, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Parse git diff output for file changes
		if strings.HasPrefix(line, "modified:") || strings.HasPrefix(line, "new file:") || strings.HasPrefix(line, "deleted:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				files = append(files, parts[len(parts)-1])
			}
		}
	}

	// If no files detected from git diff, scan for common patterns
	if len(files) == 0 {
		commonFiles := []string{
			"main.go", "server.go", "handler.go", "service.go", "api.go", "types.go",
			"main_test.go", "server_test.go", "handler_test.go",
			"index.js", "app.js", "server.js", "api.js", "types.js",
			"index.test.js", "app.test.js", "server.test.js",
			"main.py", "app.py", "server.py", "api.py", "models.py",
			"test_main.py", "test_app.py", "test_server.py",
		}
		
		for _, file := range commonFiles {
			if _, err := tl.tools.ReadFile(file); err == nil {
				files = append(files, file)
			}
		}
	}

	return files
}

func (tl *SeniorTechLead) identifyTestFiles(files []string) []string {
	var testFiles []string
	
	testPatterns := []string{
		"_test.go", ".test.js", ".test.ts", ".spec.js", ".spec.ts",
		"test_", "_test.py", "Test.java", "Tests.cs",
	}
	
	for _, file := range files {
		for _, pattern := range testPatterns {
			if strings.Contains(strings.ToLower(file), strings.ToLower(pattern)) {
				testFiles = append(testFiles, file)
				break
			}
		}
	}
	
	return testFiles
}

func (tl *SeniorTechLead) detectQualityTools() []string {
	var tools []string
	
	// Check for Go tools
	if _, err := tl.tools.ReadFile("go.mod"); err == nil {
		tools = append(tools, "go fmt", "go vet", "go mod tidy")
	}
	
	// Check for Node.js tools
	if _, err := tl.tools.ReadFile("package.json"); err == nil {
		tools = append(tools, "npm run lint", "npm audit")
	}
	
	// Check for Python tools
	if _, err := tl.tools.ReadFile("requirements.txt"); err == nil {
		tools = append(tools, "python -m flake8", "python -m black --check")
	}
	
	return tools
}

func (tl *SeniorTechLead) buildSystemPrompt(req ImplementFeatureRequest, ctx *ReviewContext) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are a Senior Tech Lead responsible for comprehensive code quality review and final approval.

**Current Task:** Review and approve feature: %s
**Project Type:** %s

**Complete Implementation Review:**
The following files were changed during implementation:
`, req.Description, req.ProjectType))

	// Show all changed files with content
	for filename, content := range ctx.FileContents {
		if len(content) > 1000 {
			content = content[:1000] + "... (truncated)"
		}
		prompt.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", filename, content))
	}

	if len(ctx.TestFiles) > 0 {
		prompt.WriteString(fmt.Sprintf("\n**Test Files Created:**\n%s\n", strings.Join(ctx.TestFiles, ", ")))
	}

	if ctx.GitDiff != "" {
		prompt.WriteString(fmt.Sprintf("\n**Git Diff Summary:**\n%s\n", ctx.GitDiff))
	}

	if len(ctx.QualityTools) > 0 {
		prompt.WriteString(fmt.Sprintf("\n**Available Quality Tools:**\n%s\n", strings.Join(ctx.QualityTools, ", ")))
	}

	prompt.WriteString(`
**Your Responsibilities:**
1. Conduct comprehensive code review for quality, security, and maintainability
2. Run linting, formatting, and static analysis tools
3. Validate architecture and design patterns
4. Check for security vulnerabilities and best practices
5. Ensure code follows project conventions and standards
6. Verify dependencies are secure and up-to-date
7. Make final approval decision for production readiness

**Code Review Criteria:**
- **Code Quality:** Clean, readable, maintainable code
- **Security:** No security vulnerabilities or sensitive data exposure
- **Performance:** Efficient algorithms and resource usage
- **Architecture:** Proper separation of concerns and design patterns
- **Testing:** Adequate test coverage and quality
- **Dependencies:** Secure, up-to-date, and necessary dependencies
- **Documentation:** Code is self-documenting with necessary comments
- **Standards:** Follows project coding standards and conventions

**Available Actions:**
- READ_FILE: Read additional files for context
- WRITE_FILE: Fix critical issues or add documentation
- EXECUTE_COMMAND: Run linting, formatting, and quality tools
- GET_GIT_DIFF: Analyze specific changes

**Response Format:**
Please respond with structured quality review:

QUALITY_REVIEW:
- Code quality assessment
- Security analysis
- Performance considerations
- Architecture evaluation
- Testing adequacy
- Dependency security
- Standards compliance

ACTION: EXECUTE_COMMAND
COMMAND: quality tool command

ACTION: WRITE_FILE (if fixes needed)
PATH: path/to/file
CONTENT:
` + "```" + `
fixed code here
` + "```" + `

FINAL_DECISION: [APPROVED/NEEDS_REVISION]
REASONING: Detailed reasoning for the decision

**Quality Standards:**
- Zero tolerance for security vulnerabilities
- Code must build without warnings
- All linting rules must pass
- No unused imports or variables
- Proper error handling throughout
- No hardcoded secrets or sensitive data

Begin by conducting a comprehensive quality review.`)

	return prompt.String()
}

func (tl *SeniorTechLead) executeQualityReview(ctx context.Context, req ImplementFeatureRequest, llmResponse string) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		NextSteps:        "Quality review complete - ready for deployment",
	}

	// Parse LLM response for actions
	actions := tl.parseActions(llmResponse)

	// Execute quality tools and fixes
	for _, action := range actions {
		switch action.Type {
		case "READ_FILE":
			// Just for context, don't need to store result
			_, err := tl.tools.ReadFile(action.Path)
			if err != nil {
				// Don't fail for missing files during review
				continue
			}

		case "WRITE_FILE":
			err := tl.tools.WriteFile(action.Path, action.Content)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to write fix file %s: %v", action.Path, err)
				return result, nil
			}
			result.FilesModified = append(result.FilesModified, action.Path)

		case "EXECUTE_COMMAND":
			if err := tl.restrictions.ValidateCommand(action.Command); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command validation failed: %v", err)
				return result, nil
			}

			output, err := tl.tools.ExecuteCommand(action.Command)
			result.CommandsExecuted = append(result.CommandsExecuted, action.Command)
			result.BuildOutput += fmt.Sprintf("\n=== %s ===\n%s", action.Command, output)
			
			if err != nil {
				// Analyze quality tool output
				qualityAnalysis := tl.analyzeQualityOutput(action.Command, output, err.Error())
				if qualityAnalysis.HasCriticalIssues {
					result.Success = false
					result.Error = fmt.Sprintf("Quality check failed: %s", qualityAnalysis.Issues)
					result.NextSteps = "Critical issues must be fixed before approval"
					return result, nil
				} else if qualityAnalysis.HasWarnings {
					result.BuildOutput += "\nWarnings detected: " + qualityAnalysis.Issues + "\n"
				}
			}
		}
	}

	// Run standard quality tools if not already executed
	qualityTools := tl.getQualityTools(req.ProjectType)
	for _, tool := range qualityTools {
		if !tl.commandAlreadyExecuted(result.CommandsExecuted, tool) {
			if err := tl.restrictions.ValidateCommand(tool); err == nil {
				output, err := tl.tools.ExecuteCommand(tool)
				result.CommandsExecuted = append(result.CommandsExecuted, tool)
				result.BuildOutput += fmt.Sprintf("\n=== %s ===\n%s", tool, output)
				
				if err != nil {
					qualityAnalysis := tl.analyzeQualityOutput(tool, output, err.Error())
					if qualityAnalysis.HasCriticalIssues {
						result.Success = false
						result.Error = fmt.Sprintf("Quality check failed: %s", qualityAnalysis.Issues)
						result.NextSteps = "Critical issues must be fixed before approval"
						return result, nil
					}
				}
			}
		}
	}

	// Final analysis of tech lead review
	finalDecision := tl.analyzeFinalDecision(result, llmResponse)
	
	if finalDecision.IsApproved {
		result.Success = true
		result.Message = "Code review passed - implementation approved for production"
		result.NextSteps = "Ready for deployment"
	} else {
		result.Success = false
		result.Error = finalDecision.Issues
		result.NextSteps = "Address quality concerns before re-review"
	}

	return result, nil
}

type QualityAnalysis struct {
	HasCriticalIssues bool
	HasWarnings      bool
	Issues           string
}

func (tl *SeniorTechLead) analyzeQualityOutput(command, output, errorText string) QualityAnalysis {
	analysis := QualityAnalysis{}
	
	combinedText := strings.ToLower(output + " " + errorText)
	
	// Critical issues that block approval
	criticalPatterns := []string{
		"security", "vulnerability", "sql injection", "xss",
		"hardcoded password", "hardcoded secret", "api key",
		"syntax error", "compile error", "build failed",
		"fatal", "critical", "severe",
	}
	
	for _, pattern := range criticalPatterns {
		if strings.Contains(combinedText, pattern) {
			analysis.HasCriticalIssues = true
			analysis.Issues += fmt.Sprintf("Critical issue in %s: %s. ", command, pattern)
		}
	}
	
	// Warnings that should be noted but don't block
	warningPatterns := []string{
		"warning", "unused", "deprecated", "inefficient",
		"complexity", "style", "convention", "lint",
	}
	
	for _, pattern := range warningPatterns {
		if strings.Contains(combinedText, pattern) {
			analysis.HasWarnings = true
			analysis.Issues += fmt.Sprintf("Warning in %s: %s. ", command, pattern)
		}
	}
	
	return analysis
}

type FinalDecision struct {
	IsApproved bool
	Issues     string
}

func (tl *SeniorTechLead) analyzeFinalDecision(result *ImplementFeatureResponse, llmResponse string) FinalDecision {
	decision := FinalDecision{}
	
	lowerResponse := strings.ToLower(llmResponse)
	
	// Look for explicit approval/rejection
	if strings.Contains(lowerResponse, "final_decision: approved") {
		decision.IsApproved = true
	} else if strings.Contains(lowerResponse, "final_decision: needs_revision") {
		decision.IsApproved = false
	} else {
		// Analyze overall quality indicators
		qualityIndicators := []string{
			"quality", "standards", "security", "performance",
			"architecture", "maintainable", "clean", "solid",
		}
		
		positiveCount := 0
		for _, indicator := range qualityIndicators {
			if strings.Contains(lowerResponse, "good "+indicator) || 
			   strings.Contains(lowerResponse, indicator+" is good") ||
			   strings.Contains(lowerResponse, "excellent "+indicator) {
				positiveCount++
			}
		}
		
		// Check for blockers
		blockers := []string{
			"security issue", "critical", "major concern", "blocker",
			"must fix", "cannot approve", "needs revision",
		}
		
		hasBlockers := false
		for _, blocker := range blockers {
			if strings.Contains(lowerResponse, blocker) {
				hasBlockers = true
				decision.Issues += blocker + "; "
				break
			}
		}
		
		// Make decision based on analysis
		if !hasBlockers && positiveCount >= 2 {
			decision.IsApproved = true
		} else {
			decision.IsApproved = false
			if decision.Issues == "" {
				decision.Issues = "Quality review indicates issues that need addressing"
			}
		}
	}
	
	return decision
}

func (tl *SeniorTechLead) getQualityTools(projectType ProjectType) []string {
	switch projectType {
	case ProjectTypeGo:
		return []string{"go fmt", "go vet", "go mod tidy"}
	case ProjectTypeTypeScript:
		return []string{"npm run lint", "npm audit"}
	case ProjectTypePython:
		return []string{"python -m flake8", "python -m black --check"}
	default:
		return []string{}
	}
}

func (tl *SeniorTechLead) commandAlreadyExecuted(commands []string, target string) bool {
	for _, cmd := range commands {
		if strings.Contains(cmd, target) {
			return true
		}
	}
	return false
}


func (tl *SeniorTechLead) parseActions(response string) []Action {
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