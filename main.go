package main

import (
	"log"

	"github.com/sentineledge/agent/internal/agent"
)

func main() {
	log.Println("Iniciando SentinelEdge Agent...")

	cfg := agent.LoadConfig()
	a := agent.New(cfg)
	a.Run()
}
