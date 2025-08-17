package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"mcp-server/internal/agent"
	"mcp-server/internal/config"
	"mcp-server/internal/debug"
	"mcp-server/internal/llm"
	"mcp-server/internal/orchestrator"
	"mcp-server/internal/tools"
)

type MCPServer struct {
	// Legacy single agent support
	agent       agent.Agent
	config      *config.AgentConfig
	
	// New multi-agent workflow support
	orchestrator agent.WorkflowOrchestrator
	workflowConfig *config.WorkflowConfig
	
	// Shared toolset for project initialization
	toolSet      *tools.ToolSet
	
	workingDir   string
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

	// Try to load workflow configuration first
	workflowConfigPath := "/app/config/agents.toml"
	if _, err := os.Stat(workflowConfigPath); os.IsNotExist(err) {
		// Running locally, use local path
		workflowConfigPath = "config/agents.toml"
	}
	
	workflowConfig, err := config.LoadWorkflowConfig(workflowConfigPath)
	if err != nil {
		log.Printf("Failed to load workflow config from %s, falling back to single agent: %v", workflowConfigPath, err)
		
		// Fallback to legacy single agent configuration
		legacyConfigPath := "/app/config/agent.toml"
		if _, err := os.Stat(legacyConfigPath); os.IsNotExist(err) {
			legacyConfigPath = "config/agent.toml"
		}
		cfg, err := config.LoadConfig(legacyConfigPath)
		if err != nil {
			log.Fatalf("Failed to load any configuration: %v", err)
		}
		
		server.config = cfg
		
		// Initialize single agent setup
		llmClient := llm.NewOllamaClient(ollamaURL, cfg.Model)
		toolSet := tools.NewToolSet(cfg.Commands, cfg.Restrictions, workingDir)
		server.toolSet = toolSet
		// Create a config for the single engineer agent
		engineerConfig := config.WorkflowAgentConfig{
			Role:          cfg.Agent.Role,
			Model:         cfg.Agent.Model,
			PerAgentTimeoutMinutes: cfg.Agent.PerAgentTimeoutMinutes,
		}
		engineer := agent.NewSeniorEngineer(llmClient, toolSet, toolSet, engineerConfig)
		server.agent = engineer
		
		fmt.Println("Running in single-agent mode")
	} else {
		// Initialize multi-agent workflow
		server.workflowConfig = workflowConfig
		
		// Create shared toolset
		toolSet := tools.NewToolSet(workflowConfig.Commands, workflowConfig.Restrictions, workingDir)
		server.toolSet = toolSet
		
		// Create orchestrator
		// Use default model from first agent config for LLM client
		defaultModel := "qwen3:14b-q4_K_M"
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
		
		fmt.Println("Running in multi-agent workflow mode")
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
	fmt.Printf("=== DEBUG: New main.go code is running ===\n")
	
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (s *MCPServer) handleToolsRequest(w http.ResponseWriter, r *http.Request) {
	tools := []MCPTool{
		{
			Name:        "implement_feature",
			Description: "Implement a software feature using Senior Engineer expertise (legacy single-agent)",
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

	// Add workflow tool if orchestrator is available
	if s.orchestrator != nil {
		workflowTool := MCPTool{
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
		}
		tools = append(tools, workflowTool)
		
		// Add project initialization tool
		initTool := MCPTool{
			Name:        "initialize_project_patterns",
			Description: "Analyze existing project and generate pattern documentation for agent coordination",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the project to analyze",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Path where pattern documentation should be generated (optional, defaults to project_path)",
					},
				},
				"required": []string{"project_path"},
			},
		}
		tools = append(tools, initTool)
		
		// Add sequential thinking tool
		sequentialThinkingTool := MCPTool{
			Name:        "sequential_thinking",
			Description: "A detailed tool for dynamic and reflective problem-solving through sequential thoughts. Helps break down complex problems into manageable steps with revision and branching capabilities.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"thought": map[string]interface{}{
						"type":        "string",
						"description": "Your current thinking step",
					},
					"nextThoughtNeeded": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether another thought step is needed",
					},
					"thoughtNumber": map[string]interface{}{
						"type":        "integer",
						"description": "Current thought number",
						"minimum":     1,
					},
					"totalThoughts": map[string]interface{}{
						"type":        "integer",
						"description": "Estimated total thoughts needed",
						"minimum":     1,
					},
					"isRevision": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this revises previous thinking",
					},
					"revisesThought": map[string]interface{}{
						"type":        "integer",
						"description": "Which thought is being reconsidered",
						"minimum":     1,
					},
					"branchFromThought": map[string]interface{}{
						"type":        "integer",
						"description": "Branching point thought number",
						"minimum":     1,
					},
					"branchId": map[string]interface{}{
						"type":        "string",
						"description": "Branch identifier",
					},
					"needsMoreThoughts": map[string]interface{}{
						"type":        "boolean",
						"description": "If more thoughts are needed",
					},
				},
				"required": []string{"thought", "nextThoughtNeeded", "thoughtNumber", "totalThoughts"},
			},
		}
		tools = append(tools, sequentialThinkingTool)
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

	switch req.Params.Name {
	case "implement_feature":
		s.handleImplementFeature(w, r, req.Params.Arguments)
	case "implement_feature_workflow":
		s.handleImplementFeatureWorkflow(w, r, req.Params.Arguments)
	case "initialize_project_patterns":
		s.handleInitializeProjectPatterns(w, r, req.Params.Arguments)
	case "sequential_thinking":
		s.handleSequentialThinking(w, r, req.Params.Arguments)
	default:
		s.sendError(w, 404, "Tool not found")
	}
}

func (s *MCPServer) handleImplementFeature(w http.ResponseWriter, r *http.Request, args map[string]interface{}) {
	if s.agent == nil {
		s.sendError(w, 503, "Single agent not available")
		return
	}

	// Parse implement_feature request
	var implReq agent.ImplementFeatureRequest
	
	if desc, ok := args["description"].(string); ok {
		implReq.Description = desc
	} else {
		s.sendError(w, 400, "Missing or invalid description")
		return
	}

	if projType, ok := args["project_type"].(string); ok {
		implReq.ProjectType = agent.ProjectType(projType)
	} else {
		s.sendError(w, 400, "Missing or invalid project_type")
		return
	}

	if workDir, ok := args["working_directory"].(string); ok {
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

func (s *MCPServer) handleImplementFeatureWorkflow(w http.ResponseWriter, r *http.Request, args map[string]interface{}) {
	if s.orchestrator == nil {
		s.sendError(w, 503, "Workflow orchestrator not available")
		return
	}

	// Parse workflow request
	var workflowReq agent.WorkflowRequest
	
	if desc, ok := args["description"].(string); ok {
		workflowReq.Description = desc
	} else {
		s.sendError(w, 400, "Missing or invalid description")
		return
	}

	if projType, ok := args["project_type"].(string); ok {
		workflowReq.ProjectType = agent.ProjectType(projType)
	} else {
		s.sendError(w, 400, "Missing or invalid project_type")
		return
	}

	if workDir, ok := args["working_directory"].(string); ok {
		workflowReq.WorkingDirectory = workDir
	} else {
		workflowReq.WorkingDirectory = s.workingDir
	}

	// Execute workflow
	result, err := s.orchestrator.ExecuteWorkflow(r.Context(), workflowReq)
	if err != nil {
		s.sendError(w, 500, fmt.Sprintf("Workflow execution failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{Result: result})
}

func (s *MCPServer) handleInitializeProjectPatterns(w http.ResponseWriter, r *http.Request, args map[string]interface{}) {
	if s.toolSet == nil {
		s.sendError(w, 503, "ToolSet not available")
		return
	}

	// Parse project initialization request
	var projectPath string
	var outputPath string
	
	if path, ok := args["project_path"].(string); ok {
		projectPath = path
	} else {
		s.sendError(w, 400, "Missing or invalid project_path")
		return
	}

	if outPath, ok := args["output_path"].(string); ok {
		outputPath = outPath
	} else {
		outputPath = projectPath // Default to project path
	}

	// Analyze project patterns
	analysis, err := s.toolSet.AnalyzeProject(projectPath)
	if err != nil {
		s.sendError(w, 500, fmt.Sprintf("Project analysis failed: %v", err))
		return
	}

	// Generate documentation
	err = s.toolSet.GenerateProjectDocumentation(analysis, outputPath)
	if err != nil {
		s.sendError(w, 500, fmt.Sprintf("Documentation generation failed: %v", err))
		return
	}

	// Return analysis results
	result := map[string]interface{}{
		"success":         true,
		"project_path":    projectPath,
		"output_path":     outputPath,
		"analysis":        analysis,
		"patterns_found":  len(analysis.Patterns),
		"documentation_files": []string{
			outputPath + "/PROJECT_PATTERNS.md",
			outputPath + "/patterns/",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{Result: result})
}

func (s *MCPServer) handleSequentialThinking(w http.ResponseWriter, r *http.Request, args map[string]interface{}) {
	if s.toolSet == nil {
		s.sendError(w, 503, "ToolSet not available")
		return
	}

	// Process the thought using the sequential thinking tool
	result, err := s.toolSet.ProcessThought(args)
	if err != nil {
		s.sendError(w, 400, fmt.Sprintf("Sequential thinking failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{Result: result})
}

func (s *MCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status": "healthy",
	}

	if s.config != nil {
		status["mode"] = "single-agent"
		status["model"] = s.config.Model
	}
	
	if s.workflowConfig != nil {
		status["mode"] = "multi-agent-workflow"
		status["agents"] = len(s.workflowConfig.Agents)
		status["max_iterations"] = s.workflowConfig.Workflow.MaxTotalIterations
		status["timeout_minutes"] = s.workflowConfig.Workflow.TimeoutMinutes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
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