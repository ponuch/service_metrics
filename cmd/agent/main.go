package main

import (
	"github.com/ponuch/service_metrics/internal/agent/config"
	"github.com/ponuch/service_metrics/internal/agent/services/agent"
)


func main() {
	agentConfig := config.New()
	agent := agent.NewAgent(*agentConfig)
	agent.Run()
}