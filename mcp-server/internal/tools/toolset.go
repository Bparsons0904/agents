package tools

import "mcp-server/internal/config"

type ToolSet struct {
	filesystem  *FileSystem
	git         *GitOperations
	commands    *CommandValidator
	workingDir  string
}

func NewToolSet(commands config.CommandsSection, restrictions config.RestrictionsSection, workingDir string) *ToolSet {
	if workingDir == "" {
		workingDir = "/app/projects" // Default
	}
	
	return &ToolSet{
		filesystem: NewFileSystem(workingDir),
		git:        NewGitOperations(workingDir),
		commands:   NewCommandValidator(commands.Allowed, restrictions.BlockedPatterns),
		workingDir: workingDir,
	}
}

func (ts *ToolSet) ReadFile(path string) (string, error) {
	return ts.filesystem.ReadFile(path)
}

func (ts *ToolSet) WriteFile(path, content string) error {
	return ts.filesystem.WriteFile(path, content)
}

func (ts *ToolSet) ExecuteCommand(command string) (string, error) {
	return ts.commands.ExecuteCommand(command)
}

func (ts *ToolSet) GetGitStatus() (string, error) {
	return ts.git.GetStatus()
}

func (ts *ToolSet) GetGitDiff() (string, error) {
	return ts.git.GetDiff()
}

func (ts *ToolSet) SetWorkingDirectory(dir string) {
	ts.workingDir = dir
	ts.filesystem = NewFileSystem(dir)
	ts.git = NewGitOperations(dir)
}

func (ts *ToolSet) UpdateRestrictions(restrictions config.RestrictionsSection) {
	ts.commands = NewCommandValidator(ts.commands.allowed, restrictions.BlockedPatterns)
}

func (ts *ToolSet) IsAllowed(command string) bool {
	return ts.commands.IsAllowed(command)
}

func (ts *ToolSet) ValidateCommand(command string) error {
	return ts.commands.ValidateCommand(command)
}