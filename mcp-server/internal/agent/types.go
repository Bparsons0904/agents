package agent

import "context"

type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypeTypeScript ProjectType = "typescript" 
	ProjectTypePython     ProjectType = "python"
)

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

type Agent interface {
	ImplementFeature(ctx context.Context, req ImplementFeatureRequest) (*ImplementFeatureResponse, error)
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
	SetWorkingDirectory(dir string)
}

type CommandRestrictions interface {
	IsAllowed(command string) bool
	ValidateCommand(command string) error
}