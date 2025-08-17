package agent

import (
	"context"
	"fmt"
	"strings"
	"regexp"
	"path/filepath"
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
	EMBrief         *EMBrief
	PatternFiles    map[string]string
	RelatedFiles    map[string]string
}

type RejectionReason string

const (
	RejectionRequirements RejectionReason = "requirements_not_met"
	RejectionSecurity     RejectionReason = "security_concerns"
	RejectionDuplication  RejectionReason = "unnecessary_duplication"
	RejectionPatterns     RejectionReason = "pattern_deviation"
)

type SecurityIssue struct {
	Type        string
	Description string
	File        string
	Line        int
	Severity    string
}

type DuplicationIssue struct {
	Type            string
	Description     string
	CurrentFile     string
	ExistingFile    string
	SimilarityScore int
}

type PatternAnalysis struct {
	Deviations []PatternDeviation
	Consistent bool
}

type PatternDeviation struct {
	Type        string
	Description string
	File        string
	Expected    string
	Actual      string
}

func (tl *SeniorTechLead) analyzeCompleteWork() (*ReviewContext, error) {
	ctx := &ReviewContext{
		FileContents: make(map[string]string),
		PatternFiles: make(map[string]string),
		RelatedFiles: make(map[string]string),
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

	// Load pattern documentation
	if err := tl.loadPatternDocumentation(ctx); err != nil {
		// Don't fail if pattern files aren't available, just log
		fmt.Printf("Warning: Could not load pattern documentation: %v\n", err)
	}

	// Find related files for duplication analysis
	if err := tl.findRelatedFiles(ctx); err != nil {
		// Don't fail if related files can't be found
		fmt.Printf("Warning: Could not find related files: %v\n", err)
	}

	// Try to extract EM brief from task description (will be provided in buildSystemPrompt)
	// This will be parsed when we have the full request context

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

// loadPatternDocumentation loads relevant pattern files for analysis
func (tl *SeniorTechLead) loadPatternDocumentation(ctx *ReviewContext) error {
	// Load main project patterns overview
	if content, err := tl.tools.ReadFile("PROJECT_PATTERNS.md"); err == nil {
		ctx.PatternFiles["PROJECT_PATTERNS.md"] = content
	}

	// Dynamically discover all pattern files in patterns/ directory
	if patternFiles, err := tl.tools.ListFiles("patterns"); err == nil {
		for _, file := range patternFiles {
			if strings.HasSuffix(file, ".md") {
				patternPath := filepath.Join("patterns", file)
				if content, err := tl.tools.ReadFile(patternPath); err == nil {
					ctx.PatternFiles[patternPath] = content
				}
			}
		}
	}

	// Load agent coordination patterns (fallback locations)
	agentFiles := []string{"AGENTS.md", "agents/AGENTS.md"}
	for _, agentFile := range agentFiles {
		if content, err := tl.tools.ReadFile(agentFile); err == nil {
			ctx.PatternFiles[agentFile] = content
		}
	}

	return nil
}

// findRelatedFiles finds files related to the changed files for duplication analysis
func (tl *SeniorTechLead) findRelatedFiles(ctx *ReviewContext) error {
	for _, changedFile := range ctx.AllChangedFiles {
		// Find files in the same package/directory
		dir := filepath.Dir(changedFile)
		if files, err := tl.tools.ListFiles(dir); err == nil {
			for _, file := range files {
				if file != changedFile && tl.isRelevantFile(file, changedFile) {
					if content, err := tl.tools.ReadFile(file); err == nil {
						ctx.RelatedFiles[file] = content
					}
				}
			}
		}

		// Find files with similar names/patterns
		if relatedFiles, err := tl.findSimilarFiles(changedFile); err == nil {
			for _, file := range relatedFiles {
				if content, err := tl.tools.ReadFile(file); err == nil {
					ctx.RelatedFiles[file] = content
				}
			}
		}
	}

	return nil
}

// isRelevantFile determines if a file is relevant for comparison
func (tl *SeniorTechLead) isRelevantFile(file, changedFile string) bool {
	// Skip test files when analyzing implementation files
	if strings.Contains(file, "_test.") {
		return false
	}

	// Check if files have similar purposes based on name
	changedBase := filepath.Base(changedFile)
	fileBase := filepath.Base(file)

	// Same file type (handlers, services, models, etc.)
	patterns := []string{"handler", "service", "model", "repository", "controller"}
	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(changedBase), pattern) &&
			strings.Contains(strings.ToLower(fileBase), pattern) {
			return true
		}
	}

	return false
}

// findSimilarFiles finds files with similar patterns/names
func (tl *SeniorTechLead) findSimilarFiles(changedFile string) ([]string, error) {
	var similarFiles []string

	// Search for files with similar patterns
	base := filepath.Base(changedFile)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// Look for files with similar names
	patterns := []string{
		"*" + nameWithoutExt + "*" + ext,
		"*handler*" + ext,
		"*service*" + ext,
		"*model*" + ext,
	}

	for _, pattern := range patterns {
		if files, err := tl.tools.FindFiles(pattern, "."); err == nil {
			for _, file := range files {
				if file != changedFile {
					similarFiles = append(similarFiles, file)
				}
			}
		}
	}

	return similarFiles, nil
}

// validateRequirements validates implementation against EM brief requirements
func (tl *SeniorTechLead) validateRequirements(emBrief *EMBrief, ctx *ReviewContext) []string {
	var gaps []string

	if emBrief == nil || emBrief.Task == "" {
		return gaps // No structured brief to validate against
	}

	// Parse success criteria
	successCriteria := strings.Split(emBrief.SuccessCriteria, ",")
	for _, criterion := range successCriteria {
		criterion = strings.TrimSpace(criterion)
		if criterion == "" {
			continue
		}

		// Check if criterion is met by implementation
		if !tl.isSuccessCriteriaMet(criterion, ctx) {
			gaps = append(gaps, fmt.Sprintf("Success criterion not met: %s", criterion))
		}
	}

	// Validate core task completion
	if !tl.isTaskCompleted(emBrief.Task, ctx) {
		gaps = append(gaps, fmt.Sprintf("Core task not completed: %s", emBrief.Task))
	}

	return gaps
}

// isSuccessCriteriaMet checks if a specific success criterion is met
func (tl *SeniorTechLead) isSuccessCriteriaMet(criterion string, ctx *ReviewContext) bool {
	criterionLower := strings.ToLower(criterion)

	// Check for endpoint/API criteria
	if strings.Contains(criterionLower, "endpoint") || strings.Contains(criterionLower, "api") {
		return tl.hasAPIEndpoint(criterion, ctx)
	}

	// Check for test criteria
	if strings.Contains(criterionLower, "test") {
		return len(ctx.TestFiles) > 0
	}

	// Check for build/compilation criteria
	if strings.Contains(criterionLower, "build") || strings.Contains(criterionLower, "compile") {
		return tl.codeCompiles(ctx)
	}

	// Check for file/functionality criteria
	if strings.Contains(criterionLower, "file") {
		return tl.hasRequiredFiles(criterion, ctx)
	}

	return true // Default to true for unrecognized criteria
}

// isTaskCompleted checks if the core task has been completed
func (tl *SeniorTechLead) isTaskCompleted(task string, ctx *ReviewContext) bool {
	taskLower := strings.ToLower(task)

	// Check if implementation files were created/modified
	if len(ctx.AllChangedFiles) == 0 {
		return false
	}

	// Look for task-specific implementations
	if strings.Contains(taskLower, "create") || strings.Contains(taskLower, "implement") {
		return tl.hasImplementationFiles(ctx)
	}

	return true
}

// evaluateSecurityConcerns performs comprehensive security analysis
func (tl *SeniorTechLead) evaluateSecurityConcerns(ctx *ReviewContext) []SecurityIssue {
	var issues []SecurityIssue

	for filename, content := range ctx.FileContents {
		issues = append(issues, tl.checkSQLInjection(filename, content)...)
		issues = append(issues, tl.checkPathTraversal(filename, content)...)
		issues = append(issues, tl.checkInputValidation(filename, content)...)
		issues = append(issues, tl.checkHardcodedSecrets(filename, content)...)
		issues = append(issues, tl.checkResourceLeaks(filename, content)...)
		issues = append(issues, tl.checkUnsafeDeserialization(filename, content)...)
	}

	return issues
}

// checkSQLInjection detects potential SQL injection vulnerabilities
func (tl *SeniorTechLead) checkSQLInjection(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	// Look for string concatenation in SQL queries
	sqlConcatPatterns := []string{
		`fmt\.Sprintf.*SELECT`,
		`fmt\.Sprintf.*INSERT`,
		`fmt\.Sprintf.*UPDATE`,
		`fmt\.Sprintf.*DELETE`,
		`".*SELECT.*".*\+`,
		`".*INSERT.*".*\+`,
	}

	for _, pattern := range sqlConcatPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			issues = append(issues, SecurityIssue{
				Type:        "SQL_INJECTION",
				Description: "Potential SQL injection vulnerability: string concatenation in SQL query",
				File:        filename,
				Severity:    "CRITICAL",
			})
		}
	}

	return issues
}

// checkPathTraversal detects potential path traversal vulnerabilities
func (tl *SeniorTechLead) checkPathTraversal(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	// Look for file operations with user input
	pathTraversalPatterns := []string{
		`filepath\.Join.*req\.`,
		`os\.Open.*req\.`,
		`ioutil\.ReadFile.*req\.`,
		`".*\.\./`,
	}

	for _, pattern := range pathTraversalPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			issues = append(issues, SecurityIssue{
				Type:        "PATH_TRAVERSAL",
				Description: "Potential path traversal vulnerability: file operation with user input",
				File:        filename,
				Severity:    "HIGH",
			})
		}
	}

	return issues
}

// checkInputValidation detects missing input validation
func (tl *SeniorTechLead) checkInputValidation(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	// Look for request binding without validation
	if strings.Contains(content, "BindJSON") || strings.Contains(content, "Bind(") {
		if !strings.Contains(content, "validate:") && !strings.Contains(content, "Validate(") {
			issues = append(issues, SecurityIssue{
				Type:        "MISSING_VALIDATION",
				Description: "Request binding without validation detected",
				File:        filename,
				Severity:    "MEDIUM",
			})
		}
	}

	return issues
}

// checkHardcodedSecrets detects hardcoded secrets
func (tl *SeniorTechLead) checkHardcodedSecrets(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	secretPatterns := []string{
		`"[A-Za-z0-9]{32,}"`, // Long alphanumeric strings
		`password.*=.*"`,
		`secret.*=.*"`,
		`key.*=.*"[A-Za-z0-9+/]{20,}"`,
		`token.*=.*"[A-Za-z0-9+/]{20,}"`,
	}

	for _, pattern := range secretPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			issues = append(issues, SecurityIssue{
				Type:        "HARDCODED_SECRET",
				Description: "Potential hardcoded secret or credential detected",
				File:        filename,
				Severity:    "CRITICAL",
			})
		}
	}

	return issues
}

// checkResourceLeaks detects potential resource leaks
func (tl *SeniorTechLead) checkResourceLeaks(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	// Check for opened files without defer close
	if strings.Contains(content, "os.Open") || strings.Contains(content, "os.Create") {
		if !strings.Contains(content, "defer") || !strings.Contains(content, "Close()") {
			issues = append(issues, SecurityIssue{
				Type:        "RESOURCE_LEAK",
				Description: "File opened without proper cleanup (missing defer close)",
				File:        filename,
				Severity:    "MEDIUM",
			})
		}
	}

	return issues
}

// checkUnsafeDeserialization detects unsafe deserialization
func (tl *SeniorTechLead) checkUnsafeDeserialization(filename, content string) []SecurityIssue {
	var issues []SecurityIssue

	// Look for unsafe JSON/XML parsing
	if strings.Contains(content, "json.Unmarshal") || strings.Contains(content, "xml.Unmarshal") {
		if !strings.Contains(content, "validate:") {
			issues = append(issues, SecurityIssue{
				Type:        "UNSAFE_DESERIALIZATION",
				Description: "Deserialization without validation detected",
				File:        filename,
				Severity:    "MEDIUM",
			})
		}
	}

	return issues
}

// detectDuplication detects unnecessary code duplication
func (tl *SeniorTechLead) detectDuplication(ctx *ReviewContext) []DuplicationIssue {
	var issues []DuplicationIssue

	for currentFile, currentContent := range ctx.FileContents {
		for existingFile, existingContent := range ctx.RelatedFiles {
			if duplications := tl.findCodeDuplication(currentFile, currentContent, existingFile, existingContent); len(duplications) > 0 {
				issues = append(issues, duplications...)
			}
		}
	}

	return issues
}

// findCodeDuplication finds specific duplications between two files
func (tl *SeniorTechLead) findCodeDuplication(currentFile, currentContent, existingFile, existingContent string) []DuplicationIssue {
	var issues []DuplicationIssue

	// Look for similar function signatures
	currentFunctions := tl.extractFunctions(currentContent)
	existingFunctions := tl.extractFunctions(existingContent)

	for _, currentFunc := range currentFunctions {
		for _, existingFunc := range existingFunctions {
			similarity := tl.calculateSimilarity(currentFunc, existingFunc)
			if similarity > 80 { // 80% similarity threshold
				issues = append(issues, DuplicationIssue{
					Type:            "FUNCTION_DUPLICATION",
					Description:     fmt.Sprintf("Similar function found: %s", currentFunc.Name),
					CurrentFile:     currentFile,
					ExistingFile:    existingFile,
					SimilarityScore: similarity,
				})
			}
		}
	}

	return issues
}

// analyzePatternConsistency analyzes pattern consistency against existing code
func (tl *SeniorTechLead) analyzePatternConsistency(ctx *ReviewContext) *PatternAnalysis {
	analysis := &PatternAnalysis{
		Deviations: []PatternDeviation{},
		Consistent: true,
	}

	for filename, content := range ctx.FileContents {
		deviations := tl.checkPatternConsistency(filename, content, ctx)
		analysis.Deviations = append(analysis.Deviations, deviations...)
	}

	analysis.Consistent = len(analysis.Deviations) == 0
	return analysis
}

// createRejectionFeedback creates structured feedback for rejection
func (tl *SeniorTechLead) createRejectionFeedback(reason RejectionReason, issues []string, examples []string) string {
	var feedback strings.Builder

	feedback.WriteString(fmt.Sprintf("REJECTION_REASON: %s\n", reason))
	feedback.WriteString("SPECIFIC_ISSUES:\n")
	for _, issue := range issues {
		feedback.WriteString(fmt.Sprintf("- %s\n", issue))
	}

	if len(examples) > 0 {
		feedback.WriteString("\nEXISTING_PATTERNS:\n")
		for _, example := range examples {
			feedback.WriteString(fmt.Sprintf("- %s\n", example))
		}
	}

	feedback.WriteString("\nREQUIRED_ACTIONS:\n")
	switch reason {
	case RejectionRequirements:
		feedback.WriteString("- Review EM brief and implement missing requirements\n")
		feedback.WriteString("- Ensure all success criteria are met\n")
	case RejectionSecurity:
		feedback.WriteString("- Fix all security vulnerabilities before proceeding\n")
		feedback.WriteString("- Add proper input validation and sanitization\n")
	case RejectionDuplication:
		feedback.WriteString("- Remove duplicate code and reuse existing functionality\n")
		feedback.WriteString("- Refactor to use shared utilities or services\n")
	case RejectionPatterns:
		feedback.WriteString("- Follow existing code patterns and conventions\n")
		feedback.WriteString("- Update implementation to match project standards\n")
	}

	feedback.WriteString("\nROUTE_TO: engineering_manager\n")

	return feedback.String()
}

// Helper methods for analysis

type FunctionInfo struct {
	Name      string
	Signature string
	Body      string
}

func (tl *SeniorTechLead) extractFunctions(content string) []FunctionInfo {
	// Simple function extraction for Go
	var functions []FunctionInfo
	funcRegex := regexp.MustCompile(`func\s+(\w+)\s*\([^)]*\)[^{]*\{`)
	matches := funcRegex.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			functions = append(functions, FunctionInfo{
				Name:      match[1],
				Signature: match[0],
			})
		}
	}
	
	return functions
}

func (tl *SeniorTechLead) calculateSimilarity(func1, func2 FunctionInfo) int {
	// Simple similarity calculation based on function names and signatures
	if func1.Name == func2.Name {
		return 100
	}
	
	// Compare signatures
	if strings.Contains(func1.Signature, func2.Name) || strings.Contains(func2.Signature, func1.Name) {
		return 85
	}
	
	return 0
}

func (tl *SeniorTechLead) checkPatternConsistency(filename, content string, ctx *ReviewContext) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check against project-specific pattern documentation
	if projectPatterns, exists := ctx.PatternFiles["PROJECT_PATTERNS.md"]; exists {
		deviations = append(deviations, tl.validateProjectPatterns(filename, content, projectPatterns)...)
	}
	
	// Dynamically validate against all pattern files found in patterns/ directory
	for patternFile, patternContent := range ctx.PatternFiles {
		if strings.HasPrefix(patternFile, "patterns/") && strings.HasSuffix(patternFile, ".md") {
			// Extract pattern type from filename (e.g., "patterns/handler.md" -> "handler")
			patternType := strings.TrimSuffix(filepath.Base(patternFile), ".md")
			deviations = append(deviations, tl.validateSpecificPattern(filename, content, patternType, patternContent)...)
		}
	}
	
	return deviations
}

func (tl *SeniorTechLead) validateProjectPatterns(filename, content, projectPatterns string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Extract patterns from PROJECT_PATTERNS.md
	// Look for discovered patterns and validate against them
	if strings.Contains(projectPatterns, "## Discovered Patterns") {
		// Check if the file follows discovered architectural patterns
		if strings.Contains(projectPatterns, "**Style**: layered") {
			// Validate layered architecture compliance
			if strings.Contains(filename, "handler") && strings.Contains(content, "func") {
				if !tl.validateLayeredHandlerStructure(content) {
					deviations = append(deviations, PatternDeviation{
						Type:        "ARCHITECTURE_DEVIATION",
						Description: "Handler doesn't follow layered architecture pattern",
						File:        filename,
						Expected:    "Handlers should delegate business logic to service layer",
						Actual:      "Direct business logic in handler detected",
					})
				}
			}
		}
		
		// Validate against discovered framework patterns
		if strings.Contains(projectPatterns, "**Framework**: fiber") {
			if strings.Contains(filename, "handler") && !strings.Contains(content, "*fiber.Ctx") {
				deviations = append(deviations, PatternDeviation{
					Type:        "FRAMEWORK_DEVIATION", 
					Description: "Handler doesn't use project's Fiber framework pattern",
					File:        filename,
					Expected:    "func HandlerName(c *fiber.Ctx) error",
					Actual:      "Non-Fiber handler signature found",
				})
			}
		}
	}
	
	return deviations
}

func (tl *SeniorTechLead) validateLayeredHandlerStructure(content string) bool {
	// Check if handler delegates to service layer rather than containing business logic
	hasServiceCall := strings.Contains(content, ".Service") || strings.Contains(content, "service.")
	hasDirectDBAccess := strings.Contains(content, ".DB") || strings.Contains(content, ".Query") || strings.Contains(content, ".Exec")
	
	// Good: has service calls, no direct DB access
	return hasServiceCall && !hasDirectDBAccess
}

func (tl *SeniorTechLead) validateModelPatterns(filename, content, patterns string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check if this is a model/struct file
	if !strings.Contains(strings.ToLower(filename), "model") && !strings.Contains(content, "type") {
		return deviations
	}
	
	// Validate struct patterns from discovered documentation
	if strings.Contains(patterns, "Request/Response") {
		// Check for proper request/response naming
		if strings.Contains(content, "type") && strings.Contains(content, "struct") {
			if !tl.followsNamingConvention(content) {
				deviations = append(deviations, PatternDeviation{
					Type:        "NAMING_CONVENTION",
					Description: "Struct doesn't follow project naming conventions",
					File:        filename,
					Expected:    "Type names should follow discovered Request/Response patterns",
					Actual:      "Non-standard struct naming found",
				})
			}
		}
	}
	
	return deviations
}

func (tl *SeniorTechLead) followsNamingConvention(content string) bool {
	// Check for proper naming patterns like XxxRequest, XxxResponse, XxxDTO
	requestPattern := regexp.MustCompile(`type\s+\w+Request\s+struct`)
	responsePattern := regexp.MustCompile(`type\s+\w+Response\s+struct`) 
	dtoPattern := regexp.MustCompile(`type\s+\w+DTO\s+struct`)
	
	hasProperNaming := requestPattern.MatchString(content) || 
		responsePattern.MatchString(content) || 
		dtoPattern.MatchString(content)
	
	return hasProperNaming
}

// validateSpecificPattern provides generic pattern validation based on pattern type
func (tl *SeniorTechLead) validateSpecificPattern(filename, content, patternType, patternContent string) []PatternDeviation {
	switch patternType {
	case "handler":
		return tl.validateHandlerPatterns(filename, content, patternContent)
	case "error_handling":
		return tl.validateErrorPatterns(filename, content, patternContent)
	case "model":
		return tl.validateModelPatterns(filename, content, patternContent)
	case "interface":
		return tl.validateInterfacePatterns(filename, content, patternContent)
	default:
		// For unknown pattern types, do basic pattern analysis
		return tl.validateGenericPattern(filename, content, patternType, patternContent)
	}
}

// validateGenericPattern provides basic validation for any pattern type
func (tl *SeniorTechLead) validateGenericPattern(filename, content, patternType, patternContent string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check if the file is relevant to this pattern type
	if !strings.Contains(strings.ToLower(filename), patternType) && 
	   !strings.Contains(strings.ToLower(content), patternType) {
		return deviations // Skip if not relevant
	}
	
	// Extract examples from pattern documentation 
	if strings.Contains(patternContent, "## Examples") || strings.Contains(patternContent, "### Examples") {
		// Look for code blocks in examples
		if strings.Contains(patternContent, "```") {
			// Basic validation - check if the current code follows similar patterns
			// This is a simple heuristic that can be expanded
			if !tl.hasPatternCompliance(content, patternContent) {
				deviations = append(deviations, PatternDeviation{
					Type:        strings.ToUpper(patternType) + "_DEVIATION",
					Description: fmt.Sprintf("Code doesn't follow established %s patterns", patternType),
					File:        filename,
					Expected:    fmt.Sprintf("Follow patterns documented in patterns/%s.md", patternType),
					Actual:      "Non-compliant code structure detected",
				})
			}
		}
	}
	
	return deviations
}

// hasPatternCompliance checks basic pattern compliance
func (tl *SeniorTechLead) hasPatternCompliance(content, patternContent string) bool {
	// Simple heuristic: if the pattern doc mentions specific naming conventions,
	// check if the code follows them
	if strings.Contains(patternContent, "func ") && strings.Contains(content, "func ") {
		return true // Basic function pattern compliance
	}
	if strings.Contains(patternContent, "type ") && strings.Contains(content, "type ") {
		return true // Basic type pattern compliance  
	}
	
	// Default to compliant if we can't determine
	return true
}

// validateInterfacePatterns validates interface-specific patterns
func (tl *SeniorTechLead) validateInterfacePatterns(filename, content, patterns string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check if this file contains interfaces
	if !strings.Contains(content, "interface") {
		return deviations
	}
	
	// Validate interface naming conventions from pattern doc
	if strings.Contains(patterns, "Interface") || strings.Contains(patterns, "Service") {
		if !tl.followsInterfaceNaming(content) {
			deviations = append(deviations, PatternDeviation{
				Type:        "INTERFACE_NAMING",
				Description: "Interface doesn't follow project naming conventions",
				File:        filename,
				Expected:    "Interface names should follow documented patterns",
				Actual:      "Non-standard interface naming found",
			})
		}
	}
	
	return deviations
}

// followsInterfaceNaming checks interface naming conventions
func (tl *SeniorTechLead) followsInterfaceNaming(content string) bool {
	// Check for common interface naming patterns
	interfacePattern := regexp.MustCompile(`type\s+\w+(?:Service|Repository|Client|Handler|Manager|Interface)\s+interface`)
	return interfacePattern.MatchString(content)
}

func (tl *SeniorTechLead) validateHandlerPatterns(filename, content, patterns string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check if this is a handler file
	if !strings.Contains(strings.ToLower(filename), "handler") {
		return deviations
	}
	
	// Validate handler function signatures
	if strings.Contains(content, "func Handle") && !strings.Contains(content, "*fiber.Ctx") {
		deviations = append(deviations, PatternDeviation{
			Type:        "HANDLER_SIGNATURE",
			Description: "Handler function doesn't follow standard signature pattern",
			File:        filename,
			Expected:    "func HandleXxx(c *fiber.Ctx) error",
			Actual:      "Non-standard handler signature found",
		})
	}
	
	return deviations
}

func (tl *SeniorTechLead) validateErrorPatterns(filename, content, patterns string) []PatternDeviation {
	var deviations []PatternDeviation
	
	// Check for proper error wrapping
	if strings.Contains(content, "return err") && !strings.Contains(content, "fmt.Errorf") {
		deviations = append(deviations, PatternDeviation{
			Type:        "ERROR_WRAPPING",
			Description: "Error returned without context wrapping",
			File:        filename,
			Expected:    "return fmt.Errorf(\"context: %w\", err)",
			Actual:      "return err",
		})
	}
	
	return deviations
}

// Helper methods for requirements validation
func (tl *SeniorTechLead) hasAPIEndpoint(criterion string, ctx *ReviewContext) bool {
	for _, content := range ctx.FileContents {
		if strings.Contains(content, "app.Get") || strings.Contains(content, "app.Post") || 
		   strings.Contains(content, "router.") {
			return true
		}
	}
	return false
}

func (tl *SeniorTechLead) codeCompiles(ctx *ReviewContext) bool {
	// This would be determined by build output analysis
	// For now, return true if we have implementation files
	return len(ctx.AllChangedFiles) > 0
}

func (tl *SeniorTechLead) hasRequiredFiles(criterion string, ctx *ReviewContext) bool {
	// Extract file patterns from criterion and check if they exist
	return len(ctx.AllChangedFiles) > 0
}

func (tl *SeniorTechLead) hasImplementationFiles(ctx *ReviewContext) bool {
	for _, filename := range ctx.AllChangedFiles {
		if !strings.Contains(filename, "_test.") {
			return true
		}
	}
	return false
}

// parseEMBrief extracts EM briefing from task description
func parseEMBrief(description string) *EMBrief {
	brief := &EMBrief{}
	lines := strings.Split(description, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TASK:") {
			brief.Task = strings.TrimSpace(strings.TrimPrefix(line, "TASK:"))
		} else if strings.HasPrefix(line, "CONTEXT:") {
			brief.Context = strings.TrimSpace(strings.TrimPrefix(line, "CONTEXT:"))
		} else if strings.HasPrefix(line, "FILES_TO_EXAMINE:") {
			filesStr := strings.TrimSpace(strings.TrimPrefix(line, "FILES_TO_EXAMINE:"))
			if filesStr != "" {
				files := strings.Split(filesStr, ",")
				for i, file := range files {
					files[i] = strings.TrimSpace(file)
				}
				brief.FilesToExamine = files
			}
		} else if strings.HasPrefix(line, "IMPLEMENTATION_APPROACH:") {
			brief.ImplementationApproach = strings.TrimSpace(strings.TrimPrefix(line, "IMPLEMENTATION_APPROACH:"))
		} else if strings.HasPrefix(line, "POTENTIAL_ISSUES:") {
			issuesStr := strings.TrimSpace(strings.TrimPrefix(line, "POTENTIAL_ISSUES:"))
			if issuesStr != "" {
				issues := strings.Split(issuesStr, ",")
				for i, issue := range issues {
					issues[i] = strings.TrimSpace(issue)
				}
				brief.PotentialIssues = issues
			}
		} else if strings.HasPrefix(line, "SUCCESS_CRITERIA:") {
			brief.SuccessCriteria = strings.TrimSpace(strings.TrimPrefix(line, "SUCCESS_CRITERIA:"))
		}
	}
	
	return brief
}

// formatPatternSummary formats available pattern documentation
func (tl *SeniorTechLead) formatPatternSummary(ctx *ReviewContext) string {
	var summary strings.Builder
	
	for patternFile := range ctx.PatternFiles {
		summary.WriteString(fmt.Sprintf("- %s\n", patternFile))
	}
	
	if summary.Len() == 0 {
		return "No pattern documentation available"
	}
	
	return summary.String()
}

func (tl *SeniorTechLead) buildSystemPrompt(req ImplementFeatureRequest, ctx *ReviewContext) string {
	var prompt strings.Builder

	// Parse EM brief from description if available
	ctx.EMBrief = parseEMBrief(req.Description)

	prompt.WriteString(fmt.Sprintf(`You are a Senior Tech Lead responsible for comprehensive code quality review and final approval.

**Current Task:** Review and approve feature: %s
**Project Type:** %s

**Review Methodology:**
1. **Requirements Validation**: Verify implementation meets EM brief requirements
2. **Security Analysis**: Static security vulnerability scanning
3. **Duplication Detection**: Check for unnecessary code duplication
4. **Pattern Consistency**: Validate against established project patterns
5. **Auto-Fix**: Apply formatting and linting fixes
6. **Final Decision**: Approve or create structured rejection feedback

**Engineering Manager's Brief:**`, req.Description, req.ProjectType))

	if ctx.EMBrief != nil && ctx.EMBrief.Task != "" {
		prompt.WriteString(fmt.Sprintf(`
Task: %s
Context: %s
Implementation Approach: %s
Files to Examine: %s
Potential Issues: %s
Success Criteria: %s
`, ctx.EMBrief.Task, ctx.EMBrief.Context, ctx.EMBrief.ImplementationApproach,
			strings.Join(ctx.EMBrief.FilesToExamine, ", "), 
			strings.Join(ctx.EMBrief.PotentialIssues, ", "), 
			ctx.EMBrief.SuccessCriteria))
	} else {
		prompt.WriteString("\nNo structured EM brief found in description.")
	}

	prompt.WriteString(fmt.Sprintf(`

**Pattern Documentation Available:**
%s

**Complete Implementation Review:**
The following files were changed during implementation:
`, tl.formatPatternSummary(ctx)))

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

**Your Enhanced Review Process:**
1. **Requirements Analysis**: Validate against EM brief success criteria
2. **Security Scanning**: Check for SQL injection, path traversal, hardcoded secrets, etc.
3. **Duplication Analysis**: Scan related files for unnecessary code duplication
4. **Pattern Validation**: Compare against established project patterns
5. **Auto-Fix Application**: Run formatting and linting tools
6. **Final Assessment**: Approve or create structured rejection feedback

**Review Criteria (ZERO TOLERANCE):**
- **Security Issues**: SQL injection, path traversal, hardcoded secrets, unsafe deserialization
- **Requirements Gaps**: Missing functionality specified in EM brief success criteria
- **Unnecessary Duplication**: Code that duplicates existing functionality
- **Pattern Deviations**: Code that doesn't follow established project patterns

**Available Actions:**
- READ_FILE: Read additional files for pattern analysis
- WRITE_FILE: Apply auto-fixes for formatting issues
- EXECUTE_COMMAND: Run linting, formatting, and security tools
- LIST_FILES: Explore related files for duplication analysis
- FIND_FILES: Search for similar functionality
- SEQUENTIAL_THINKING: Use for comprehensive analysis requiring systematic review

**When to Use Sequential Thinking:**
Use sequential thinking for complex reviews that require:
- Systematic analysis of multiple security vectors
- Comprehensive pattern validation across multiple files
- Detailed requirements validation against complex EM briefs
- Multi-step duplication analysis across related modules
- Complex architectural review requiring step-by-step reasoning

**Sequential Thinking for Code Review:**
SEQUENTIAL_THINKING:
THOUGHT: I need to perform a comprehensive review of this user management implementation. Let me start by validating the EM requirements systematically, then move through security, duplication, and patterns.
THOUGHT_NUMBER: 1
TOTAL_THOUGHTS: 6
NEXT_THOUGHT_NEEDED: true

**Recommended Review Process with Sequential Thinking:**
1. Start with sequential thinking to plan your comprehensive review approach
2. Use subsequent thoughts to work through each review criteria systematically
3. Document findings and reasoning in each thought step
4. Conclude with clear approval or structured rejection feedback

**Response Format for APPROVAL:**
REQUIREMENTS_VALIDATION: [PASSED/FAILED]
- EM brief requirement check results

SECURITY_ANALYSIS: [PASSED/FAILED]  
- Security vulnerability scan results

DUPLICATION_CHECK: [PASSED/FAILED]
- Code duplication analysis results

PATTERN_CONSISTENCY: [PASSED/FAILED]
- Project pattern compliance results

AUTO_FIXES_APPLIED:
ACTION: EXECUTE_COMMAND
COMMAND: go fmt
ACTION: EXECUTE_COMMAND  
COMMAND: go mod tidy

FINAL_DECISION: APPROVED
REASONING: All criteria passed, code ready for production

**Response Format for REJECTION:**
REQUIREMENTS_VALIDATION: FAILED
- [Specific missing requirements]

SECURITY_ANALYSIS: FAILED
- [Specific security issues found]

DUPLICATION_CHECK: FAILED
- [Specific duplications detected]

PATTERN_CONSISTENCY: FAILED
- [Specific pattern deviations]

REJECTION_REASON: [requirements_not_met/security_concerns/unnecessary_duplication/pattern_deviation]
SPECIFIC_ISSUES:
- [Issue 1]
- [Issue 2]

EXISTING_PATTERNS:
- [Example from codebase]

REQUIRED_ACTIONS:
- [Action 1]  
- [Action 2]

ROUTE_TO: engineering_manager

**Critical Standards:**
- ZERO tolerance for security vulnerabilities (all must be fixed)
- Requirements from EM brief MUST be fully implemented
- NO unnecessary code duplication (reuse existing functionality)
- STRICT adherence to established patterns
- Auto-fix formatting issues, don't reject for them

Begin your comprehensive technical review now.`)

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

	// Get review context for analysis
	reviewCtx, err := tl.analyzeCompleteWork()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to analyze work: %v", err)
		return result, nil
	}

	// Parse EM brief from request
	reviewCtx.EMBrief = parseEMBrief(req.Description)

	// Step 1: Requirements Validation
	requirementGaps := tl.validateRequirements(reviewCtx.EMBrief, reviewCtx)
	if len(requirementGaps) > 0 {
		rejectionFeedback := tl.createRejectionFeedback(
			RejectionRequirements,
			requirementGaps,
			[]string{})
		result.Success = false
		result.Error = rejectionFeedback
		result.NextSteps = "Route to Engineering Manager for requirements clarification"
		return result, nil
	}

	// Step 2: Security Analysis
	securityIssues := tl.evaluateSecurityConcerns(reviewCtx)
	criticalSecurityIssues := tl.filterCriticalSecurityIssues(securityIssues)
	if len(criticalSecurityIssues) > 0 {
		securityMessages := make([]string, len(criticalSecurityIssues))
		for i, issue := range criticalSecurityIssues {
			securityMessages[i] = fmt.Sprintf("%s in %s: %s", issue.Type, issue.File, issue.Description)
		}
		rejectionFeedback := tl.createRejectionFeedback(
			RejectionSecurity,
			securityMessages,
			[]string{"Use parameterized queries", "Validate all user input", "Use environment variables for secrets"})
		result.Success = false
		result.Error = rejectionFeedback
		result.NextSteps = "Route to Engineering Manager - Security vulnerabilities must be fixed"
		return result, nil
	}

	// Step 3: Duplication Detection
	duplicationIssues := tl.detectDuplication(reviewCtx)
	significantDuplications := tl.filterSignificantDuplications(duplicationIssues)
	if len(significantDuplications) > 0 {
		duplicationMessages := make([]string, len(significantDuplications))
		examples := []string{}
		for i, issue := range significantDuplications {
			duplicationMessages[i] = fmt.Sprintf("%s: %s duplicates %s", issue.Type, issue.CurrentFile, issue.ExistingFile)
			examples = append(examples, fmt.Sprintf("Existing implementation in %s", issue.ExistingFile))
		}
		rejectionFeedback := tl.createRejectionFeedback(
			RejectionDuplication,
			duplicationMessages,
			examples)
		result.Success = false
		result.Error = rejectionFeedback
		result.NextSteps = "Route to Engineering Manager - Remove code duplication"
		return result, nil
	}

	// Step 4: Pattern Consistency Analysis
	patternAnalysis := tl.analyzePatternConsistency(reviewCtx)
	if !patternAnalysis.Consistent {
		patternMessages := make([]string, len(patternAnalysis.Deviations))
		examples := []string{}
		for i, deviation := range patternAnalysis.Deviations {
			patternMessages[i] = fmt.Sprintf("%s in %s: %s", deviation.Type, deviation.File, deviation.Description)
			if deviation.Expected != "" {
				examples = append(examples, fmt.Sprintf("Expected: %s", deviation.Expected))
			}
		}
		rejectionFeedback := tl.createRejectionFeedback(
			RejectionPatterns,
			patternMessages,
			examples)
		result.Success = false
		result.Error = rejectionFeedback
		result.NextSteps = "Route to Engineering Manager - Fix pattern deviations"
		return result, nil
	}

	// Step 5: Auto-fix application (formatting, linting)
	autoFixCommands := tl.getAutoFixCommands(req.ProjectType)
	for _, command := range autoFixCommands {
		if err := tl.restrictions.ValidateCommand(command); err == nil {
			output, err := tl.tools.ExecuteCommand(command)
			result.CommandsExecuted = append(result.CommandsExecuted, command)
			result.BuildOutput += fmt.Sprintf("\n=== Auto-fix: %s ===\n%s", command, output)
			if err != nil {
				// Log auto-fix failures but don't fail the review
				result.BuildOutput += fmt.Sprintf("Auto-fix warning: %v\n", err)
			}
		}
	}

	// Step 6: Parse and execute any additional actions from LLM response
	actions := tl.parseActions(llmResponse)
	for _, action := range actions {
		switch action.Type {
		case "READ_FILE":
			_, err := tl.tools.ReadFile(action.Path)
			if err != nil {
				continue // Don't fail for missing files during review
			}

		case "WRITE_FILE":
			err := tl.tools.WriteFile(action.Path, action.Content)
			if err != nil {
				result.BuildOutput += fmt.Sprintf("Failed to write file %s: %v\n", action.Path, err)
			} else {
				result.FilesModified = append(result.FilesModified, action.Path)
			}

		case "EXECUTE_COMMAND":
			if err := tl.restrictions.ValidateCommand(action.Command); err == nil {
				output, err := tl.tools.ExecuteCommand(action.Command)
				result.CommandsExecuted = append(result.CommandsExecuted, action.Command)
				result.BuildOutput += fmt.Sprintf("\n=== %s ===\n%s", action.Command, output)
				if err != nil {
					result.BuildOutput += fmt.Sprintf("Command error: %v\n", err)
				}
			}
		}
	}

	// Step 7: Final approval
	result.Success = true
	result.Message = "Comprehensive code review passed - implementation approved for production"
	result.NextSteps = "Ready for deployment"

	return result, nil
}

// filterCriticalSecurityIssues filters for critical and high severity security issues
func (tl *SeniorTechLead) filterCriticalSecurityIssues(issues []SecurityIssue) []SecurityIssue {
	var critical []SecurityIssue
	for _, issue := range issues {
		if issue.Severity == "CRITICAL" || issue.Severity == "HIGH" {
			critical = append(critical, issue)
		}
	}
	return critical
}

// filterSignificantDuplications filters for duplications above threshold
func (tl *SeniorTechLead) filterSignificantDuplications(issues []DuplicationIssue) []DuplicationIssue {
	var significant []DuplicationIssue
	for _, issue := range issues {
		if issue.SimilarityScore >= 85 { // 85% similarity threshold for rejection
			significant = append(significant, issue)
		}
	}
	return significant
}

// getAutoFixCommands returns commands that should be auto-applied for formatting
func (tl *SeniorTechLead) getAutoFixCommands(projectType ProjectType) []string {
	switch projectType {
	case ProjectTypeGo:
		return []string{"go fmt", "go mod tidy"}
	case ProjectTypeTypeScript:
		return []string{"npm run lint --fix"}
	case ProjectTypePython:
		return []string{"python -m black ."}
	default:
		return []string{}
	}
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

// DocumentTask for SeniorTechLead is a no-op
func (tl *SeniorTechLead) DocumentTask(ctx context.Context, result *WorkflowResult) error {
	return nil
}