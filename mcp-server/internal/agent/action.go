package agent

// Action represents a structured action that agents can parse from LLM responses
type Action struct {
	Type    string
	Path    string
	Content string
	Command string
}