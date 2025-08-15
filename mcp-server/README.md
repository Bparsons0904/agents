# Senior Engineer MCP Agent - Proof of Concept

A single-agent MCP server that implements a Senior Engineer coding assistant. This PoC validates the core infrastructure before building the full multi-agent system.

## Features

- **Single Tool Interface**: `implement_feature` tool accessible via MCP protocol
- **LLM Integration**: Uses Ollama with Qwen3:14b model for code generation
- **Command Restrictions**: Configurable allowlist/blocklist for security
- **Project Support**: Go, TypeScript, and Python projects
- **File Operations**: Safe filesystem access within project boundaries
- **Git Integration**: Status, diff, and log operations
- **Build Validation**: Automatic build/test execution after implementation

## Architecture

```
mcp-server/
├── cmd/mcp-server/          # MCP server entry point
├── internal/
│   ├── agent/              # Senior Engineer implementation
│   ├── llm/                # Ollama client
│   ├── tools/              # Filesystem, git, and command tools
│   └── config/             # Configuration management
└── config/                 # Agent configuration files
```

## Usage

### Building and Running

```bash
# Build the server
go build -o mcp-server ./cmd/mcp-server

# Run with Docker Compose
docker-compose up mcp-server
```

### MCP Tool Usage

The server exposes a single tool `implement_feature`:

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

### Configuration

Configuration is loaded from `/app/config/agent.toml`:

```toml
[agent]
role = "senior_engineer"
model = "qwen3:14b"
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

Test the server locally:

```bash
# Check health
curl http://localhost:8080/health

# List tools
curl http://localhost:8080/tools

# Test implement_feature tool
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
```

## Success Criteria

- ✅ MCP Integration: Successfully exposes `implement_feature` tool
- ✅ Ollama Communication: Makes API calls to local Ollama instance  
- ✅ Command Restrictions: Validates commands against configuration
- ✅ File Operations: Read/write files safely within project directory
- ✅ Build Validation: Executes build commands and captures output
- ✅ Error Reporting: Clear error messages for debugging

## Next Steps

1. Add context file reading (CLAUDE.md, AGENTS.md)
2. Implement retry logic and iteration limits
3. Add additional agent roles (QA, Tech Lead, EM)
4. Build multi-agent orchestration workflow
5. Enhanced error recovery and routing between agents