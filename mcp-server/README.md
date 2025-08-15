# Multi-Agent MCP Server

A fully implemented MCP (Model Context Protocol) server featuring a complete multi-agent workflow system for comprehensive software development. Includes Engineering Manager, Senior Engineer, Senior QA, and Senior Tech Lead agents working in orchestrated collaboration.

## Features

- **Dual-Mode Operation**: Legacy single-agent (`implement_feature`) + Multi-agent workflow (`implement_feature_workflow`)
- **Four Specialized Agents**: Engineering Manager, Senior Engineer, Senior QA Engineer, Senior Tech Lead
- **Workflow Orchestration**: Smart routing engine with 20+ decision rules and error recovery
- **LLM Integration**: Uses Ollama with Qwen3:14b-q4_K_M for reliable code generation
- **Command Restrictions**: Per-agent security boundaries and configurable allowlists
- **Project Support**: Go, TypeScript, and Python projects with language-specific tooling
- **Git Integration**: Context gathering, diff analysis, commit history, and project understanding
- **Quality Assurance**: Automated testing, code review, and linting integration

## Architecture

```
mcp-server/
├── cmd/mcp-server/          # MCP server entry point
├── internal/
│   ├── agent/              # Multi-agent implementations (EM, Engineer, QA, Tech Lead)
│   ├── orchestrator/       # Workflow orchestration and smart routing engine
│   ├── llm/                # Ollama client integration
│   ├── tools/              # Filesystem, git, and command tools
│   └── config/             # Configuration management
├── config/
│   ├── agent.toml          # Legacy single-agent configuration
│   └── agents.toml         # Multi-agent workflow configuration
└── Dockerfile              # Multi-stage Docker build
```

## Usage

### Building and Running

```bash
# Using Docker Compose (recommended)
docker compose up

# Build MCP server only
docker compose build --no-cache mcp-server

# Local development build
go build -o mcp-server ./cmd/mcp-server
```

### Health Checks

```bash
# Check Ollama model availability
curl http://localhost:11434/api/tags

# Check MCP server health (shows agent count and mode)
curl http://localhost:8080/health
```

### MCP Tool Usage

The server exposes two tools:

#### Legacy Single-Agent Tool
```json
{
  "name": "implement_feature",
  "arguments": {
    "description": "Add a new HTTP handler for user authentication",
    "project_type": "go",
    "working_directory": "/app/projects/my-project"
  }
}
```

#### Multi-Agent Workflow Tool
```json
{
  "name": "implement_feature_workflow", 
  "arguments": {
    "description": "Create a Go Fiber web server with /health endpoint",
    "project_type": "go",
    "working_directory": "/app/test-projects"
  }
}
```

### Configuration

Configuration is loaded from `/app/config/agent.toml`:

```toml
[agent]
role = "senior_engineer"
model = "qwen3:14b-q4_K_M"
max_tokens = 4000

[commands]
allowed = [
    "go build", "go test", "npm install", "npm run build",
    "python -m pytest", "git status", "git diff"
]

[restrictions]
blocked_patterns = [
    "sudo", "rm -rf", "chmod +x", "systemctl"
]
```

## API Endpoints

- `GET /tools` - List available MCP tools
- `POST /call` - Execute tool calls
- `GET /health` - Health check

## Security Features

- **Command Validation**: All commands are validated against allowlist/blocklist
- **Path Restriction**: File operations are restricted to project directory
- **Input Sanitization**: All inputs are validated before processing
- **Fail-Fast**: Single attempt with clear error reporting

## Testing

### Command Line Testing

```bash
# Check health and agent status
curl http://localhost:8080/health

# List available tools
curl http://localhost:8080/tools

# Test legacy single-agent mode
curl -X POST http://localhost:8080/call \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "implement_feature",
      "arguments": {
        "description": "Create a simple hello world function",
        "project_type": "go"
      }
    }
  }'

# Test multi-agent workflow (takes 2-5 minutes)
curl -X POST http://localhost:8080/call \
  -H "Content-Type: application/json" \
  -d '{
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

### Postman Testing

For easier testing and development, you can use Postman:

#### 1. Health Check
- **Method**: `GET`
- **URL**: `http://localhost:8080/health`
- **Expected Response**: 
  ```json
  {
    "status": "healthy",
    "mode": "multi-agent-workflow", 
    "agents": 4,
    "max_iterations": 7,
    "timeout_minutes": 15
  }
  ```

#### 2. List Available Tools
- **Method**: `GET` 
- **URL**: `http://localhost:8080/tools`
- **Expected Response**: Array with `implement_feature` and `implement_feature_workflow` tools

#### 3. Multi-Agent Workflow Test
- **Method**: `POST`
- **URL**: `http://localhost:8080/call`
- **Headers**: `Content-Type: application/json`
- **Body** (raw JSON):
  ```json
  {
    "method": "tools/call",
    "params": {
      "name": "implement_feature_workflow",
      "arguments": {
        "description": "As a Product Manager, I need a simple Go Fiber web server with a /health endpoint that returns JSON status. This will help us monitor service availability in production. The endpoint should return status: ok and a timestamp.",
        "project_type": "go",
        "working_directory": "/app/test-projects"
      }
    }
  }
  ```
- **Expected Duration**: 2-5 minutes
- **Expected Response**: Detailed workflow results with agent summaries, files modified, and execution history

#### 4. Legacy Single-Agent Test
- **Method**: `POST`
- **URL**: `http://localhost:8080/call` 
- **Headers**: `Content-Type: application/json`
- **Body** (raw JSON):
  ```json
  {
    "method": "tools/call",
    "params": {
      "name": "implement_feature",
      "arguments": {
        "description": "Create a simple main.go with hello world",
        "project_type": "go"
      }
    }
  }
  ```
- **Expected Duration**: 30-60 seconds
- **Expected Response**: Simple implementation result

## Success Criteria

### Multi-Agent System ✅
- ✅ **Four Agent Types**: Engineering Manager, Senior Engineer, Senior QA, Senior Tech Lead
- ✅ **Workflow Orchestration**: Complete EM → Engineer → QA → Tech Lead pipeline 
- ✅ **Smart Routing**: Dynamic agent transitions with 20+ decision rules
- ✅ **Dual-Mode Operation**: Legacy single-agent + multi-agent workflow tools
- ✅ **Tested & Operational**: Successfully tested with Product Manager feature requests

### Core Infrastructure ✅
- ✅ **MCP Integration**: Exposes both `implement_feature` and `implement_feature_workflow` tools
- ✅ **Ollama Communication**: Stable API calls to Qwen3:14b-q4_K_M model
- ✅ **Command Restrictions**: Per-agent security boundaries and validation
- ✅ **File Operations**: Safe filesystem access within project boundaries
- ✅ **Git Integration**: Context gathering, diff analysis, project history
- ✅ **Error Recovery**: Iteration limits, timeout handling, workflow diagnostics
- ✅ **Docker Deployment**: Containerized services with health checks

### Quality Assurance ✅
- ✅ **Code Generation**: Functional Go Fiber web server with health endpoint created
- ✅ **Agent Collaboration**: EM planning → Engineer implementation workflow validated
- ✅ **Performance**: 2m 8s execution time for complete multi-agent workflow
- ✅ **Configuration Management**: TOML-based multi-agent configuration system

## Production Readiness

The multi-agent MCP server is fully implemented and operational. Key features:

- **Agent Count**: 4 specialized agents with distinct roles
- **Workflow Duration**: 2-5 minutes for typical multi-agent features  
- **Model**: Qwen3:14b-q4_K_M quantized for efficient resource usage
- **Security**: Command restrictions and filesystem boundaries enforced
- **Integration**: Ready for Claude Code MCP client integration