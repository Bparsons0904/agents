package agent

import (
	"fmt"
	"mcp-server/internal/config"
	"mcp-server/internal/debug"
)

type DefaultAgentFactory struct{
	debugLogger *debug.DebugLogger
}

func NewAgentFactory(debugLogger *debug.DebugLogger) AgentFactory {
	return &DefaultAgentFactory{
		debugLogger: debugLogger,
	}
}

func (f *DefaultAgentFactory) CreateAgent(role AgentRole, llmClient LLMClient, toolSet ToolSet, restrictions CommandRestrictions, cfg config.WorkflowAgentConfig) (Agent, error) {
	switch role {
	case AgentRoleEM:
		return NewEngineeringManager(llmClient, toolSet, restrictions, f.debugLogger), nil
	case AgentRoleEngineer:
		return NewSeniorEngineer(llmClient, toolSet, restrictions, cfg), nil
	case AgentRoleQA:
		return NewSeniorQAEngineer(llmClient, toolSet, restrictions), nil
	case AgentRoleTechLead:
		return NewSeniorTechLead(llmClient, toolSet, restrictions), nil
	default:
		return nil, fmt.Errorf("unknown agent role: %s", role)
	}
}