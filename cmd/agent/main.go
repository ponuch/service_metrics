package main

import (
	"github.com/ponuch/service_metrics/internal/agent/config"
	"github.com/ponuch/service_metrics/internal/agent/services/agent"
)


func main() {
	agentConfig := config.New()
	// agent := NewAgent(agentConfig)
	agent := mainagent.NewAgent(*agentConfig)
	agent.Run()
	// agent.Run()
}