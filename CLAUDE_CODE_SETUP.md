# Claude Code MCP Integration Setup

This guide shows how to integrate the Agent Workflow MCP server with Claude Code.

## Quick Setup

### 1. Start the Services
```bash
# Start Ollama and MCP services
cd /home/bobparsons/Development/agents
docker compose up -d
```

### 2. Configure Claude Code

Add this configuration to your Claude Code MCP settings:

**File**: `~/.claude/mcp_servers.json`
```json
{
  "mcpServers": {
    "agent-workflow": {
      "command": "/home/bobparsons/Development/agents/mcp-server/mcp-stdio",
      "args": [],
      "env": {
        "OLLAMA_URL": "http://localhost:11434",
        "PROJECT_ROOT": "/home/bobparsons/Development/agents/projects",
        "AGENT_DEBUG": "true",
        "AGENT_DEBUG_DIR": "/home/bobparsons/Development/agents/debug-logs"
      }
    }
  }
}
```

### 3. Test the Integration

In Claude Code, you can now use:

```
Use the implement_feature_workflow tool to add a /status endpoint to my Go API that returns the current server status and timestamp.
```

## Available Tools

### `implement_feature_workflow`
Complete feature implementation using multi-agent workflow (EM → Engineer → QA → Tech Lead)

**Parameters**:
- `description` (required): Feature description and requirements
- `project_type` (required): "go", "typescript", or "python"  
- `working_directory` (optional): Project root directory path

**Example Usage**:
```
Add a new GET /users endpoint that returns a JSON array of mock users with id, name, and email fields to my existing Go Fiber API.
```

## Model Configuration

**Current Setup**:
- **All Agents**: `qwen2.5-coder:14b-instruct-q6_K` (12GB VRAM)
- **Port**: 8765 (HTTP server) + stdio interface for Claude Code
- **Workflow**: EM → Engineer → QA → Tech Lead (2-5 minutes)

## Project Structure

**Expected Project Types**:
- **Go**: Fiber/Gin web APIs, console applications
- **TypeScript**: Node.js APIs, React applications  
- **Python**: FastAPI, Flask, Django applications

**Best Practices**:
- Use for **single feature additions** to existing projects
- Provide **specific, focused** requirements
- Include **project context** in working_directory

## Troubleshooting

### Services Not Running
```bash
# Check service status
docker ps
curl http://localhost:11434/api/tags  # Ollama
curl http://localhost:8765/health     # MCP Server
```

### Model Issues
```bash
# Check available models
docker exec agent-ollama ollama list

# Download missing models
docker exec agent-ollama ollama pull qwen2.5-coder:14b-instruct-q6_K
```

### Logs
```bash
# View agent workflow logs
docker logs agent-mcp-server --tail 20

# View debug logs
ls /home/bobparsons/Development/agents/debug-logs/
```

## Performance

**VRAM Requirements**: 12GB for the coding model
**Typical Duration**: 2-5 minutes for feature implementation  
**Success Rate**: High for focused, single-feature requests
**Quality**: Full multi-agent review process ensures robust implementations

## Integration Benefits

✅ **Quality Assurance**: Four-agent review process  
✅ **Context Awareness**: Reads existing project patterns  
✅ **Language Support**: Go, TypeScript, Python optimized  
✅ **Error Recovery**: Intelligent retry and fix mechanisms  
✅ **Testing**: Automatic test generation and validation  
✅ **Documentation**: Code documentation and comments