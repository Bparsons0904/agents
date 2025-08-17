package orchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"mcp-server/internal/agent"
)

type RoutingDecision struct {
	NextAgent AgentRole
	Reason    string
	Priority  int // Higher priority takes precedence
}

type ErrorPattern struct {
	Pattern     *regexp.Regexp
	Category    string
	Severity    int // 1=low, 2=medium, 3=high, 4=critical
	Suggestions []string
}

type ErrorContext struct {
	Category     string
	Severity     int
	Suggestions  []string
	IsRecoverable bool
	RequiresHelp  bool
}

type RoutingRule struct {
	FromAgent    AgentRole
	Condition    func(*agent.ImplementFeatureResponse) bool
	NextAgent    AgentRole
	Reason       string
	Priority     int
}

// RoutingEngine handles complex agent routing logic
type RoutingEngine struct {
	rules        []RoutingRule
	errorPatterns []ErrorPattern
}

func NewRoutingEngine() *RoutingEngine {
	re := &RoutingEngine{}
	re.initializeErrorPatterns()
	re.initializeRules()
	return re
}

func (re *RoutingEngine) initializeErrorPatterns() {
	re.errorPatterns = []ErrorPattern{
		// Critical build errors
		{
			Pattern:  regexp.MustCompile(`(?i)(undefined:\s*\w+|undeclared name:\s*\w+|cannot find[\s\w]*:\s*\w+)`),
			Category: "undefined_symbol",
			Severity: 4,
			Suggestions: []string{
				"Check import statements",
				"Verify function/variable names",
				"Add missing declarations",
			},
		},
		{
			Pattern:  regexp.MustCompile(`(?i)(syntax error|unexpected \w+|expected \w+)`),
			Category: "syntax_error",
			Severity: 4,
			Suggestions: []string{
				"Check brackets, braces, and parentheses",
				"Verify function signatures",
				"Check for missing semicolons or commas",
			},
		},
		{
			Pattern:  regexp.MustCompile(`(?i)(import cycle|circular import|cyclic import)`),
			Category: "import_cycle",
			Severity: 3,
			Suggestions: []string{
				"Restructure package dependencies",
				"Create interface abstraction",
				"Move shared code to separate package",
			},
		},
		
		// Dependency and module errors
		{
			Pattern:  regexp.MustCompile(`(?i)(module\s+\w+\s+not found|no such file or directory|package \w+ is not in GOROOT)`),
			Category: "missing_dependency",
			Severity: 3,
			Suggestions: []string{
				"Run 'go mod tidy'",
				"Add missing dependency with 'go get'",
				"Check module path in go.mod",
			},
		},
		{
			Pattern:  regexp.MustCompile(`(?i)(permission denied|access denied|operation not permitted)`),
			Category: "permission_error",
			Severity: 3,
			Suggestions: []string{
				"Check file permissions",
				"Verify write access to target directory",
				"Run with appropriate privileges",
			},
		},
		
		// Test-related errors
		{
			Pattern:  regexp.MustCompile(`(?i)(test failed|assertion failed|panic: test timed out)`),
			Category: "test_failure",
			Severity: 2,
			Suggestions: []string{
				"Review test logic and assertions",
				"Check test data and setup",
				"Verify function behavior",
			},
		},
		{
			Pattern:  regexp.MustCompile(`(?i)(no tests to run|no test files|testing: warning: no tests to run)`),
			Category: "no_tests",
			Severity: 1,
			Suggestions: []string{
				"Create test files with *_test.go pattern",
				"Add test functions starting with 'Test'",
				"Check test file naming conventions",
			},
		},
		
		// Runtime and logic errors
		{
			Pattern:  regexp.MustCompile(`(?i)(panic:|runtime error|nil pointer dereference|index out of range)`),
			Category: "runtime_error",
			Severity: 4,
			Suggestions: []string{
				"Add nil checks",
				"Validate array/slice bounds",
				"Add error handling",
			},
		},
		{
			Pattern:  regexp.MustCompile(`(?i)(type \w+ has no field \w+|cannot use \w+ as \w+ value)`),
			Category: "type_error",
			Severity: 3,
			Suggestions: []string{
				"Check struct field names",
				"Verify type compatibility",
				"Add type conversions where needed",
			},
		},
		
		// Network and external service errors
		{
			Pattern:  regexp.MustCompile(`(?i)(connection refused|timeout|no route to host|dial tcp.*refused)`),
			Category: "network_error",
			Severity: 2,
			Suggestions: []string{
				"Check service availability",
				"Verify network connectivity",
				"Review endpoint URLs and ports",
			},
		},
		
		// Git and version control errors
		{
			Pattern:  regexp.MustCompile(`(?i)(fatal: not a git repository|git.*error|merge conflict)`),
			Category: "git_error",
			Severity: 2,
			Suggestions: []string{
				"Initialize git repository if needed",
				"Resolve merge conflicts",
				"Check git configuration",
			},
		},
	}
}

func (re *RoutingEngine) initializeRules() {
	re.rules = []RoutingRule{
		// Engineering Manager Rules
		{
			FromAgent: AgentRoleEM,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return result.Success
			},
			NextAgent: AgentRoleEngineer,
			Reason:    "Plan approved, starting implementation",
			Priority:  10,
		},
		{
			FromAgent: AgentRoleEM,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success
			},
			NextAgent: AgentRoleEM, // Stay in EM for replanning
			Reason:    "Planning failed, retrying",
			Priority:  5,
		},

		// Senior Engineer Rules
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return result.Success && len(result.FilesModified) > 0
			},
			NextAgent: AgentRoleQA,
			Reason:    "Implementation complete, needs testing",
			Priority:  20,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				// Task completed successfully without file changes usually means:
				// 1. It was a maintenance task (like dependency resolution)
				// 2. The implementation failed but agent reported success
				// Route to Tech Lead for final assessment
				return result.Success && len(result.FilesModified) == 0 && 
					   !strings.Contains(strings.ToLower(result.Message), "failed") &&
					   !strings.Contains(strings.ToLower(result.Error), "failed")
			},
			NextAgent: AgentRoleTechLead,
			Reason:    "Task completed without file changes, skip to quality review",
			Priority:  19,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "syntax_error" || errorCtx.Category == "undefined_symbol" || 
					   errorCtx.Category == "type_error" || errorCtx.Category == "import_cycle"
			},
			NextAgent: AgentRoleEngineer, // Stay in Engineer to fix critical build issues
			Reason:    "Critical build errors detected, continuing implementation fixes",
			Priority:  18,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "missing_dependency" || errorCtx.Category == "permission_error"
			},
			NextAgent: AgentRoleEM,
			Reason:    "Dependency or permission issues detected, need planning support",
			Priority:  15,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "runtime_error" && errorCtx.Severity >= 3
			},
			NextAgent: AgentRoleEngineer, // Stay to fix runtime issues
			Reason:    "Runtime errors detected, applying targeted fixes",
			Priority:  16,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "network_error" || errorCtx.Category == "git_error"
			},
			NextAgent: AgentRoleEM,
			Reason:    "External system issues detected, need guidance",
			Priority:  12,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success
			},
			NextAgent: AgentRoleEM,
			Reason:    "Implementation failed, need replanning",
			Priority:  5,
		},

		// Senior QA Engineer Rules
		{
			FromAgent: AgentRoleQA,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return result.Success && re.hasTestsAdded(result)
			},
			NextAgent: AgentRoleTechLead,
			Reason:    "Tests added and passing, ready for quality review",
			Priority:  20,
		},
		{
			FromAgent: AgentRoleQA,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "test_failure" && errorCtx.Severity >= 2
			},
			NextAgent: AgentRoleEngineer,
			Reason:    "Significant test failures found, implementation needs fixes",
			Priority:  17,
		},
		{
			FromAgent: AgentRoleQA,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				if result.Success {
					return false
				}
				errorCtx := re.analyzeErrorContext(result)
				return errorCtx.Category == "no_tests" || errorCtx.Severity <= 1
			},
			NextAgent: AgentRoleQA, // Stay to create proper tests
			Reason:    "Missing or insufficient tests, continuing test development",
			Priority:  10,
		},
		{
			FromAgent: AgentRoleQA,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success && re.isNonTestableCode(result)
			},
			NextAgent: AgentRoleTechLead,
			Reason:    "Code determined non-testable, skip to quality review",
			Priority:  10,
		},
		{
			FromAgent: AgentRoleQA,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success
			},
			NextAgent: AgentRoleEM,
			Reason:    "QA process failed, need guidance",
			Priority:  5,
		},

		// Senior Tech Lead Rules
		{
			FromAgent: AgentRoleTechLead,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return result.Success
			},
			NextAgent: AgentRoleTechLead, // Workflow complete (handled elsewhere)
			Reason:    "Quality review passed, workflow complete",
			Priority:  20,
		},
		{
			FromAgent: AgentRoleTechLead,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success && re.isQualityIssue(result)
			},
			NextAgent: AgentRoleEngineer,
			Reason:    "Quality issues found, need implementation fixes",
			Priority:  15,
		},
		{
			FromAgent: AgentRoleTechLead,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success && re.isStructuredRejection(result)
			},
			NextAgent: AgentRoleEM,
			Reason:    "Tech Lead structured rejection - routing to EM as requested",
			Priority:  20,
		},
		{
			FromAgent: AgentRoleTechLead,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success && re.isArchitectureIssue(result)
			},
			NextAgent: AgentRoleEM,
			Reason:    "Architecture concerns, need replanning",
			Priority:  15,
		},
		{
			FromAgent: AgentRoleTechLead,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success
			},
			NextAgent: AgentRoleEM,
			Reason:    "Tech lead review failed, need guidance",
			Priority:  5,
		},
	}
}

func (re *RoutingEngine) RouteAgent(currentAgent AgentRole, result *agent.ImplementFeatureResponse) (AgentRole, string, error) {
	var bestDecision *RoutingDecision
	
	for _, rule := range re.rules {
		if rule.FromAgent == currentAgent && rule.Condition(result) {
			decision := &RoutingDecision{
				NextAgent: rule.NextAgent,
				Reason:    rule.Reason,
				Priority:  rule.Priority,
			}
			
			if bestDecision == nil || decision.Priority > bestDecision.Priority {
				bestDecision = decision
			}
		}
	}
	
	if bestDecision == nil {
		return "", "", fmt.Errorf("no routing rule matched for agent %s with result: success=%v", 
			currentAgent, result.Success)
	}
	
	return bestDecision.NextAgent, bestDecision.Reason, nil
}

// Enhanced error analysis for smarter routing decisions
func (re *RoutingEngine) analyzeErrorContext(result *agent.ImplementFeatureResponse) ErrorContext {
	errorText := strings.ToLower(result.Error + " " + result.BuildOutput + " " + result.Message)
	
	// Find the most severe matching pattern
	var bestMatch *ErrorPattern
	for _, pattern := range re.errorPatterns {
		if pattern.Pattern.MatchString(errorText) {
			if bestMatch == nil || pattern.Severity > bestMatch.Severity {
				bestMatch = &pattern
			}
		}
	}
	
	if bestMatch == nil {
		// Default context for unmatched errors
		return ErrorContext{
			Category:     "unknown",
			Severity:     2,
			Suggestions:  []string{"Review error logs", "Check implementation logic"},
			IsRecoverable: true,
			RequiresHelp:  false,
		}
	}
	
	// Determine recoverability and help requirements based on category and severity
	isRecoverable := bestMatch.Severity <= 3
	requiresHelp := bestMatch.Severity >= 3 && 
		(bestMatch.Category == "missing_dependency" || 
		 bestMatch.Category == "permission_error" ||
		 bestMatch.Category == "network_error")
	
	return ErrorContext{
		Category:     bestMatch.Category,
		Severity:     bestMatch.Severity,
		Suggestions:  bestMatch.Suggestions,
		IsRecoverable: isRecoverable,
		RequiresHelp:  requiresHelp,
	}
}

// GetErrorSuggestions provides contextual suggestions for error resolution
func (re *RoutingEngine) GetErrorSuggestions(result *agent.ImplementFeatureResponse) []string {
	errorCtx := re.analyzeErrorContext(result)
	return errorCtx.Suggestions
}

// IsErrorRecoverable determines if an error can be recovered from without external help
func (re *RoutingEngine) IsErrorRecoverable(result *agent.ImplementFeatureResponse) bool {
	errorCtx := re.analyzeErrorContext(result)
	return errorCtx.IsRecoverable
}

// Helper functions to analyze agent results

func (re *RoutingEngine) isBuildError(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	// Use enhanced error analysis for more accurate detection
	errorCtx := re.analyzeErrorContext(result)
	buildCategories := []string{
		"syntax_error", "undefined_symbol", "type_error", 
		"import_cycle", "missing_dependency",
	}
	
	for _, category := range buildCategories {
		if errorCtx.Category == category {
			return true
		}
	}
	
	return false
}

func (re *RoutingEngine) isMissingDependency(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	// Use enhanced error analysis for more accurate detection
	errorCtx := re.analyzeErrorContext(result)
	return errorCtx.Category == "missing_dependency"
}

func (re *RoutingEngine) hasTestsAdded(result *agent.ImplementFeatureResponse) bool {
	for _, file := range result.FilesModified {
		if re.isTestFile(file) {
			return true
		}
	}
	return false
}

func (re *RoutingEngine) isTestFile(filename string) bool {
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

func (re *RoutingEngine) isTestFailure(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" && result.BuildOutput == "" {
		return false
	}
	
	// Use enhanced error analysis for more accurate detection
	errorCtx := re.analyzeErrorContext(result)
	return errorCtx.Category == "test_failure"
}

func (re *RoutingEngine) isNonTestableCode(result *agent.ImplementFeatureResponse) bool {
	if result.Message == "" {
		return false
	}
	
	messageText := strings.ToLower(result.Message)
	nonTestablePatterns := []string{
		"non-testable", "cannot test", "untestable",
		"no tests needed", "testing not applicable",
		"manual testing only", "ui only", "configuration only",
	}
	
	for _, pattern := range nonTestablePatterns {
		if strings.Contains(messageText, pattern) {
			return true
		}
	}
	
	return false
}

func (re *RoutingEngine) isQualityIssue(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	errorText := strings.ToLower(result.Error)
	qualityIssues := []string{
		"code quality", "lint", "format", "style",
		"naming convention", "complexity", "duplication",
		"security", "performance", "maintainability",
	}
	
	for _, pattern := range qualityIssues {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
}

func (re *RoutingEngine) isArchitectureIssue(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	errorText := strings.ToLower(result.Error)
	architectureIssues := []string{
		"architecture", "design pattern", "separation of concerns",
		"coupling", "cohesion", "dependency injection",
		"interface design", "api design", "structure",
	}
	
	for _, pattern := range architectureIssues {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
}

func (re *RoutingEngine) isStructuredRejection(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	errorText := strings.ToLower(result.Error)
	
	// Look for structured rejection patterns from enhanced Tech Lead
	structuredPatterns := []string{
		"rejection_reason:",
		"route_to: engineering_manager",
		"requirements_not_met",
		"security_concerns", 
		"unnecessary_duplication",
		"pattern_deviation",
	}
	
	for _, pattern := range structuredPatterns {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
}