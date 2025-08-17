package tools

import (
	"context"
	"mcp-server/internal/config"
)

type ToolSet struct {
	filesystem        *FileSystem
	git               *GitOperations
	commands          *CommandValidator
	webSearch         *WebSearch
	projectInit       *ProjectInitializer
	sequentialThinking *SequentialThinkingTool
	workingDir        string
}

func NewToolSet(commands config.CommandsSection, restrictions config.RestrictionsSection, workingDir string) *ToolSet {
	if workingDir == "" {
		workingDir = "/app/projects" // Default
	}
	
	ts := &ToolSet{
		filesystem:        NewFileSystem(workingDir),
		git:               NewGitOperations(workingDir),
		commands:          NewCommandValidator(commands.Allowed, restrictions.BlockedPatterns, workingDir),
		webSearch:         NewWebSearch(),
		sequentialThinking: NewSequentialThinkingTool(),
		workingDir:        workingDir,
	}
	
	// Initialize project initializer with self-reference
	ts.projectInit = NewProjectInitializer(ts)
	
	return ts
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

func (ts *ToolSet) GetGitLog(limit int) (string, error) {
	return ts.git.GetLog(limit)
}

func (ts *ToolSet) GetGitDiffNameOnly() (string, error) {
	return ts.git.GetDiffNameOnly()
}

func (ts *ToolSet) GetGitDiffCached() (string, error) {
	return ts.git.GetDiffCached()
}

func (ts *ToolSet) GetGitShow(commitHash string) (string, error) {
	return ts.git.GetShow(commitHash)
}

func (ts *ToolSet) GetGitBranch() (string, error) {
	return ts.git.GetBranch()
}

func (ts *ToolSet) IsGitRepo() bool {
	return ts.git.IsGitRepo()
}

func (ts *ToolSet) SetWorkingDirectory(dir string) {
	ts.workingDir = dir
	ts.filesystem = NewFileSystem(dir)
	ts.git = NewGitOperations(dir)
	ts.commands = NewCommandValidator(ts.commands.allowed, ts.commands.blockedPatterns, dir)
	ts.projectInit = NewProjectInitializer(ts)
}

func (ts *ToolSet) GetWorkingDirectory() string {
	return ts.workingDir
}

func (ts *ToolSet) UpdateRestrictions(restrictions config.RestrictionsSection) {
	ts.commands = NewCommandValidator(ts.commands.allowed, restrictions.BlockedPatterns, ts.workingDir)
}

func (ts *ToolSet) IsAllowed(command string) bool {
	return ts.commands.IsAllowed(command)
}

func (ts *ToolSet) ValidateCommand(command string) error {
	return ts.commands.ValidateCommand(command)
}

func (ts *ToolSet) GetAllowedCommands() []string {
	return ts.commands.allowed
}

func (ts *ToolSet) ListFiles(path string) ([]string, error) {
	return ts.filesystem.ListFiles(path)
}

func (ts *ToolSet) FindFiles(pattern string, searchPath string) ([]string, error) {
	return ts.filesystem.FindFiles(pattern, searchPath)
}

func (ts *ToolSet) SearchForSolution(query string) (*SearchResponse, error) {
	return ts.webSearch.SearchForSolution(query)
}

func (ts *ToolSet) SearchForError(errorMessage string) (*SearchResponse, error) {
	return ts.webSearch.SearchForError(errorMessage)
}

func (ts *ToolSet) AnalyzeProject(projectPath string) (*ProjectAnalysis, error) {
	return ts.projectInit.AnalyzeProject(context.Background(), projectPath)
}

func (ts *ToolSet) GenerateProjectDocumentation(analysis *ProjectAnalysis, outputPath string) error {
	return ts.projectInit.GenerateProjectDocumentation(analysis, outputPath)
}

// Sequential thinking methods
func (ts *ToolSet) ProcessThought(args map[string]interface{}) (interface{}, error) {
	return ts.sequentialThinking.ProcessThought(args)
}

func (ts *ToolSet) GetThoughtHistory() *ThoughtHistory {
	return ts.sequentialThinking.GetThoughtHistory()
}

func (ts *ToolSet) ResetThoughts() {
	ts.sequentialThinking.Reset()
}