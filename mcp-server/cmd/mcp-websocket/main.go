package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mcp-server/internal/agent"
	"mcp-server/internal/config"
	"mcp-server/internal/debug"
	"mcp-server/internal/llm"
	"mcp-server/internal/orchestrator"
	"mcp-server/internal/tools"
)

type InteractiveServer struct {
	orchestrator   agent.WorkflowOrchestrator
	workflowConfig *config.WorkflowConfig
	toolSet        *tools.ToolSet
	workingDir     string
	
	// Session management
	sessions       map[string]*Session
	sessionsMutex  sync.RWMutex
	upgrader       websocket.Upgrader
}

type Session struct {
	ID           string
	WebSocket    *websocket.Conn
	CurrentAgent string
	Context      context.Context
	Cancel       context.CancelFunc
	PendingQuery *AgentQuery
	QueryChannel chan PMResponse
	CreatedAt    time.Time
	LastActivity time.Time
}

type AgentQuery struct {
	SessionID   string                 `json:"session_id"`
	Agent       string                 `json:"agent"`
	Question    string                 `json:"question"`
	Options     []string               `json:"options,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	TimeoutSec  int                    `json:"timeout_sec"`
}

type PMResponse struct {
	SessionID string `json:"session_id"`
	Answer    string `json:"answer"`
	Continue  bool   `json:"continue"`
}

type ProgressUpdate struct {
	SessionID    string                 `json:"session_id"`
	Type         string                 `json:"type"` // "progress", "query", "complete", "error"
	Agent        string                 `json:"agent,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Progress     int                    `json:"progress,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

type WebSocketRequest struct {
	Type      string                 `json:"type"` // "start_workflow", "pm_response"
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
}

func main() {
	// Set up logging
	log.SetOutput(os.Stderr)
	
	// Get configuration with dynamic project detection
	workingDir := os.Getenv("PROJECT_ROOT")
	autoDetect := os.Getenv("AUTO_DETECT_PROJECT") == "true"
	fallbackRoot := os.Getenv("FALLBACK_PROJECT_ROOT")
	
	if workingDir == "" {
		if autoDetect && fallbackRoot != "" {
			workingDir = fallbackRoot
			log.Printf("Using auto-detect mode with fallback: %s", workingDir)
		} else {
			workingDir = "/app/projects"
		}
	}

	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://ollama:11434"
	}

	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "8766"
	}

	server := &InteractiveServer{
		workingDir: workingDir,
		sessions:   make(map[string]*Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}

	// Load workflow configuration
	workflowConfigPath := "/app/config/agents.toml"
	if _, err := os.Stat(workflowConfigPath); os.IsNotExist(err) {
		workflowConfigPath = "config/agents.toml"
	}
	
	workflowConfig, err := config.LoadWorkflowConfig(workflowConfigPath)
	if err != nil {
		log.Fatalf("Failed to load workflow config: %v", err)
	}
	
	server.workflowConfig = workflowConfig
	
	// Create shared toolset
	toolSet := tools.NewToolSet(workflowConfig.Commands, workflowConfig.Restrictions, workingDir)
	server.toolSet = toolSet
	
	// Create orchestrator with interactive capabilities
	defaultModel := "qwen2.5-coder:14b-instruct-q6_K"
	if len(workflowConfig.Agents) > 0 {
		for _, agentCfg := range workflowConfig.Agents {
			defaultModel = agentCfg.Model
			break
		}
	}
	
	llmClient := llm.NewOllamaClient(ollamaURL, defaultModel)
	orchestratorInstance := orchestrator.NewWorkflowOrchestrator(llmClient, toolSet, workflowConfig)
	
	// Initialize debug logger
	debugConfig := config.GetDebugConfig()
	debugLogger := debug.NewDebugLogger(debugConfig.Enabled, debugConfig.LogDir)
	
	// Register all agents with interactive callbacks
	agentFactory := agent.NewAgentFactory(debugLogger)
	for roleName, agentConfig := range workflowConfig.Agents {
		role := agent.AgentRole(agentConfig.Role)
		agentLLMClient := llm.NewOllamaClient(ollamaURL, agentConfig.Model)
		
		agentInstance, err := agentFactory.CreateAgent(role, agentLLMClient, toolSet, toolSet, agentConfig)
		if err != nil {
			log.Fatalf("Failed to create agent %s: %v", roleName, err)
		}
		
		// Wrap agent with interactive capabilities
		interactiveAgent := server.wrapAgentWithInteractive(agentInstance, string(role))
		orchestratorInstance.RegisterAgent(role, interactiveAgent)
	}
	
	server.orchestrator = orchestratorInstance
	
	// Start session cleanup routine
	go server.sessionCleanup()
	
	// Setup HTTP routes
	http.HandleFunc("/ws", server.handleWebSocket)
	http.HandleFunc("/health", server.handleHealth)
	
	log.Printf("Interactive MCP Server starting on port %s", port)
	log.Printf("Ollama URL: %s", ollamaURL)
	log.Printf("Working Directory: %s", workingDir)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (s *InteractiveServer) wrapAgentWithInteractive(agent agent.Agent, agentName string) agent.Agent {
	// This would be implemented to intercept agent decisions and ask PM when needed
	// For now, return the original agent - full implementation would require 
	// modifying the agent interface to support interactive callbacks
	return agent
}

func (s *InteractiveServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	
	session := &Session{
		ID:           sessionID,
		WebSocket:    conn,
		Context:      ctx,
		Cancel:       cancel,
		QueryChannel: make(chan PMResponse, 1),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	
	s.sessionsMutex.Lock()
	s.sessions[sessionID] = session
	s.sessionsMutex.Unlock()
	
	log.Printf("New WebSocket session: %s", sessionID)
	
	// Send session info
	s.sendUpdate(session, ProgressUpdate{
		SessionID: sessionID,
		Type:      "session_started",
		Message:   "Interactive session established",
		Data: map[string]interface{}{
			"session_id": sessionID,
			"capabilities": []string{"workflow", "progress_updates", "pm_queries"},
		},
	})
	
	// Handle WebSocket messages
	for {
		var req WebSocketRequest
		if err := conn.ReadJSON(&req); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		
		session.LastActivity = time.Now()
		
		switch req.Type {
		case "start_workflow":
			go s.handleWorkflowRequest(session, req.Data)
		case "pm_response":
			s.handlePMResponse(session, req)
		default:
			s.sendUpdate(session, ProgressUpdate{
				SessionID: sessionID,
				Type:      "error",
				Message:   fmt.Sprintf("Unknown request type: %s", req.Type),
			})
		}
	}
	
	// Cleanup
	s.sessionsMutex.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMutex.Unlock()
	cancel()
	
	log.Printf("WebSocket session closed: %s", sessionID)
}

func (s *InteractiveServer) handleWorkflowRequest(session *Session, data map[string]interface{}) {
	// Parse workflow request
	var workflowReq agent.WorkflowRequest
	
	if desc, ok := data["description"].(string); ok {
		workflowReq.Description = desc
	} else {
		s.sendUpdate(session, ProgressUpdate{
			SessionID: session.ID,
			Type:      "error",
			Message:   "Missing or invalid description",
		})
		return
	}

	if projType, ok := data["project_type"].(string); ok {
		workflowReq.ProjectType = agent.ProjectType(projType)
	} else {
		s.sendUpdate(session, ProgressUpdate{
			SessionID: session.ID,
			Type:      "error",
			Message:   "Missing or invalid project_type",
		})
		return
	}

	if workDir, ok := data["working_directory"].(string); ok {
		workflowReq.WorkingDirectory = workDir
	} else {
		// Try to detect project from Claude Code context or use fallback
		if detectedProject := s.detectProjectDirectory(data); detectedProject != "" {
			workflowReq.WorkingDirectory = detectedProject
		} else {
			workflowReq.WorkingDirectory = s.workingDir
		}
	}

	// Send workflow started
	s.sendUpdate(session, ProgressUpdate{
		SessionID: session.ID,
		Type:      "progress",
		Agent:     "orchestrator",
		Status:    "workflow_started",
		Progress:  0,
		Message:   "Multi-agent workflow initiated",
		Data: map[string]interface{}{
			"description":       workflowReq.Description,
			"project_type":      workflowReq.ProjectType,
			"working_directory": workflowReq.WorkingDirectory,
		},
	})

	// Execute workflow with progress updates
	result, err := s.orchestrator.ExecuteWorkflow(session.Context, workflowReq)
	
	if err != nil {
		s.sendUpdate(session, ProgressUpdate{
			SessionID: session.ID,
			Type:      "error",
			Message:   fmt.Sprintf("Workflow execution failed: %v", err),
		})
		return
	}

	// Send completion
	s.sendUpdate(session, ProgressUpdate{
		SessionID: session.ID,
		Type:      "complete",
		Progress:  100,
		Message:   "Workflow completed successfully",
		Data: map[string]interface{}{
			"result": result,
		},
	})
}

func (s *InteractiveServer) handlePMResponse(session *Session, req WebSocketRequest) {
	if responseData, ok := req.Data["response"].(string); ok {
		response := PMResponse{
			SessionID: session.ID,
			Answer:    responseData,
			Continue:  true,
		}
		
		// Send response to waiting agent
		select {
		case session.QueryChannel <- response:
			log.Printf("PM response sent to agent: %s", responseData)
		default:
			log.Printf("No agent waiting for PM response")
		}
	}
}

func (s *InteractiveServer) sendUpdate(session *Session, update ProgressUpdate) {
	update.SessionID = session.ID
	
	if err := session.WebSocket.WriteJSON(update); err != nil {
		log.Printf("Failed to send update to session %s: %v", session.ID, err)
	}
}

func (s *InteractiveServer) detectProjectDirectory(data map[string]interface{}) string {
	// Check if Claude Code provided a project context
	if projectHint, ok := data["project_hint"].(string); ok {
		return projectHint
	}
	
	// Check if there's a current_directory in the context
	if currentDir, ok := data["current_directory"].(string); ok {
		return currentDir
	}
	
	// For now, return empty - in a full implementation, this could:
	// - Look for git repositories in common development directories
	// - Check recent Claude Code workspace history
	// - Scan for project files (package.json, go.mod, etc.)
	return ""
}

func (s *InteractiveServer) sessionCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		s.sessionsMutex.Lock()
		for sessionID, session := range s.sessions {
			if time.Since(session.LastActivity) > 30*time.Minute {
				log.Printf("Cleaning up inactive session: %s", sessionID)
				session.Cancel()
				session.WebSocket.Close()
				delete(s.sessions, sessionID)
			}
		}
		s.sessionsMutex.Unlock()
	}
}

func (s *InteractiveServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sessionsMutex.RLock()
	sessionCount := len(s.sessions)
	s.sessionsMutex.RUnlock()
	
	status := map[string]interface{}{
		"status":         "healthy",
		"mode":           "interactive-websocket",
		"agents":         len(s.workflowConfig.Agents),
		"active_sessions": sessionCount,
		"websocket_endpoint": "/ws",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}