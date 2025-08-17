package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"mcp-server/internal/agent"
	"mcp-server/internal/config"
	"mcp-server/internal/debug"
	"mcp-server/internal/llm"
	"mcp-server/internal/orchestrator"
	"mcp-server/internal/tools"
)

type MCPServer struct {
	orchestrator   agent.WorkflowOrchestrator
	workflowConfig *config.WorkflowConfig
	toolSet        *tools.ToolSet
	workingDir     string
}

type MCPRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func main() {
	// Set up logging to stderr so it doesn't interfere with stdio
	log.SetOutput(os.Stderr)
	
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

	server := &MCPServer{
		workingDir: workingDir,
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
	
	// Create orchestrator
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
	
	// Register all agents
	agentFactory := agent.NewAgentFactory(debugLogger)
	for roleName, agentConfig := range workflowConfig.Agents {
		role := agent.AgentRole(agentConfig.Role)
		agentLLMClient := llm.NewOllamaClient(ollamaURL, agentConfig.Model)
		
		agentInstance, err := agentFactory.CreateAgent(role, agentLLMClient, toolSet, toolSet, agentConfig)
		if err != nil {
			log.Fatalf("Failed to create agent %s: %v", roleName, err)
		}
		
		orchestratorInstance.RegisterAgent(role, agentInstance)
	}
	
	server.orchestrator = orchestratorInstance
	
	log.Printf("MCP Server initialized in stdio mode")
	log.Printf("Ollama URL: %s", ollamaURL)
	log.Printf("Working Directory: %s", workingDir)
	
	// Start stdio message loop
	server.handleStdio()
}

func (s *MCPServer) handleStdio() {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(req.ID, -32700, "Parse error", err)
			continue
		}
		
		s.handleRequest(req)
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stdin: %v", err)
	}
}

func (s *MCPServer) handleRequest(req MCPRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.sendError(req.ID, -32601, "Method not found", req.Method)
	}
}

func (s *MCPServer) handleInitialize(req MCPRequest) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "agent-workflow-mcp",
			"version": "1.0.0",
		},
	}
	
	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleToolsList(req MCPRequest) {
	tools := []MCPTool{
		{
			Name:        "implement_feature_workflow",
			Description: "Complete feature implementation using multi-agent workflow (EM -> Engineer -> QA -> Tech Lead)",
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

	result := map[string]interface{}{
		"tools": tools,
	}
	
	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleToolsCall(req MCPRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params", err)
		return
	}
	
	switch params.Name {
	case "implement_feature_workflow":
		s.handleImplementFeatureWorkflow(req, params.Arguments)
	default:
		s.sendError(req.ID, -32601, "Tool not found", params.Name)
	}
}

func (s *MCPServer) handleImplementFeatureWorkflow(req MCPRequest, args map[string]interface{}) {
	// Parse workflow request
	var workflowReq agent.WorkflowRequest
	
	if desc, ok := args["description"].(string); ok {
		workflowReq.Description = desc
	} else {
		s.sendError(req.ID, -32602, "Missing or invalid description", nil)
		return
	}

	if projType, ok := args["project_type"].(string); ok {
		workflowReq.ProjectType = agent.ProjectType(projType)
	} else {
		s.sendError(req.ID, -32602, "Missing or invalid project_type", nil)
		return
	}

	if workDir, ok := args["working_directory"].(string); ok {
		workflowReq.WorkingDirectory = workDir
	} else {
		workflowReq.WorkingDirectory = s.workingDir
	}

	// Execute workflow
	result, err := s.orchestrator.ExecuteWorkflow(nil, workflowReq)
	if err != nil {
		s.sendError(req.ID, -32603, "Workflow execution failed", err.Error())
		return
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) sendResponse(id interface{}, result interface{}) {
	response := MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  result,
	}
	
	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}
	
	fmt.Println(string(data))
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	response := MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	
	responseData, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling error response: %v", err)
		return
	}
	
	fmt.Println(string(responseData))
}