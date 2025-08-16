package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DebugLogger provides comprehensive debugging capabilities for agents
type DebugLogger struct {
	enabled     bool
	baseDir     string
	currentFile string
}

// AgentThought represents what an agent is thinking about
type AgentThought struct {
	Timestamp   time.Time `json:"timestamp"`
	Agent       string    `json:"agent"`
	Phase       string    `json:"phase"`
	Task        string    `json:"task"`
	Thinking    string    `json:"thinking"`
	Context     string    `json:"context"`
	PlanOfAction string   `json:"plan_of_action"`
}

// AgentAction represents what an agent actually does
type AgentAction struct {
	Timestamp   time.Time `json:"timestamp"`
	Agent       string    `json:"agent"`
	ActionType  string    `json:"action_type"`
	Command     string    `json:"command,omitempty"`
	FilePath    string    `json:"file_path,omitempty"`
	Content     string    `json:"content,omitempty"`
	Result      string    `json:"result"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// AgentDecision represents why an agent made a choice
type AgentDecision struct {
	Timestamp   time.Time `json:"timestamp"`
	Agent       string    `json:"agent"`
	Decision    string    `json:"decision"`
	Reasoning   string    `json:"reasoning"`
	Alternatives []string  `json:"alternatives"`
	Confidence  int       `json:"confidence"` // 1-10 scale
}

// NewDebugLogger creates a new debug logger
func NewDebugLogger(enabled bool, baseDir string) *DebugLogger {
	if baseDir == "" {
		baseDir = "/tmp/agent-debug"
	}
	
	// Create debug directory if it doesn't exist
	if enabled {
		os.MkdirAll(baseDir, 0755)
	}
	
	return &DebugLogger{
		enabled: enabled,
		baseDir: baseDir,
	}
}

// StartNewSession creates a new debug session file
func (dl *DebugLogger) StartNewSession(sessionID string) error {
	if !dl.enabled {
		return nil
	}
	
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("session-%s-%s.log", sessionID, timestamp)
	dl.currentFile = filepath.Join(dl.baseDir, filename)
	
	// Write session header
	header := fmt.Sprintf(`
=== AGENT DEBUGGING SESSION ===
Session ID: %s
Started: %s
File: %s

`, sessionID, time.Now().Format(time.RFC3339), dl.currentFile)
	
	return dl.writeToFile(header)
}

// LogThought logs what an agent is thinking
func (dl *DebugLogger) LogThought(thought AgentThought) error {
	if !dl.enabled {
		return nil
	}
	
	entry := fmt.Sprintf(`
[%s] 🧠 AGENT THOUGHT - %s
Phase: %s
Task: %s
Context: %s

💭 Thinking:
%s

📋 Plan of Action:
%s

---

`, thought.Timestamp.Format("15:04:05"), thought.Agent, thought.Phase, thought.Task, thought.Context, thought.Thinking, thought.PlanOfAction)
	
	return dl.writeToFile(entry)
}

// LogAction logs what an agent actually does
func (dl *DebugLogger) LogAction(action AgentAction) error {
	if !dl.enabled {
		return nil
	}
	
	status := "✅ SUCCESS"
	if !action.Success {
		status = "❌ FAILED"
	}
	
	entry := fmt.Sprintf(`
[%s] 🚀 AGENT ACTION - %s %s
Type: %s
`, action.Timestamp.Format("15:04:05"), action.Agent, status, action.ActionType)
	
	if action.Command != "" {
		entry += fmt.Sprintf("Command: %s\n", action.Command)
	}
	if action.FilePath != "" {
		entry += fmt.Sprintf("File: %s\n", action.FilePath)
	}
	if action.Content != "" {
		// Truncate very long content
		content := action.Content
		if len(content) > 500 {
			content = content[:500] + "... [truncated]"
		}
		entry += fmt.Sprintf("Content:\n%s\n", content)
	}
	
	entry += fmt.Sprintf("Result: %s\n", action.Result)
	
	if action.Error != "" {
		entry += fmt.Sprintf("Error: %s\n", action.Error)
	}
	
	entry += "---\n\n"
	
	return dl.writeToFile(entry)
}

// LogDecision logs why an agent made a decision
func (dl *DebugLogger) LogDecision(decision AgentDecision) error {
	if !dl.enabled {
		return nil
	}
	
	confidence := "🟢"
	if decision.Confidence < 7 {
		confidence = "🟡"
	}
	if decision.Confidence < 4 {
		confidence = "🔴"
	}
	
	entry := fmt.Sprintf(`
[%s] 🤔 AGENT DECISION - %s %s
Decision: %s
Confidence: %d/10

🎯 Reasoning:
%s

`, decision.Timestamp.Format("15:04:05"), decision.Agent, confidence, decision.Decision, decision.Confidence, decision.Reasoning)
	
	if len(decision.Alternatives) > 0 {
		entry += "🔄 Alternatives Considered:\n"
		for i, alt := range decision.Alternatives {
			entry += fmt.Sprintf("  %d. %s\n", i+1, alt)
		}
	}
	
	entry += "---\n\n"
	
	return dl.writeToFile(entry)
}

// LogError logs critical errors with context
func (dl *DebugLogger) LogError(agent string, phase string, err error, context string) error {
	if !dl.enabled {
		return nil
	}
	
	entry := fmt.Sprintf(`
[%s] 💥 CRITICAL ERROR - %s
Phase: %s
Context: %s
Error: %s

---

`, time.Now().Format("15:04:05"), agent, phase, context, err.Error())
	
	return dl.writeToFile(entry)
}

// LogRecoveryAttempt logs error recovery attempts
func (dl *DebugLogger) LogRecoveryAttempt(agent string, errorType string, recovery string, success bool) error {
	if !dl.enabled {
		return nil
	}
	
	status := "✅ RECOVERED"
	if !success {
		status = "❌ RECOVERY FAILED"
	}
	
	entry := fmt.Sprintf(`
[%s] 🔧 ERROR RECOVERY - %s %s
Error Type: %s
Recovery Strategy: %s

---

`, time.Now().Format("15:04:05"), agent, status, errorType, recovery)
	
	return dl.writeToFile(entry)
}

// LogWorkflowTransition logs when agents transition
func (dl *DebugLogger) LogWorkflowTransition(fromAgent, toAgent, reason string) error {
	if !dl.enabled {
		return nil
	}
	
	entry := fmt.Sprintf(`
[%s] 🔄 WORKFLOW TRANSITION
From: %s → To: %s
Reason: %s

---

`, time.Now().Format("15:04:05"), fromAgent, toAgent, reason)
	
	return dl.writeToFile(entry)
}

// GetCurrentLogFile returns the path to the current log file
func (dl *DebugLogger) GetCurrentLogFile() string {
	return dl.currentFile
}

// IsEnabled returns whether debugging is enabled
func (dl *DebugLogger) IsEnabled() bool {
	return dl.enabled
}

// writeToFile appends content to the current debug file
func (dl *DebugLogger) writeToFile(content string) error {
	if dl.currentFile == "" {
		return fmt.Errorf("no debug session started")
	}
	
	file, err := os.OpenFile(dl.currentFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	
	_, err = file.WriteString(content)
	return err
}

// DumpAgentState logs the complete state of an agent for debugging
func (dl *DebugLogger) DumpAgentState(agent string, state interface{}) error {
	if !dl.enabled {
		return nil
	}
	
	entry := fmt.Sprintf(`
[%s] 📊 AGENT STATE DUMP - %s
State: %+v

---

`, time.Now().Format("15:04:05"), agent, state)
	
	return dl.writeToFile(entry)
}