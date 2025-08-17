# Agent Workflow Configuration Guide

## Global vs Per-Project Configuration

### ✅ Recommended: Global Configuration (Set Once)

**File**: `~/.claude/mcp_servers.json`
```json
{
  "mcpServers": {
    "agent-workflow-interactive": {
      "command": "/home/bobparsons/Development/agents/mcp-server/mcp-websocket",
      "args": [],
      "env": {
        "WS_PORT": "8766",
        "OLLAMA_URL": "http://localhost:11434",
        "AUTO_DETECT_PROJECT": "true",
        "FALLBACK_PROJECT_ROOT": "/home/bobparsons/Development",
        "AGENT_DEBUG_DIR": "/home/bobparsons/.claude/agent-debug-logs"
      }
    }
  }
}
```

### How Auto-Detection Works

1. **Claude Code Context**: When you use the agent in Claude Code, it will try to detect your current project
2. **Fallback Search**: If no specific project is detected, it uses `FALLBACK_PROJECT_ROOT` as the base directory
3. **Manual Override**: You can always specify `working_directory` in your request to override detection

### Configuration Options

#### Core Settings
- `WS_PORT`: WebSocket server port (default: 8766)
- `OLLAMA_URL`: Local Ollama instance URL (default: http://localhost:11434)

#### Project Detection
- `AUTO_DETECT_PROJECT`: Enable automatic project detection (true/false)
- `FALLBACK_PROJECT_ROOT`: Base directory to search for projects
- `WORKSPACE_ROOTS`: Comma-separated list of development directories (alternative to fallback)

#### Debug & Logging
- `AGENT_DEBUG_DIR`: Where to store agent debug logs (global location recommended)
- `AGENT_DEBUG`: Enable debug logging (true/false)
- `AGENT_DEBUG_VERBOSE`: Enable verbose debug output (true/false)

## Usage Examples

### Example 1: Auto-Detection (Most Common)
```
Claude Code: "Add a /health endpoint to my API"
Agent System: 
  → Detects current project from Claude Code workspace
  → Uses that project's directory
  → Implements feature
```

### Example 2: Specific Project Override
```
Claude Code: "Add user authentication to my backend API project"
Include in request: working_directory: "/home/bobparsons/Development/my-backend"
Agent System:
  → Uses specified directory
  → Ignores auto-detection
```

### Example 3: Multiple Development Areas
```json
{
  "env": {
    "WORKSPACE_ROOTS": "/home/bobparsons/Development,/home/bobparsons/Projects,/home/bobparsons/work"
  }
}
```

## Directory Structure Recommendations

### Recommended Setup
```
/home/bobparsons/
├── Development/              ← FALLBACK_PROJECT_ROOT
│   ├── project1/
│   ├── project2/
│   └── agents/              ← This agent system
├── .claude/
│   ├── mcp_servers.json     ← Global config
│   └── agent-debug-logs/    ← Centralized debug logs
```

### Benefits of This Structure
- ✅ **One-time setup**: Configure once, works everywhere
- ✅ **Auto-detection**: Finds your projects automatically  
- ✅ **Centralized logs**: All debug info in one place
- ✅ **Flexible**: Can override when needed
- ✅ **Multi-project**: Works across all your development work

## Alternative Configurations

### Per-Project Configuration (Not Recommended)
If you absolutely need per-project configs:

```json
{
  "mcpServers": {
    "agent-workflow-project1": {
      "command": "/home/bobparsons/Development/agents/mcp-server/mcp-websocket",
      "env": {
        "PROJECT_ROOT": "/home/bobparsons/Development/project1"
      }
    },
    "agent-workflow-project2": {
      "command": "/home/bobparsons/Development/agents/mcp-server/mcp-websocket", 
      "env": {
        "PROJECT_ROOT": "/home/bobparsons/Development/project2"
      }
    }
  }
}
```

### Docker-based Projects
For projects running in containers:

```json
{
  "env": {
    "FALLBACK_PROJECT_ROOT": "/home/bobparsons/Development",
    "DOCKER_SUPPORT": "true",
    "DOCKER_PROJECT_MOUNT": "/workspace"
  }
}
```

## Troubleshooting

### Project Not Detected
```bash
# Check what the agent system sees
curl "http://localhost:8766/health"

# Verify your fallback directory exists
ls -la /home/bobparsons/Development

# Test with explicit directory
# In Claude Code: "working_directory: /path/to/project"
```

### Debug Logs
```bash
# View recent debug activity
ls -la ~/.claude/agent-debug-logs/

# Check WebSocket server logs
# If running in background, check logs with docker logs or journalctl
```

### Multiple Ollama Instances
```json
{
  "env": {
    "OLLAMA_URL": "http://localhost:11434",
    "OLLAMA_BACKUP_URL": "http://localhost:11435"
  }
}
```

## Best Practices

### ✅ Do:
- Use global configuration with auto-detection
- Set up centralized debug logging
- Use descriptive project directory names
- Keep the agent system updated

### ❌ Avoid:
- Per-project MCP configurations (maintenance nightmare)
- Hardcoded absolute paths in requests
- Running multiple agent instances simultaneously
- Disabling debug logs (helpful for troubleshooting)

## Migration from Per-Project Setup

If you already have per-project configs:

1. **Backup existing config**: `cp ~/.claude/mcp_servers.json ~/.claude/mcp_servers.json.backup`
2. **Replace with global config**: Use the recommended global configuration above
3. **Test with one project**: Verify auto-detection works
4. **Remove per-project configs**: Clean up the old entries

The global configuration approach makes the agent system much more user-friendly and maintainable!