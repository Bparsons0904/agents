package agent

import (
	"context"
	"mcp-server/internal/config"
	"mcp-server/internal/tools"
	"time"
)

// Project and Agent Types
type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypeTypeScript ProjectType = "typescript" 
	ProjectTypePython     ProjectType = "python"
)

type AgentRole string

const (
	AgentRoleEM           AgentRole = "engineering_manager"
	AgentRoleEngineer     AgentRole = "senior_engineer"
	AgentRoleQA           AgentRole = "senior_qa"
	AgentRoleTechLead     AgentRole = "senior_tech_lead"
)

// Request and Response Types
type ImplementFeatureRequest struct {
	Description      string      `json:"description"`
	ProjectType      ProjectType `json:"project_type"`
	WorkingDirectory string      `json:"working_directory,omitempty"`
}

type ImplementFeatureResponse struct {
	Success          bool     `json:"success"`
	Message          string   `json:"message"`
	FilesModified    []string `json:"files_modified"`
	CommandsExecuted []string `json:"commands_executed"`
	BuildOutput      string   `json:"build_output"`
	NextSteps        string   `json:"next_steps"`
	Error            string   `json:"error,omitempty"`
}

// Workflow Types
type WorkflowRequest struct {
	Description      string      `json:"description"`
	ProjectType      ProjectType `json:"project_type"`
	WorkingDirectory string      `json:"working_directory"`
}

type WorkflowResult struct {
	Success          bool                        `json:"success"`
	CompletedPhases  []string                    `json:"completed_phases"`
	FilesModified    []string                    `json:"files_modified"`
	TestsAdded       []string                    `json:"tests_added"`
	QualityChecks    []string                    `json:"quality_checks"`
	BuildOutput      string                      `json:"build_output"`
	AgentSummaries   map[string]AgentSummary     `json:"agent_summaries"`
	WorkflowHistory  []AgentTransition           `json:"workflow_history"`
	NextSteps        string                      `json:"next_steps"`
	Error            string                      `json:"error,omitempty"`
	FailureReason    string                      `json:"failure_reason,omitempty"`
}

type AgentSummary struct {
	Role           string   `json:"role"`
	TaskCompleted  string   `json:"task_completed"`
	FilesChanged   []string `json:"files_changed"`
	Iterations     int      `json:"iterations"`
	Success        bool     `json:"success"`
}

type AgentTransition struct {
	FromAgent AgentRole `json:"from_agent"`
	ToAgent   AgentRole `json:"to_agent"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Core Interfaces
type Agent interface {
	ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error)
	DocumentTask(ctx context.Context, result *WorkflowResult) error
}

type WorkflowOrchestrator interface {
	ExecuteWorkflow(ctx context.Context, req WorkflowRequest) (*WorkflowResult, error)
	RegisterAgent(role AgentRole, agent Agent)
}

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type ToolSet interface {
	ReadFile(path string) (string, error)
	WriteFile(path, content string) error
	ExecuteCommand(command string) (string, error)
	GetGitStatus() (string, error)
	GetGitDiff() (string, error)
	GetGitLog(limit int) (string, error)
	SetWorkingDirectory(dir string)
	GetWorkingDirectory() string
	ListFiles(path string) ([]string, error)
	FindFiles(pattern string, searchPath string) ([]string, error)
	SearchForSolution(query string) (*tools.SearchResponse, error)
	SearchForError(errorMessage string) (*tools.SearchResponse, error)
}

type CommandRestrictions interface {
	IsAllowed(command string) bool
	ValidateCommand(command string) error
}

// Agent Factory Interface
type AgentFactory interface {
	CreateAgent(role AgentRole, llmClient LLMClient, toolSet ToolSet, restrictions CommandRestrictions, cfg config.WorkflowAgentConfig) (Agent, error)
}