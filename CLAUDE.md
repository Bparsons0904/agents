# Claude Instructions for MCP Agent System

## Project Overview

This is a proof-of-concept MCP (Model Context Protocol) server implementing a Senior Engineer coding assistant agent. The system uses Ollama with Qwen3:14b for local LLM inference and provides secure, restricted code implementation capabilities.

## Architecture

- **MCP Server** (`mcp-server/`): Go-based HTTP server exposing MCP tools
- **Ollama Service**: Local LLM inference with Qwen3:14b model
- **Docker Compose**: Orchestrates both services with shared networking
- **Security**: Command restrictions and filesystem boundaries

## Key Components

### MCP Server Structure
```
mcp-server/
├── cmd/mcp-server/          # HTTP server entry point
├── internal/
│   ├── agent/              # Senior Engineer agent implementation
│   ├── llm/                # Ollama client integration
│   ├── tools/              # Filesystem, git, command tools
│   └── config/             # TOML configuration management
├── config/agent.toml       # Agent behavior configuration
└── Dockerfile              # Multi-stage build
```

### Important Files
- `docker-compose.yml`: Service orchestration with health checks
- `mcp-server/config/agent.toml`: Command allowlist/blocklist configuration
- `mcp-server/README.md`: Implementation documentation

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

# Feature implementation test
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
```

## Current State

### Completed Features ✅
- MCP server with implement_feature tool
- Ollama integration with Qwen3:14b
- Command restriction system
- Filesystem and git operations
- Docker containerization
- Configuration management
- Security boundaries

### Model Status
- Ollama container running but may show "unhealthy" during model download
- Qwen3:14b model (~9GB) downloads on first startup
- Health check passes once model is available
- Model data persists in `ollama_data` volume

## Future Enhancements

### Planned Features
1. Context file reading (CLAUDE.md, AGENTS.md support)
2. Multi-agent orchestration (QA, Tech Lead, EM roles)
3. Retry logic and iteration limits
4. Enhanced error recovery
5. Workspace management for multiple projects

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
- Qwen3:14b requires ~9GB VRAM for optimal performance
- Consider qwen3:8b for systems with limited VRAM
- SSD storage recommended for model loading speed

### Claude Code Integration
- Server runs on port 8080 for MCP protocol communication
- Standard JSON-RPC format for tool calls
- Designed for integration with Claude Code's MCP client