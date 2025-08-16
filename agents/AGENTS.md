# Agent Knowledge Base

This file is managed by the Engineering Manager agent to maintain context and learnings between tasks.

## Current Workflow Patterns

### EM-Engineer Coordination
- **Structured Briefing Format**: EM provides structured briefs with TASK, CONTEXT, FILES_TO_EXAMINE, IMPLEMENTATION_APPROACH, POTENTIAL_ISSUES, and SUCCESS_CRITERIA
- **Engineer Brief Parsing**: Engineer agent parses EM briefings for structured guidance
- **Error Categorization**: Errors are categorized for intelligent routing (approach_issue, pattern_mismatch, structure_issue, setup_issue)
- **Escalation Logic**: Certain error types automatically route back to EM for replanning

### Project Organization Standards
- Use /app/test-projects/ for feature implementations to avoid conflicts
- Initialize proper project structure (go.mod, directory organization)
- Clean up conflicting files before starting new implementations
- Maintain clear separation between different features

### Implementation Patterns
- **Go Projects**: Follow standard Go project layout with internal/ directory structure
- **Authentication**: Use JWT-based authentication with proper middleware
- **API Design**: RESTful endpoints with consistent error handling
- **Testing**: Comprehensive test coverage with table-driven tests where appropriate

## Successful Implementations

### 2024 Implementations
- **JWT Authentication API**: Successfully implemented user authentication service with JWT tokens, middleware, and proper error handling
- **User CRUD Service**: Complete user management service with database operations and validation
- **Health Check Endpoints**: Standard health check implementation for service monitoring
- **Go Fiber Web Server**: Fast HTTP server implementation with proper routing and middleware

## Known Challenges and Solutions

### Build Issues
- **Go Module Conflicts**: Always run `go mod tidy` after implementation
- **Import Conflicts**: Use structured imports and avoid circular dependencies
- **Path Issues**: Use relative imports within project structure

### Coordination Issues
- **Context Loss**: Solved with structured EM briefing format
- **Error Recovery**: Implemented intelligent error categorization and routing
- **Pattern Consistency**: Ongoing - enhanced Tech Lead review needed

## Tech Lead Enhancement Areas
- Requirements validation against EM brief
- Security analysis for common vulnerabilities
- Duplication detection across related files
- Pattern consistency analysis
- Structured rejection feedback with routing through EM