package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mcp-server/internal/agent"
	"mcp-server/internal/config"
)

// Use agent types directly
type AgentRole = agent.AgentRole

const (
	AgentRoleEM           = agent.AgentRoleEM
	AgentRoleEngineer     = agent.AgentRoleEngineer
	AgentRoleQA           = agent.AgentRoleQA
	AgentRoleTechLead     = agent.AgentRoleTechLead
)

type WorkflowOrchestrator struct {
	agents        map[AgentRole]agent.Agent
	llmClient     agent.LLMClient
	toolSet       agent.ToolSet
	config        *config.WorkflowConfig
	routingEngine *RoutingEngine
}

type WorkflowState struct {
	CurrentAgent    AgentRole
	IterationCounts map[AgentRole]int
	TaskDescription string
	ProjectContext  *ProjectContext
	WorkflowHistory []agent.AgentTransition
	StartTime       time.Time
}


type ProjectContext struct {
	GitStatus       string
	GitLog          string
	ClaudeMd        string
	AgentsMd        string
	WorkingDir      string
	ProjectType     agent.ProjectType
}

// Use agent package types
type WorkflowRequest = agent.WorkflowRequest
type WorkflowResult = agent.WorkflowResult
type AgentSummary = agent.AgentSummary
type AgentTransition = agent.AgentTransition

func NewWorkflowOrchestrator(llmClient agent.LLMClient, toolSet agent.ToolSet, config *config.WorkflowConfig) *WorkflowOrchestrator {
	return &WorkflowOrchestrator{
		agents:        make(map[AgentRole]agent.Agent),
		llmClient:     llmClient,
		toolSet:       toolSet,
		config:        config,
		routingEngine: NewRoutingEngine(),
	}
}

func (wo *WorkflowOrchestrator) RegisterAgent(role agent.AgentRole, agentInstance agent.Agent) {
	wo.agents[role] = agentInstance
}

func (wo *WorkflowOrchestrator) ExecuteWorkflow(ctx context.Context, req agent.WorkflowRequest) (*agent.WorkflowResult, error) {
	// Initialize workflow state
	state := &WorkflowState{
		CurrentAgent:    AgentRoleEM, // Always start with Engineering Manager
		IterationCounts: make(map[AgentRole]int),
		TaskDescription: req.Description,
		WorkflowHistory: []agent.AgentTransition{},
		StartTime:       time.Now(),
	}

	// Set working directory
	if req.WorkingDirectory != "" {
		wo.toolSet.SetWorkingDirectory(req.WorkingDirectory)
	}

	// Gather project context
	projectContext, err := wo.gatherProjectContext(req)
	if err != nil {
		return &WorkflowResult{
			Success:       false,
			Error:         fmt.Sprintf("Failed to gather project context: %v", err),
			FailureReason: "context_gathering_failed",
		}, nil
	}
	state.ProjectContext = projectContext

	// Initialize result
	result := &agent.WorkflowResult{
		Success:         true,
		CompletedPhases: []string{},
		FilesModified:   []string{},
		TestsAdded:      []string{},
		QualityChecks:   []string{},
		AgentSummaries:  make(map[string]AgentSummary),
		WorkflowHistory: []agent.AgentTransition{},
		NextSteps:       "Workflow completed successfully",
	}

	// Execute workflow loop
	for {
		// Check timeout
		if time.Since(state.StartTime) > time.Duration(wo.config.Workflow.TimeoutMinutes)*time.Minute {
			result.Success = false
			result.Error = "Workflow timeout exceeded"
			result.FailureReason = "timeout"
			break
		}

		// Check iteration limits
		if err := wo.checkIterationLimits(state); err != nil {
			result.Success = false
			result.Error = err.Error()
			result.FailureReason = "iteration_limit_exceeded"
			break
		}

		// Execute current agent with error recovery
		agentResult, err := wo.executeCurrentAgent(ctx, state, req)
		if err != nil {
			// Try to handle recoverable errors
			recoveryAction := wo.analyzeAndRecoverFromError(err, state)
			if recoveryAction.CanRecover {
				// Log the error and continue with recovery
				state.WorkflowHistory = append(state.WorkflowHistory, AgentTransition{
					FromAgent: state.CurrentAgent,
					ToAgent:   recoveryAction.NextAgent,
					Reason:    fmt.Sprintf("Error recovery: %s", recoveryAction.Reason),
					Timestamp: time.Now(),
				})
				state.CurrentAgent = recoveryAction.NextAgent
				continue
			}
			
			// Unrecoverable error
			result.Success = false
			result.Error = fmt.Sprintf("Agent execution failed: %v", err)
			result.FailureReason = wo.categorizeFailure(err)
			break
		}

		// Update result with agent output
		wo.updateResultWithAgent(result, state.CurrentAgent, agentResult)

		// Validate workflow health
		if err := wo.validateWorkflowHealth(state); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Workflow health check failed: %v", err)
			result.FailureReason = "workflow_health_failed"
			break
		}
		
		// Check if workflow is complete
		if wo.isWorkflowComplete(state, agentResult) {
			result.CompletedPhases = append(result.CompletedPhases, "workflow_complete")
			break
		}

		// Route to next agent
		nextAgent, reason, err := wo.routeToNextAgent(state, agentResult)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Routing failed: %v", err)
			result.FailureReason = "routing_failed"
			break
		}

		// Record transition
		transition := AgentTransition{
			FromAgent: state.CurrentAgent,
			ToAgent:   nextAgent,
			Reason:    reason,
			Timestamp: time.Now(),
		}
		state.WorkflowHistory = append(state.WorkflowHistory, transition)
		
		// Update state
		state.IterationCounts[state.CurrentAgent]++
		state.CurrentAgent = nextAgent
		// Pass the output of the previous agent as the task for the next one.
		if agentResult.NextSteps != "" {
			state.TaskDescription = agentResult.NextSteps
		}
	}

	// Finalize result with diagnostics
	result.WorkflowHistory = state.WorkflowHistory
	wo.enhanceResultWithDiagnostics(result, state)

	// If workflow was successful, call EM to document the task
	if result.Success {
		if em, ok := wo.agents[AgentRoleEM]; ok {
			if err := em.DocumentTask(ctx, result); err != nil {
				// Log the documentation failure, but don't fail the whole workflow
				log.Printf("EM failed to document task: %v", err)
			}
		}
	}
	
	return result, nil
}

func (wo *WorkflowOrchestrator) gatherProjectContext(req WorkflowRequest) (*ProjectContext, error) {
	ctx := &ProjectContext{
		WorkingDir:  req.WorkingDirectory,
		ProjectType: req.ProjectType,
	}

	// Get git status
	gitStatus, err := wo.toolSet.GetGitStatus()
	if err == nil {
		ctx.GitStatus = gitStatus
	} else {
		ctx.GitStatus = "No git repository or git error"
	}

	// Get git log
	gitLog, err := wo.toolSet.GetGitDiff() // Using existing GetGitDiff, will extend later
	if err == nil {
		ctx.GitLog = gitLog
	}

	// Try to read CLAUDE.md
	claudeMd, err := wo.toolSet.ReadFile("CLAUDE.md")
	if err == nil {
		ctx.ClaudeMd = claudeMd
	}

	// Try to read AGENTS.md  
	agentsMd, err := wo.toolSet.ReadFile("AGENTS.md")
	if err == nil {
		ctx.AgentsMd = agentsMd
	}

	return ctx, nil
}

func (wo *WorkflowOrchestrator) checkIterationLimits(state *WorkflowState) error {
	// Check total workflow iterations
	totalIterations := 0
	for _, count := range state.IterationCounts {
		totalIterations += count
	}
	
	if totalIterations >= wo.config.Workflow.MaxTotalIterations {
		return fmt.Errorf("maximum total iterations (%d) exceeded", wo.config.Workflow.MaxTotalIterations)
	}

	// Check current agent iteration limit
	maxForAgent := wo.getMaxIterationsForAgent(state.CurrentAgent)
	if state.IterationCounts[state.CurrentAgent] >= maxForAgent {
		return fmt.Errorf("maximum iterations for agent %s (%d) exceeded", state.CurrentAgent, maxForAgent)
	}

	return nil
}

func (wo *WorkflowOrchestrator) getMaxIterationsForAgent(role AgentRole) int {
	if agentCfg, exists := wo.config.Agents[string(role)]; exists {
		return agentCfg.MaxIterations
	}
	return 2 // default
}

func (wo *WorkflowOrchestrator) executeCurrentAgent(ctx context.Context, state *WorkflowState, req WorkflowRequest) (*agent.ImplementFeatureResponse, error) {
	currentAgent, exists := wo.agents[state.CurrentAgent]
	if !exists {
		return nil, fmt.Errorf("agent %s not registered", state.CurrentAgent)
	}

	// Convert workflow request to agent request
	agentReq := agent.ImplementFeatureRequest{
		Description:      wo.buildAgentPrompt(state, req),
		ProjectType:      req.ProjectType,
		WorkingDirectory: req.WorkingDirectory,
	}

	return currentAgent.ImplementFeature(ctx, agentReq)
}

func (wo *WorkflowOrchestrator) buildAgentPrompt(state *WorkflowState, req WorkflowRequest) string {
	// The TaskDescription in the state is the source of truth for the current task.
	// The EM updates this field, and subsequent agents use the updated description.
	basePrompt := state.TaskDescription

	// Add project context, which is always useful
	if state.ProjectContext != nil {
		if state.ProjectContext.ClaudeMd != "" {
			basePrompt += "\n\nProject Instructions (CLAUDE.md):\n" + state.ProjectContext.ClaudeMd
		}
		if state.ProjectContext.AgentsMd != "" {
			basePrompt += "\n\nAgent Instructions (AGENTS.md):\n" + state.ProjectContext.AgentsMd
		}
	}

	return basePrompt
}

func (wo *WorkflowOrchestrator) updateResultWithAgent(result *WorkflowResult, role AgentRole, agentResult *agent.ImplementFeatureResponse) {
	// Create agent summary
	summary := AgentSummary{
		Role:          string(role),
		TaskCompleted: agentResult.Message,
		FilesChanged:  agentResult.FilesModified,
		Iterations:    1, // Will be updated later
		Success:       agentResult.Success,
	}
	result.AgentSummaries[string(role)] = summary

	// Merge files modified
	result.FilesModified = append(result.FilesModified, agentResult.FilesModified...)

	// Append build output
	if agentResult.BuildOutput != "" {
		result.BuildOutput += fmt.Sprintf("\n=== %s Output ===\n%s", role, agentResult.BuildOutput)
	}

	// Add phase completion
	result.CompletedPhases = append(result.CompletedPhases, string(role))

	// Update specific fields based on agent type
	switch role {
	case AgentRoleQA:
		// QA agent adds tests
		result.TestsAdded = append(result.TestsAdded, agentResult.FilesModified...)
	case AgentRoleTechLead:
		// Tech Lead adds quality checks
		result.QualityChecks = append(result.QualityChecks, agentResult.CommandsExecuted...)
	}
}

func (wo *WorkflowOrchestrator) isWorkflowComplete(state *WorkflowState, agentResult *agent.ImplementFeatureResponse) bool {
	// Workflow is complete when Tech Lead finishes successfully
	return state.CurrentAgent == AgentRoleTechLead && agentResult.Success
}

func (wo *WorkflowOrchestrator) routeToNextAgent(state *WorkflowState, agentResult *agent.ImplementFeatureResponse) (AgentRole, string, error) {
	return wo.routingEngine.RouteAgent(state.CurrentAgent, agentResult)
}

// Error Recovery and Analysis

type RecoveryAction struct {
	CanRecover bool
	NextAgent  AgentRole
	Reason     string
}

func (wo *WorkflowOrchestrator) analyzeAndRecoverFromError(err error, state *WorkflowState) RecoveryAction {
	errorMsg := strings.ToLower(err.Error())
	
	// LLM connection errors - can often retry
	if strings.Contains(errorMsg, "connection") || strings.Contains(errorMsg, "timeout") {
		return RecoveryAction{
			CanRecover: true,
			NextAgent:  state.CurrentAgent, // Retry same agent
			Reason:     "Connection error, retrying",
		}
	}
	
	// Agent not registered - route to EM for replanning
	if strings.Contains(errorMsg, "not registered") {
		return RecoveryAction{
			CanRecover: true,
			NextAgent:  AgentRoleEM,
			Reason:     "Agent unavailable, replanning",
		}
	}
	
	// Tool/command validation errors - route to EM
	if strings.Contains(errorMsg, "command") || strings.Contains(errorMsg, "restricted") {
		return RecoveryAction{
			CanRecover: true,
			NextAgent:  AgentRoleEM,
			Reason:     "Command restrictions, need alternative approach",
		}
	}
	
	// File system errors - may be recoverable
	if strings.Contains(errorMsg, "file") || strings.Contains(errorMsg, "directory") {
		return RecoveryAction{
			CanRecover: true,
			NextAgent:  AgentRoleEM,
			Reason:     "File system issue, need planning support",
		}
	}
	
	// Context deadline/timeout - retry with EM
	if strings.Contains(errorMsg, "deadline") || strings.Contains(errorMsg, "context") {
		return RecoveryAction{
			CanRecover: true,
			NextAgent:  AgentRoleEM,
			Reason:     "Timeout occurred, simplifying approach",
		}
	}
	
	// Default: unrecoverable
	return RecoveryAction{
		CanRecover: false,
		Reason:     "Unrecoverable error",
	}
}

func (wo *WorkflowOrchestrator) categorizeFailure(err error) string {
	errorMsg := strings.ToLower(err.Error())
	
	if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline") {
		return "timeout"
	}
	if strings.Contains(errorMsg, "connection") {
		return "connection_failed"
	}
	if strings.Contains(errorMsg, "not registered") {
		return "agent_unavailable"
	}
	if strings.Contains(errorMsg, "command") || strings.Contains(errorMsg, "restricted") {
		return "command_restriction"
	}
	if strings.Contains(errorMsg, "file") || strings.Contains(errorMsg, "directory") {
		return "filesystem_error"
	}
	if strings.Contains(errorMsg, "config") {
		return "configuration_error"
	}
	if strings.Contains(errorMsg, "git") {
		return "git_error"
	}
	
	return "unknown_error"
}

// Workflow Health and Monitoring

func (wo *WorkflowOrchestrator) validateWorkflowHealth(state *WorkflowState) error {
	// Check for infinite loops
	if len(state.WorkflowHistory) > 3 {
		recentTransitions := state.WorkflowHistory[len(state.WorkflowHistory)-3:]
		if wo.detectLoop(recentTransitions) {
			return fmt.Errorf("detected workflow loop, aborting")
		}
	}
	
	// Check for excessive back-and-forth
	emCount := 0
	for i := len(state.WorkflowHistory) - 1; i >= 0 && i >= len(state.WorkflowHistory)-5; i-- {
		if state.WorkflowHistory[i].ToAgent == AgentRoleEM {
			emCount++
		}
	}
	if emCount > 3 {
		return fmt.Errorf("excessive EM interventions, workflow may be stuck")
	}
	
	return nil
}

func (wo *WorkflowOrchestrator) detectLoop(transitions []AgentTransition) bool {
	if len(transitions) < 3 {
		return false
	}
	
	// Check for A->B->A pattern
	for i := 0; i < len(transitions)-2; i++ {
		if transitions[i].FromAgent == transitions[i+2].FromAgent &&
		   transitions[i].ToAgent == transitions[i+2].ToAgent {
			return true
		}
	}
	
	return false
}

// Enhanced result processing

func (wo *WorkflowOrchestrator) enhanceResultWithDiagnostics(result *WorkflowResult, state *WorkflowState) {
	// Add workflow diagnostics
	if len(state.WorkflowHistory) > 0 {
		lastTransition := state.WorkflowHistory[len(state.WorkflowHistory)-1]
		result.NextSteps += fmt.Sprintf(" Last transition: %s -> %s (%s)", 
			lastTransition.FromAgent, lastTransition.ToAgent, lastTransition.Reason)
	}
	
	// Add performance metrics
	totalDuration := time.Since(state.StartTime)
	result.BuildOutput += fmt.Sprintf("\n\n=== Workflow Diagnostics ===\n")
	result.BuildOutput += fmt.Sprintf("Total duration: %v\n", totalDuration)
	result.BuildOutput += fmt.Sprintf("Total transitions: %d\n", len(state.WorkflowHistory))
	
	// Add iteration counts per agent
	for role, count := range state.IterationCounts {
		result.BuildOutput += fmt.Sprintf("%s iterations: %d\n", role, count)
	}
	
	// Update agent summaries with actual iteration counts
	for role, count := range state.IterationCounts {
		if summary, exists := result.AgentSummaries[string(role)]; exists {
			summary.Iterations = count
			result.AgentSummaries[string(role)] = summary
		}
	}
}