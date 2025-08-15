package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"mcp-server/internal/agent"
	"mcp-server/internal/config"
	"mcp-server/internal/llm"
	"mcp-server/internal/tools"
)

type MCPServer struct {
	agent       agent.Agent
	config      *config.AgentConfig
	workingDir  string
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("/app/config/agent.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get working directory
	workingDir := os.Getenv("PROJECT_ROOT")
	if workingDir == "" {
		workingDir = "/app/projects"
	}

	// Initialize LLM client
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://ollama:11434"
	}
	
	llmClient := llm.NewOllamaClient(ollamaURL, cfg.Model)

	// Initialize tools
	toolSet := tools.NewToolSet(cfg.Commands, cfg.Restrictions, workingDir)

	// Initialize Senior Engineer agent
	engineer := agent.NewSeniorEngineer(llmClient, toolSet, toolSet)

	server := &MCPServer{
		agent:      engineer,
		config:     cfg,
		workingDir: workingDir,
	}

	// Register MCP endpoints
	http.HandleFunc("/tools", server.handleToolsRequest)
	http.HandleFunc("/call", server.handleToolCall)
	http.HandleFunc("/health", server.handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting MCP Server on port %s\n", port)
	fmt.Printf("Ollama URL: %s\n", ollamaURL)
	fmt.Printf("Working Directory: %s\n", workingDir)
	
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (s *MCPServer) handleToolsRequest(w http.ResponseWriter, r *http.Request) {
	tools := []MCPTool{
		{
			Name:        "implement_feature",
			Description: "Implement a software feature using Senior Engineer expertise",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Feature description and requirements",
					},
					"project_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"go", "typescript", "python"},
						"description": "Project type for language-specific handling",
					},
					"working_directory": map[string]interface{}{
						"type":        "string",
						"description": "Project root directory path",
					},
				},
				"required": []string{"description", "project_type"},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}

func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string `json:"method"`
		Params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		} `json:"params"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, 400, "Invalid JSON")
		return
	}

	if req.Params.Name != "implement_feature" {
		s.sendError(w, 404, "Tool not found")
		return
	}

	// Parse implement_feature request
	var implReq agent.ImplementFeatureRequest
	
	if desc, ok := req.Params.Arguments["description"].(string); ok {
		implReq.Description = desc
	} else {
		s.sendError(w, 400, "Missing or invalid description")
		return
	}

	if projType, ok := req.Params.Arguments["project_type"].(string); ok {
		implReq.ProjectType = agent.ProjectType(projType)
	} else {
		s.sendError(w, 400, "Missing or invalid project_type")
		return
	}

	if workDir, ok := req.Params.Arguments["working_directory"].(string); ok {
		implReq.WorkingDirectory = workDir
	} else {
		implReq.WorkingDirectory = s.workingDir
	}

	// Execute feature implementation
	result, err := s.agent.ImplementFeature(r.Context(), implReq)
	if err != nil {
		s.sendError(w, 500, fmt.Sprintf("Implementation failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{Result: result})
}

func (s *MCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"model":  s.config.Model,
	})
}

func (s *MCPServer) sendError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(MCPResponse{
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	})
}