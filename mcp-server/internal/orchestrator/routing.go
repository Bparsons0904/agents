package orchestrator

import (
	"fmt"
	"strings"

	"mcp-server/internal/agent"
)

type RoutingDecision struct {
	NextAgent AgentRole
	Reason    string
	Priority  int // Higher priority takes precedence
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
	rules []RoutingRule
}

func NewRoutingEngine() *RoutingEngine {
	re := &RoutingEngine{}
	re.initializeRules()
	return re
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
				return !result.Success && re.isBuildError(result)
			},
			NextAgent: AgentRoleEngineer, // Stay in Engineer to fix build
			Reason:    "Build failed, fixing implementation",
			Priority:  15,
		},
		{
			FromAgent: AgentRoleEngineer,
			Condition: func(result *agent.ImplementFeatureResponse) bool {
				return !result.Success && re.isMissingDependency(result)
			},
			NextAgent: AgentRoleEM,
			Reason:    "Missing dependencies, need planning support",
			Priority:  10,
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
				return !result.Success && re.isTestFailure(result)
			},
			NextAgent: AgentRoleEngineer,
			Reason:    "Tests found implementation bugs, needs fixes",
			Priority:  15,
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

// Helper functions to analyze agent results

func (re *RoutingEngine) isBuildError(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	errorText := strings.ToLower(result.Error + " " + result.BuildOutput)
	buildErrors := []string{
		"build failed", "compilation error", "compile error",
		"syntax error", "undefined:", "not declared",
		"missing package", "import error", "cannot find",
	}
	
	for _, pattern := range buildErrors {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
}

func (re *RoutingEngine) isMissingDependency(result *agent.ImplementFeatureResponse) bool {
	if result.Error == "" {
		return false
	}
	
	errorText := strings.ToLower(result.Error)
	dependencyErrors := []string{
		"missing dependency", "package not found", "module not found",
		"cannot find module", "no such file or directory",
		"import path does not exist",
	}
	
	for _, pattern := range dependencyErrors {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
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
	
	errorText := strings.ToLower(result.Error + " " + result.BuildOutput)
	testFailures := []string{
		"test failed", "assertion failed", "assertion error",
		"expected", "actual", "test failure", "failed test",
		"tests failed", "spec failed",
	}
	
	for _, pattern := range testFailures {
		if strings.Contains(errorText, pattern) {
			return true
		}
	}
	
	return false
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