package agent

import (
	"context"
	"fmt"
	"strings"
)

type SeniorQAEngineer struct {
	llmClient    LLMClient
	tools        ToolSet
	restrictions CommandRestrictions
}

func NewSeniorQAEngineer(llmClient LLMClient, tools ToolSet, restrictions CommandRestrictions) *SeniorQAEngineer {
	return &SeniorQAEngineer{
		llmClient:    llmClient,
		tools:        tools,
		restrictions: restrictions,
	}
}

func (qa *SeniorQAEngineer) ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error) {
	// Step 1: Set working directory if specified
	if req.WorkingDirectory != "" {
		qa.tools.SetWorkingDirectory(req.WorkingDirectory)
	}

	// Step 2: Analyze what was implemented by previous agent
	implementationContext, err := qa.analyzeImplementation()
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to analyze implementation: %v", err),
		}, nil
	}

	// Step 3: Build system prompt with context
	prompt := qa.buildSystemPrompt(req, implementationContext)

	// Step 4: Generate test strategy from LLM
	response, err := qa.llmClient.Generate(ctx, prompt)
	if err != nil {
		return &ImplementFeatureResponse{
			Success: false,
			Error:   fmt.Sprintf("LLM generation failed: %v", err),
		}, nil
	}

	// Step 5: Execute test implementation
	return qa.executeTestImplementation(ctx, req, response)
}

type ImplementationContext struct {
	GitDiff          string
	ModifiedFiles    []string
	FileContents     map[string]string
	ProjectType      ProjectType
	TestingFramework string
	ExistingTests    []string
}

func (qa *SeniorQAEngineer) analyzeImplementation() (*ImplementationContext, error) {
	ctx := &ImplementationContext{
		FileContents: make(map[string]string),
	}

	// Get git diff to see what changed
	gitDiff, err := qa.tools.GetGitDiff()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %v", err)
	}
	ctx.GitDiff = gitDiff

	// Parse modified files from git diff or try to identify them
	ctx.ModifiedFiles = qa.parseModifiedFiles(gitDiff)

	// Read modified files to understand implementation
	for _, file := range ctx.ModifiedFiles {
		if content, err := qa.tools.ReadFile(file); err == nil {
			ctx.FileContents[file] = content
		}
	}

	// Determine testing framework and existing tests
	ctx.TestingFramework = qa.detectTestingFramework(ctx.ModifiedFiles)
	ctx.ExistingTests = qa.findExistingTests()

	return ctx, nil
}

func (qa *SeniorQAEngineer) parseModifiedFiles(gitDiff string) []string {
	var files []string
	lines := strings.Split(gitDiff, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Basic parsing - in a real implementation this would be more robust
		if strings.HasPrefix(line, "modified:") || strings.HasPrefix(line, "new file:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				files = append(files, parts[len(parts)-1])
			}
		}
	}

	// If no files detected from git diff, try to identify common patterns
	if len(files) == 0 {
		commonFiles := []string{
			"main.go", "server.go", "handler.go", "service.go",
			"app.js", "index.js", "server.js", "api.js",
			"main.py", "app.py", "server.py", "handler.py",
		}
		
		for _, file := range commonFiles {
			if _, err := qa.tools.ReadFile(file); err == nil {
				files = append(files, file)
			}
		}
	}

	return files
}

func (qa *SeniorQAEngineer) detectTestingFramework(files []string) string {
	// Check for existing test files to determine framework
	for _, file := range files {
		if strings.Contains(file, ".go") {
			return "go_test"
		}
		if strings.Contains(file, ".js") || strings.Contains(file, ".ts") {
			return "jest"
		}
		if strings.Contains(file, ".py") {
			return "pytest"
		}
	}
	return "unknown"
}

func (qa *SeniorQAEngineer) findExistingTests() []string {
	var testFiles []string
	
	// Common test file patterns
	testPatterns := []string{
		"*_test.go", "*test*.js", "*test*.ts", "*spec*.js", "*spec*.ts",
		"test_*.py", "*_test.py",
	}
	
	for _, pattern := range testPatterns {
		// This is simplified - in a real implementation we'd use filepath.Glob
		// or similar directory scanning
		if strings.Contains(pattern, "_test.go") {
			if _, err := qa.tools.ReadFile("main_test.go"); err == nil {
				testFiles = append(testFiles, "main_test.go")
			}
		}
	}
	
	return testFiles
}

func (qa *SeniorQAEngineer) buildSystemPrompt(req ImplementFeatureRequest, ctx *ImplementationContext) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are a Senior QA Engineer focused on strategic testing of critical functionality.

**Current Task:** Write essential tests for: %s
**Project Type:** %s
**Testing Framework:** %s

**Your Philosophy:**
- Quality over quantity: Minimal tests that catch real issues
- Focus on critical paths and user-facing functionality
- No line coverage goals - test what matters
- Every test must add value and catch actual bugs

**Implementation Analysis:**
The following files were modified/created:
`, req.Description, req.ProjectType, ctx.TestingFramework))

	for filename, content := range ctx.FileContents {
		if len(content) > 800 {
			content = content[:800] + "... (truncated)"
		}
		prompt.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", filename, content))
	}

	if ctx.GitDiff != "" {
		prompt.WriteString(fmt.Sprintf("\n**Git Diff:**\n%s\n", ctx.GitDiff))
	}

	if len(ctx.ExistingTests) > 0 {
		prompt.WriteString(fmt.Sprintf("\n**Existing Tests:**\n%s\n", strings.Join(ctx.ExistingTests, ", ")))
	}

	prompt.WriteString(`
**Your Responsibilities:**
1. **IDENTIFY CRITICAL AREAS**: Determine what functionality is most important to test
2. **STRATEGIC TESTING**: Write minimal tests that provide maximum bug detection
3. **EXECUTION VALIDATION**: Always run tests and ensure they pass before completion
4. **FAILURE ANALYSIS**: Distinguish between test issues and implementation bugs

**Critical Area Identification Framework:**
HIGH PRIORITY - Must Test:
- Public APIs and user-facing functions
- Error handling and edge cases
- Business logic and calculations
- Data validation and sanitization
- Integration points and dependencies

MEDIUM PRIORITY - Test if Complex:
- Helper functions with business logic
- Complex algorithms or transformations
- State management

LOW PRIORITY - Skip Unless Trivial:
- Simple getters/setters
- Configuration loading
- Obvious wrapper functions

**Minimal Test Strategy:**
- ONE test per function for happy path
- ONE test for most common error condition
- ONE test for critical edge case (if applicable)
- NO exhaustive permutation testing
- NO tests for framework/library functionality

**Available Actions:**
- READ_FILE: Read existing test files to understand patterns
- WRITE_FILE: Create new test files
- EXECUTE_COMMAND: Run test commands (MANDATORY before completion)
- SEQUENTIAL_THINKING: Use for complex test analysis and planning

**When to Use Sequential Thinking:**
Use sequential thinking when:
- Analyzing complex implementations with multiple components
- Planning comprehensive test coverage for intricate features
- Debugging test failures or understanding implementation issues
- Determining critical paths and edge cases systematically
- Breaking down testing strategy for complex business logic

**Sequential Thinking for Testing:**
SEQUENTIAL_THINKING:
THOUGHT: I need to analyze this user management implementation to identify the most critical test cases. Let me start by understanding what functionality was implemented.
THOUGHT_NUMBER: 1
TOTAL_THOUGHTS: 4
NEXT_THOUGHT_NEEDED: true

**Response Format:**
CRITICAL_ANALYSIS:
- List HIGH PRIORITY areas that need testing
- Justify why each area is critical
- Identify minimal test cases needed

ACTION: WRITE_FILE
PATH: path/to/test/file
CONTENT:
` + "```" + `
test code here
` + "```" + `

ACTION: EXECUTE_COMMAND
COMMAND: test command

**Quality Criteria:**
- Tests must validate actual functionality, not implementation details
- Each test should catch a real failure scenario
- Tests must be deterministic and reliable
- ALL tests must pass before completing QA phase

Begin by identifying critical areas and implementing targeted tests.`)

	return prompt.String()
}

func (qa *SeniorQAEngineer) executeTestImplementation(ctx context.Context, req ImplementFeatureRequest, llmResponse string) (*ImplementFeatureResponse, error) {
	result := &ImplementFeatureResponse{
		Success:          true,
		FilesModified:    []string{},
		CommandsExecuted: []string{},
		BuildOutput:      "",
		NextSteps:        "Tests implemented and validated",
	}

	// Parse LLM response for actions
	actions := qa.parseActions(llmResponse)

	for _, action := range actions {
		switch action.Type {
		case "READ_FILE":
			// Just for context, don't need to store result
			_, err := qa.tools.ReadFile(action.Path)
			if err != nil {
				// Don't fail for missing files during exploration
				continue
			}

		case "WRITE_FILE":
			err := qa.tools.WriteFile(action.Path, action.Content)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Failed to write test file %s: %v", action.Path, err)
				return result, nil
			}
			result.FilesModified = append(result.FilesModified, action.Path)

		case "EXECUTE_COMMAND":
			if err := qa.restrictions.ValidateCommand(action.Command); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Command validation failed: %v", err)
				return result, nil
			}

			output, err := qa.tools.ExecuteCommand(action.Command)
			result.CommandsExecuted = append(result.CommandsExecuted, action.Command)
			result.BuildOutput += output + "\n"
			
			if err != nil {
				// Test failures should be analyzed, not immediately treated as errors
				testAnalysis := qa.analyzeTestResults(output, err.Error())
				if testAnalysis.HasImplementationBugs {
					result.Success = false
					result.Error = fmt.Sprintf("Tests revealed implementation bugs: %s", testAnalysis.Issues)
					result.NextSteps = "Implementation needs fixes before proceeding"
					return result, nil
				} else if testAnalysis.HasTestIssues {
					// Test issues - continue and let QA iterate
					result.BuildOutput += "\nTest Issues Detected: " + testAnalysis.Issues + "\n"
				}
			}
		}
	}

	// MANDATORY: Ensure tests are executed and pass before completion
	testCommand := qa.getTestCommand(req.ProjectType)
	testsExecuted := qa.commandAlreadyExecuted(result.CommandsExecuted, testCommand)
	
	if testCommand != "" && !testsExecuted {
		if err := qa.restrictions.ValidateCommand(testCommand); err == nil {
			output, err := qa.tools.ExecuteCommand(testCommand)
			result.CommandsExecuted = append(result.CommandsExecuted, testCommand)
			result.BuildOutput += "\nMandatory Test Execution:\n" + output
			testsExecuted = true
			
			if err != nil {
				testAnalysis := qa.analyzeTestResults(output, err.Error())
				if testAnalysis.HasImplementationBugs {
					result.Success = false
					result.Error = fmt.Sprintf("Tests revealed implementation bugs: %s", testAnalysis.Issues)
					result.NextSteps = "Implementation needs fixes before proceeding"
					return result, nil
				} else {
					result.Success = false
					result.Error = fmt.Sprintf("Test execution failed: %s", testAnalysis.Issues)
					result.NextSteps = "Fix test issues and re-run"
					return result, nil
				}
			}
		} else {
			result.Success = false
			result.Error = fmt.Sprintf("Cannot validate test execution: %v", err)
			result.NextSteps = "Fix test command permissions"
			return result, nil
		}
	}

	// MANDATORY: Tests must have been executed successfully
	if !testsExecuted {
		result.Success = false
		result.Error = "QA phase requires test execution - no tests were run"
		result.NextSteps = "Implement and execute tests before completion"
		return result, nil
	}

	// Validate that test output shows success
	if !qa.validateTestSuccess(result.BuildOutput) {
		result.Success = false
		result.Error = "Test execution output indicates failures or issues"
		result.NextSteps = "All tests must pass before QA completion"
		return result, nil
	}

	// Final analysis of QA work with stricter validation
	qaAnalysis := qa.analyzeQAWork(result, llmResponse)
	
	if qaAnalysis.HasAdequateTests && qaAnalysis.TestCount > 0 {
		result.Message = "Strategic tests implemented and validated - all tests passing"
		result.NextSteps = "Ready for tech lead quality review"
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("QA validation failed: %s", qaAnalysis.Issues)
		result.NextSteps = "Address test implementation issues"
	}

	return result, nil
}

type TestAnalysis struct {
	HasImplementationBugs bool
	HasTestIssues        bool
	Issues               string
}

func (qa *SeniorQAEngineer) analyzeTestResults(output, errorText string) TestAnalysis {
	analysis := TestAnalysis{}
	
	combinedText := strings.ToLower(output + " " + errorText)
	
	// Check for implementation bugs (test failures that indicate code issues)
	implementationBugPatterns := []string{
		"assertion failed", "expected", "actual", "AssertionError",
		"test failed", "failure:", "error:", "exception:",
		"nil pointer", "index out of bounds", "runtime error",
	}
	
	for _, pattern := range implementationBugPatterns {
		if strings.Contains(combinedText, strings.ToLower(pattern)) {
			analysis.HasImplementationBugs = true
			analysis.Issues += "Test failures indicate implementation bugs. "
			break
		}
	}
	
	// Check for test issues (problems with test code itself)
	testIssuePatterns := []string{
		"syntax error", "compile error", "import error",
		"undefined", "not declared", "cannot find",
		"test setup failed", "mock error",
	}
	
	for _, pattern := range testIssuePatterns {
		if strings.Contains(combinedText, strings.ToLower(pattern)) {
			analysis.HasTestIssues = true
			analysis.Issues += "Test code has issues that need fixing. "
			break
		}
	}
	
	return analysis
}

type QAAnalysis struct {
	HasAdequateTests bool
	Issues          string
	TestCount       int
	CoverageAreas   []string
}

func (qa *SeniorQAEngineer) validateTestSuccess(buildOutput string) bool {
	lowerOutput := strings.ToLower(buildOutput)
	
	// Check for test success indicators
	successPatterns := []string{
		"pass", "ok", "success", "all tests passed",
		"0 failed", "✓", "✔", "passed",
	}
	
	// Check for failure indicators
	failurePatterns := []string{
		"fail", "error", "assertion", "exception",
		"failed", "✗", "✖", "timeout", "panic",
	}
	
	hasSuccess := false
	for _, pattern := range successPatterns {
		if strings.Contains(lowerOutput, pattern) {
			hasSuccess = true
			break
		}
	}
	
	hasFailure := false
	for _, pattern := range failurePatterns {
		if strings.Contains(lowerOutput, pattern) {
			hasFailure = true
			break
		}
	}
	
	// Must have success indicators and no failure indicators
	return hasSuccess && !hasFailure
}

func (qa *SeniorQAEngineer) analyzeQAWork(result *ImplementFeatureResponse, llmResponse string) QAAnalysis {
	analysis := QAAnalysis{}
	
	// Count test files created
	testFileCount := 0
	for _, file := range result.FilesModified {
		if qa.isTestFile(file) {
			testFileCount++
		}
	}
	analysis.TestCount = testFileCount
	
	// Analyze critical area coverage from LLM response
	lowerResponse := strings.ToLower(llmResponse)
	
	// Look for evidence of critical area identification
	criticalAreaKeywords := []string{
		"critical", "high priority", "public api", "user-facing",
		"business logic", "error handling", "validation",
		"integration", "edge case", "happy path",
	}
	
	criticalAreaCount := 0
	for _, keyword := range criticalAreaKeywords {
		if strings.Contains(lowerResponse, keyword) {
			criticalAreaCount++
		}
	}
	
	// Check for strategic testing approach
	strategicKeywords := []string{
		"minimal", "strategic", "targeted", "essential",
		"must test", "critical path", "functionality",
	}
	
	strategicCount := 0
	for _, keyword := range strategicKeywords {
		if strings.Contains(lowerResponse, keyword) {
			strategicCount++
		}
	}
	
	// Validate quality over quantity approach
	if testFileCount >= 1 && criticalAreaCount >= 3 && strategicCount >= 2 {
		analysis.HasAdequateTests = true
	} else {
		analysis.HasAdequateTests = false
		
		var issues []string
		if testFileCount == 0 {
			issues = append(issues, "No test files created")
		}
		if criticalAreaCount < 3 {
			issues = append(issues, "Insufficient critical area identification")
		}
		if strategicCount < 2 {
			issues = append(issues, "Missing strategic testing approach")
		}
		if !strings.Contains(lowerResponse, "critical_analysis") {
			issues = append(issues, "Missing required critical analysis section")
		}
		
		analysis.Issues = strings.Join(issues, "; ")
	}
	
	return analysis
}

func (qa *SeniorQAEngineer) isTestFile(filename string) bool {
	testPatterns := []string{
		"_test.go", ".test.js", ".test.ts", ".spec.js", ".spec.ts",
		"test_", "_test.py", "/test/", "/tests/",
	}
	
	for _, pattern := range testPatterns {
		if strings.Contains(strings.ToLower(filename), pattern) {
			return true
		}
	}
	
	return false
}

func (qa *SeniorQAEngineer) getTestCommand(projectType ProjectType) string {
	switch projectType {
	case ProjectTypeGo:
		return "go test ./..."
	case ProjectTypeTypeScript:
		return "npm test"
	case ProjectTypePython:
		return "python -m pytest"
	default:
		return ""
	}
}

func (qa *SeniorQAEngineer) commandAlreadyExecuted(commands []string, target string) bool {
	for _, cmd := range commands {
		if strings.Contains(cmd, target) {
			return true
		}
	}
	return false
}


func (qa *SeniorQAEngineer) parseActions(response string) []Action {
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

// DocumentTask for SeniorQAEngineer is a no-op
func (qa *SeniorQAEngineer) DocumentTask(ctx context.Context, result *WorkflowResult) error {
	return nil
}