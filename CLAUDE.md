# Claude Instructions for MCP Agent System

## Project Overview

This is a fully implemented MCP (Model Context Protocol) server featuring a multi-agent workflow system for comprehensive software development. The system uses Ollama with Qwen3:14b-q4_K_M for local LLM inference and provides secure, restricted code implementation capabilities through orchestrated agent collaboration.

## Architecture

- **MCP Server** (`mcp-server/`): Go-based HTTP server with multi-agent orchestration
- **Multi-Agent System**: Engineering Manager, Senior Engineer, Senior QA, Senior Tech Lead
- **Ollama Service**: Local LLM inference with Qwen3:14b-q4_K_M model
- **Docker Compose**: Orchestrates both services with shared networking
- **Security**: Command restrictions and filesystem boundaries

## Key Components

### MCP Server Structure
```
mcp-server/
├── cmd/mcp-server/          # HTTP server entry point
├── internal/
│   ├── agent/              # Multi-agent implementations (EM, Engineer, QA, Tech Lead)
│   ├── orchestrator/       # Workflow orchestration and routing engine
│   ├── llm/                # Ollama client integration
│   ├── tools/              # Filesystem, git, command tools
│   └── config/             # TOML configuration management
├── config/
│   ├── agent.toml          # Legacy single-agent configuration
│   └── agents.toml         # Multi-agent workflow configuration
└── Dockerfile              # Multi-stage build
```

### Important Files
- `docker-compose.yml`: Service orchestration with health checks
- `mcp-server/config/agents.toml`: Multi-agent workflow configuration
- `mcp-server/config/agent.toml`: Legacy single-agent configuration
- `mcp-server/internal/orchestrator/workflow.go`: Core workflow orchestration
- `mcp-server/internal/orchestrator/routing.go`: Smart agent routing engine
- `mcp-server/README.md`: Implementation documentation

### Multi-Agent Workflow
The system implements a complete software development workflow:

1. **Engineering Manager**: Analyzes requirements, reads project context (CLAUDE.md, AGENTS.md), creates implementation plans
2. **Senior Engineer**: Implements features based on EM plans, creates/modifies code files
3. **Senior QA Engineer**: Analyzes implementations via git diff, writes comprehensive tests
4. **Senior Tech Lead**: Reviews code quality, runs linters/formatters, validates architecture

**Workflow Flow**: EM → Engineer → QA → Tech Lead → Complete
**Smart Routing**: Dynamic agent transitions based on result analysis and error recovery

## Development Guidelines

### Go Code Standards
- Use camelCase for file names (per user preference)
- Follow existing patterns in agent/ and tools/ packages
- Implement proper error handling with context
- Use interfaces for testability (LLMClient, ToolSet, CommandRestrictions)

### Security Requirements
- ALL commands must be validated against allowlist in `agent.toml`
- File operations restricted to project directory scope
- No sudo, rm -rf, or system-level operations allowed
- Path traversal protection for all file access

### Configuration Management
- Use TOML for all configuration files
- Support both file-based and environment variable configuration
- Graceful fallbacks to sensible defaults
- Clear validation with helpful error messages

## Commands and Operations

### Docker Management
```bash
# Start Ollama only (for model downloads)
docker-compose up ollama

# Start both services
docker-compose up

# Build MCP server only
docker-compose build mcp-server

# View logs
docker logs agent-ollama
docker logs agent-mcp-server
```

### Testing Commands
```bash
# Health checks
curl http://localhost:11434/api/tags    # Ollama
curl http://localhost:8080/health       # MCP Server

# MCP tool discovery
curl http://localhost:8080/tools

# Legacy single-agent test
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature",
    "arguments": {
      "description": "Create a hello world function",
      "project_type": "go"
    }
  }
}'

# Multi-agent workflow test
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature_workflow",
    "arguments": {
      "description": "Create a Go Fiber web server with /health endpoint",
      "project_type": "go",
      "working_directory": "/app/test-projects"
    }
  }
}'
```

## Current State

### Completed Features ✅
- **Multi-Agent MCP Server**: Full workflow orchestration system
- **Four Specialized Agents**: Engineering Manager, Senior Engineer, Senior QA, Senior Tech Lead
- **Smart Routing Engine**: Dynamic agent transitions with 20+ decision rules
- **Dual-Mode Support**: Legacy single-agent + new multi-agent workflow
- **MCP Tools**: `implement_feature` (legacy) and `implement_feature_workflow` (multi-agent)
- **Ollama Integration**: Qwen3:14b-q4_K_M model with consistent references
- **Command Restriction System**: Per-agent security boundaries
- **Git Integration**: Context gathering, diff analysis, project history
- **Docker Deployment**: Containerized services with health checks
- **Configuration Management**: TOML-based agent and workflow configuration
- **Error Recovery**: Iteration limits, timeout handling, workflow diagnostics

### Model Status
- **Ollama Integration**: Qwen3:14b-q4_K_M model fully operational
- **Model Size**: ~9GB, downloads on first startup
- **Health Status**: All agents operational, 4 registered agents
- **Model Persistence**: Data persists in `ollama_data` volume
- **Performance**: Successful multi-agent workflow execution (tested)

### Testing Results
- **Multi-Agent Workflow**: Successfully tested with Product Manager feature request
- **Execution Time**: ~2m 8s for complete EM → Engineer workflow
- **Code Generation**: Functional Go Fiber web server with health endpoint created
- **Agent Collaboration**: EM planning → Engineer implementation working correctly
- **Iteration Management**: Proper limits and error handling operational

## Future Enhancements

### Potential Enhancements
1. **Iteration Limit Tuning**: Increase per-agent limits for complex features
2. **Full Workflow Completion**: QA and Tech Lead integration for complete pipeline
3. **Performance Optimization**: Parallel agent execution where possible
4. **Advanced Context**: Enhanced project analysis and pattern recognition
5. **Workspace Management**: Multi-project support and isolation

### Integration Points
- Claude Code MCP client integration
- VS Code extension support
- CI/CD pipeline integration
- Multi-project workspace support

## Troubleshooting

### Common Issues
1. **Ollama "unhealthy"**: Normal during model download, wait for completion
2. **MCP build failures**: Check Go module dependencies with `go mod tidy`
3. **Command restrictions**: Verify allowlist in `config/agent.toml`
4. **File access denied**: Ensure paths are within project directory

### Debug Commands
```bash
# Check container status
docker ps -a

# View detailed logs
docker logs agent-ollama --tail 50
docker logs agent-mcp-server --tail 50

# Test Ollama directly
docker exec agent-ollama ollama list
```

## Important Notes

### Security Considerations
- This is a development/testing environment
- Production deployments need additional security hardening
- GPU access required for optimal Ollama performance
- Network isolation recommended for production

### Performance
- **Model**: Qwen3:14b-q4_K_M requires ~9GB VRAM for optimal performance
- **Workflow Duration**: 2-5 minutes for typical multi-agent features
- **Memory Usage**: Efficient quantized model (q4_K_M) for resource optimization
- **Storage**: SSD recommended for model loading speed
- **Scaling**: Consider qwen3:8b for systems with limited VRAM

### Claude Code Integration
- Server runs on port 8080 for MCP protocol communication
- Standard JSON-RPC format for tool calls
- Designed for integration with Claude Code's MCP client