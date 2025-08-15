package agent

import (
	"fmt"
	"mcp-server/internal/config"
)

type DefaultAgentFactory struct{}

func NewAgentFactory() AgentFactory {
	return &DefaultAgentFactory{}
}

func (f *DefaultAgentFactory) CreateAgent(role AgentRole, llmClient LLMClient, toolSet ToolSet, restrictions CommandRestrictions, cfg config.WorkflowAgentConfig) (Agent, error) {
	switch role {
	case AgentRoleEM:
		return NewEngineeringManager(llmClient, toolSet, restrictions), nil
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