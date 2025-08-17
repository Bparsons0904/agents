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
- `mcp-server/internal/orchestrator/routing.go`: Smart agent routing engine with structured rejection handling
- `mcp-server/internal/agent/manager.go`: Enhanced EM with structured briefing format
- `mcp-server/internal/agent/engineer.go`: Enhanced Engineer with brief parsing and error categorization
- `mcp-server/internal/agent/techlead.go`: Comprehensive Tech Lead with security analysis and pattern validation
- `agents/AGENTS.md`: Agent knowledge base and coordination patterns
- `agents/patterns/`: Pattern documentation for consistency analysis
- `mcp-server/README.md`: Implementation documentation

### Multi-Agent Workflow

The system implements a complete software development workflow:

1. **Engineering Manager**: Analyzes requirements, reads project context (CLAUDE.md, AGENTS.md), creates structured implementation briefs
2. **Senior Engineer**: Implements features based on EM structured briefs with enhanced error categorization
3. **Senior QA Engineer**: Analyzes implementations via git diff, writes comprehensive tests
4. **Senior Tech Lead**: Comprehensive quality review with security analysis, pattern validation, and structured rejections

**Workflow Flow**: EM → Engineer → QA → Tech Lead → Complete
**Smart Routing**: Dynamic agent transitions with enhanced coordination and structured feedback loops

### Enhanced Tech Lead Capabilities

The Tech Lead agent has been significantly enhanced with comprehensive review capabilities:

#### 🔒 Security Analysis (Zero Tolerance)
- **SQL Injection Detection**: Identifies string concatenation in SQL queries
- **Path Traversal Protection**: Detects unsafe file operations with user input  
- **Input Validation**: Ensures request binding includes proper validation
- **Secret Detection**: Identifies hardcoded API keys, passwords, tokens
- **Resource Leak Prevention**: Checks for unclosed files, connections, goroutines
- **Unsafe Deserialization**: Validates JSON/XML parsing with proper checks

#### 📋 Requirements Validation
- **EM Brief Analysis**: Validates implementation against Engineering Manager's success criteria
- **Task Completion Verification**: Ensures core requirements are fully implemented
- **Endpoint Validation**: Confirms required APIs and functionality are present
- **Build Verification**: Validates code compiles and meets technical requirements

#### 🔄 Duplication Detection
- **Function Analysis**: Detects similar functions across related files (80%+ similarity threshold)
- **Pattern Recognition**: Identifies duplicate business logic and validation patterns
- **Scope-Aware Scanning**: Analyzes same package, utility functions, and related functionality
- **Smart File Matching**: Handlers→handlers, services→services, models→models

#### 📐 Pattern Consistency
- **Documentation Integration**: Validates against established patterns in `/agents/patterns/`
- **Handler Patterns**: Ensures consistent function signatures and response formats
- **Error Handling**: Validates proper error wrapping and context preservation
- **Architecture Compliance**: Enforces project-specific conventions and standards

#### 🔄 Structured Rejection System
- **Four Rejection Categories**: Requirements, Security, Duplication, Patterns
- **Detailed Feedback**: Specific issues with examples and required actions
- **EM Routing**: All rejections route back through Engineering Manager for coordination
- **Auto-Fix Capability**: Applies formatting/linting fixes automatically (doesn't reject for these)

#### ⚡ Enhanced Review Process
1. **Requirements Analysis** → Validate EM brief success criteria
2. **Security Scanning** → Zero-tolerance vulnerability detection  
3. **Duplication Analysis** → Prevent unnecessary code duplication
4. **Pattern Validation** → Enforce established conventions
5. **Auto-Fix Application** → Apply formatting and linting improvements
6. **Final Assessment** → Approve or provide structured rejection feedback

## Development Guidelines

### File Organization Standards

- **ALL test files must go in `test-projects/` directory**
- **Never create test JSON files in project root**
- Test files include: `*test*.json`, `*-test.json`, `quick-test.json`, etc.
- Use existing `.gitignore` patterns to prevent test file clutter
- Keep project root clean and organized

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
docker compose up ollama

# Start both services
docker compose up

# Build MCP server only
docker compose build mcp-server

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
- **Enhanced Coordination System**: EM-Engineer structured briefing format with success criteria
- **Smart Routing Engine**: Dynamic agent transitions with 20+ decision rules + structured rejection handling
- **Comprehensive Tech Lead**: Security analysis, pattern validation, duplication detection, requirements validation
- **Documentation Structure**: Organized `/agents/` directory with pattern documentation
- **Dual-Mode Support**: Legacy single-agent + new multi-agent workflow
- **MCP Tools**: `implement_feature` (legacy) and `implement_feature_workflow` (multi-agent)
- **Ollama Integration**: Qwen3:14b-q4_K_M model with consistent references
- **Command Restriction System**: Per-agent security boundaries
- **Git Integration**: Context gathering, diff analysis, project history
- **Docker Deployment**: Containerized services with health checks
- **Configuration Management**: TOML-based agent and workflow configuration
- **Error Recovery**: Enhanced iteration limits, timeout handling, intelligent error categorization
- **Build Environment Optimization**: Enhanced Go commands, module management, and fallback strategies
- **Web Search Integration**: DuckDuckGo API for agent self-correction and external knowledge
- **Dynamic Pattern Discovery**: Automatic pattern scanning instead of hardcoded pattern lists
- **Comprehensive Debugging System**: Agent thought and action logging to debug/ directory
- **Sequential Thinking Integration**: Step-by-step reasoning tool for complex analysis
- **Autonomous Directory Creation**: Automatic missing directory detection and creation with proper context management
- **Simplified Engineering Manager**: Streamlined EM role as pure orchestrator, eliminating over-planning bottlenecks
- **Enhanced Error Recovery**: Context-aware file system error recovery with intelligent command validation
- **Robust Implementation Tooling**: End-to-end workflow completion with automatic project setup and compilation

### Model Status

- **Ollama Integration**: Qwen3:14b-q4_K_M model fully operational
- **Model Size**: ~9GB, downloads on first startup
- **Health Status**: All agents operational, 4 registered agents
- **Model Persistence**: Data persists in `ollama_data` volume
- **Performance**: Successful multi-agent workflow execution (tested)

### Testing Results

- **Multi-Agent Workflow**: Successfully tested with enhanced coordination improvements
- **EM-Engineer Coordination**: Simplified EM role eliminates over-planning, faster task assignment
- **Code Generation**: Functional Go applications with proper architecture and patterns
- **Agent Collaboration**: Enhanced EM briefing → Engineer parsing → Tech Lead analysis
- **Iteration Management**: Intelligent error categorization and routing operational
- **Tech Lead Enhancements**: Comprehensive security analysis, pattern validation, and structured feedback
- **Directory Creation Recovery**: 100% success rate for missing directory scenarios
- **End-to-End Implementation**: Verified working console apps and web APIs with compilation success
- **Error Recovery Systems**: Context-aware file system and module initialization recovery
- **Command Validation**: Robust single-command validation preventing compound command issues

### Current Implementation Capabilities

The system can successfully implement complete software projects from scratch:

#### **Verified Working Features:**
- **✅ Console Applications**: Simple Go programs with proper compilation and execution
- **✅ Web APIs**: HTTP servers with JSON endpoints and proper error handling  
- **✅ Project Setup**: Automatic directory creation, module initialization, and dependency management
- **✅ Error Recovery**: Intelligent handling of missing files, directories, and module declarations
- **✅ Build Process**: Successful compilation with proper binary generation

#### **Example Implementations:**
```bash
# Simple Console App Test
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature_workflow",
    "arguments": {
      "description": "Create a simple Go file that prints Hello World to the console",
      "project_type": "go",
      "working_directory": "/app/test-projects/simple-api"
    }
  }
}'

# Web API Test
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature_workflow",
    "arguments": {
      "description": "Create a Go HTTP server with a /health endpoint that returns JSON status",
      "project_type": "go",
      "working_directory": "/app/test-projects/web-api"
    }
  }
}'
```

#### **Typical Workflow Success:**
1. **EM**: Simple task assignment (no over-planning)
2. **Engineer**: Automatic directory creation → module setup → implementation
3. **Error Recovery**: Handles missing directories and modules automatically
4. **Build**: Successful compilation with working executables
5. **Result**: Complete, functional applications ready for use

## Future Enhancements

### Potential Enhancements

1. **QA Agent Enhancement**: Implement comprehensive test generation and analysis capabilities
2. **Pattern Learning**: Dynamic pattern documentation updates based on successful implementations
3. **Performance Optimization**: Parallel agent execution where possible
4. **Advanced Security**: Integration with external security scanning tools
5. **Workspace Management**: Multi-project support and isolation
6. **Metrics and Analytics**: Success rate tracking and workflow optimization
7. **Template System**: Reusable implementation templates for common patterns

### Integration Points

- Claude Code MCP client integration
- VS Code extension support
- CI/CD pipeline integration
- Multi-project workspace support

## Troubleshooting

### Common Issues

1. **Ollama "unhealthy"**: Normal during model download, wait for completion
2. **MCP build failures**: Check Go module dependencies with `go mod tidy`
3. **Command restrictions**: Verify allowlist in `config/agents.toml` (note: uses single commands, no compound commands)
4. **File access denied**: Ensure paths are within project directory
5. **Directory creation failures**: ✅ **RESOLVED** - Automatic directory creation now working
6. **Module initialization errors**: ✅ **RESOLVED** - Context-aware `go mod init` recovery implemented
7. **Workflow timeouts**: ✅ **RESOLVED** - Simplified EM role eliminates over-planning delays

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


- Use /home/bobparsons/Development/agents/test-projects for testing agents.

## Quick Reference - Latest Working Setup

### Successful Test Commands
```bash
# Test Console Application
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature_workflow",
    "arguments": {
      "description": "Create a simple Go file that prints Hello World",
      "project_type": "go",
      "working_directory": "/app/test-projects/console-test"
    }
  }
}'

# Test Web API
curl -X POST http://localhost:8080/call -H "Content-Type: application/json" -d '{
  "method": "tools/call",
  "params": {
    "name": "implement_feature_workflow",
    "arguments": {
      "description": "Create a Go HTTP server with /health endpoint returning JSON",
      "project_type": "go", 
      "working_directory": "/app/test-projects/api-test"
    }
  }
}'
```

### Key Improvements Made
- **✅ Autonomous Setup**: Agents automatically create directories and initialize projects
- **✅ Error Recovery**: Context-aware handling of missing files and modules  
- **✅ Command Validation**: Single-command execution prevents validation failures
- **✅ Simplified Workflow**: Streamlined EM role eliminates over-planning bottlenecks
- **✅ End-to-End Success**: Complete implementation from empty directory to working executable