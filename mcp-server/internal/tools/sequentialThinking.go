package tools

import (
	"fmt"
	"strings"
)

// ThoughtData represents a single thought in the sequential thinking process
type ThoughtData struct {
	Thought           string  `json:"thought"`
	ThoughtNumber     int     `json:"thoughtNumber"`
	TotalThoughts     int     `json:"totalThoughts"`
	NextThoughtNeeded bool    `json:"nextThoughtNeeded"`
	IsRevision        *bool   `json:"isRevision,omitempty"`
	RevisesThought    *int    `json:"revisesThought,omitempty"`
	BranchFromThought *int    `json:"branchFromThought,omitempty"`
	BranchID          *string `json:"branchId,omitempty"`
	NeedsMoreThoughts *bool   `json:"needsMoreThoughts,omitempty"`
}

// ThoughtHistory tracks the sequential thinking process
type ThoughtHistory struct {
	Thoughts []ThoughtData            `json:"thoughts"`
	Branches map[string][]ThoughtData `json:"branches"`
}

// SequentialThinkingTool provides structured thinking capabilities
type SequentialThinkingTool struct {
	history *ThoughtHistory
}

// NewSequentialThinkingTool creates a new sequential thinking tool
func NewSequentialThinkingTool() *SequentialThinkingTool {
	return &SequentialThinkingTool{
		history: &ThoughtHistory{
			Thoughts: make([]ThoughtData, 0),
			Branches: make(map[string][]ThoughtData),
		},
	}
}

// ProcessThought handles a single thought in the sequential thinking process
func (st *SequentialThinkingTool) ProcessThought(args map[string]interface{}) (interface{}, error) {
	// Parse and validate thought data
	thoughtData, err := st.parseThoughtData(args)
	if err != nil {
		return nil, fmt.Errorf("invalid thought data: %w", err)
	}

	// Adjust total thoughts if current exceeds it
	if thoughtData.ThoughtNumber > thoughtData.TotalThoughts {
		thoughtData.TotalThoughts = thoughtData.ThoughtNumber
	}

	// Add to history
	st.history.Thoughts = append(st.history.Thoughts, thoughtData)

	// Handle branching
	if thoughtData.BranchFromThought != nil && thoughtData.BranchID != nil {
		if st.history.Branches[*thoughtData.BranchID] == nil {
			st.history.Branches[*thoughtData.BranchID] = make([]ThoughtData, 0)
		}
		st.history.Branches[*thoughtData.BranchID] = append(st.history.Branches[*thoughtData.BranchID], thoughtData)
	}

	// Format for logging (similar to the Node.js version)
	formattedThought := st.formatThought(thoughtData)
	
	// Return structured response
	response := map[string]interface{}{
		"thoughtNumber":        thoughtData.ThoughtNumber,
		"totalThoughts":        thoughtData.TotalThoughts,
		"nextThoughtNeeded":    thoughtData.NextThoughtNeeded,
		"branches":             st.getBranchNames(),
		"thoughtHistoryLength": len(st.history.Thoughts),
		"formattedThought":     formattedThought,
	}

	return response, nil
}

// parseThoughtData validates and parses the input arguments
func (st *SequentialThinkingTool) parseThoughtData(args map[string]interface{}) (ThoughtData, error) {
	var data ThoughtData

	// Required fields
	thought, ok := args["thought"].(string)
	if !ok || thought == "" {
		return data, fmt.Errorf("thought must be a non-empty string")
	}
	data.Thought = thought

	thoughtNumber, ok := args["thoughtNumber"].(float64)
	if !ok || thoughtNumber < 1 {
		return data, fmt.Errorf("thoughtNumber must be a positive integer")
	}
	data.ThoughtNumber = int(thoughtNumber)

	totalThoughts, ok := args["totalThoughts"].(float64)
	if !ok || totalThoughts < 1 {
		return data, fmt.Errorf("totalThoughts must be a positive integer")
	}
	data.TotalThoughts = int(totalThoughts)

	nextThoughtNeeded, ok := args["nextThoughtNeeded"].(bool)
	if !ok {
		return data, fmt.Errorf("nextThoughtNeeded must be a boolean")
	}
	data.NextThoughtNeeded = nextThoughtNeeded

	// Optional fields
	if isRevision, ok := args["isRevision"].(bool); ok {
		data.IsRevision = &isRevision
	}

	if revisesThought, ok := args["revisesThought"].(float64); ok {
		val := int(revisesThought)
		data.RevisesThought = &val
	}

	if branchFromThought, ok := args["branchFromThought"].(float64); ok {
		val := int(branchFromThought)
		data.BranchFromThought = &val
	}

	if branchID, ok := args["branchId"].(string); ok {
		data.BranchID = &branchID
	}

	if needsMoreThoughts, ok := args["needsMoreThoughts"].(bool); ok {
		data.NeedsMoreThoughts = &needsMoreThoughts
	}

	return data, nil
}

// formatThought creates a visually formatted representation of the thought
func (st *SequentialThinkingTool) formatThought(thoughtData ThoughtData) string {
	var prefix, context string

	if thoughtData.IsRevision != nil && *thoughtData.IsRevision {
		prefix = "🔄 Revision"
		if thoughtData.RevisesThought != nil {
			context = fmt.Sprintf(" (revising thought %d)", *thoughtData.RevisesThought)
		}
	} else if thoughtData.BranchFromThought != nil {
		prefix = "🌿 Branch"
		branchInfo := fmt.Sprintf(" (from thought %d", *thoughtData.BranchFromThought)
		if thoughtData.BranchID != nil {
			branchInfo += fmt.Sprintf(", ID: %s", *thoughtData.BranchID)
		}
		context = branchInfo + ")"
	} else {
		prefix = "💭 Thought"
		context = ""
	}

	header := fmt.Sprintf("%s %d/%d%s", prefix, thoughtData.ThoughtNumber, thoughtData.TotalThoughts, context)
	
	// Create a simple border
	borderLength := max(len(header), len(thoughtData.Thought)) + 4
	border := strings.Repeat("─", borderLength)
	
	return fmt.Sprintf(`
┌%s┐
│ %s │
├%s┤
│ %s │
└%s┘`, border, header, border, thoughtData.Thought, border)
}

// getBranchNames returns the names of all branches
func (st *SequentialThinkingTool) getBranchNames() []string {
	names := make([]string, 0, len(st.history.Branches))
	for name := range st.history.Branches {
		names = append(names, name)
	}
	return names
}

// GetThoughtHistory returns the complete thought history
func (st *SequentialThinkingTool) GetThoughtHistory() *ThoughtHistory {
	return st.history
}

// Reset clears the thought history
func (st *SequentialThinkingTool) Reset() {
	st.history.Thoughts = make([]ThoughtData, 0)
	st.history.Branches = make(map[string][]ThoughtData)
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}