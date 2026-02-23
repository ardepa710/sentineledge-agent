package main

import (
	"log"
	"os"

	"github.com/kardianos/service"
	"github.com/sentineledge/agent/internal/agent"
)

type program struct {
	agent *agent.Agent
}

func (p *program) Start(s service.Service) error {
	go p.agent.Run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	log.Println("Stoppping SentinelEdge Agent...")
	return nil
}

func main() {
	svcConfig := &service.Config{
		Name:        "SentinelEdgeAgent",
		DisplayName: "SentinelEdge Agent",
		Description: "SentinelEdge monitoring and automation Agent",
	}

	cfg := agent.LoadConfig()
	a := agent.New(cfg)
	prg := &program{agent: a}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Manejo de comandos: install, uninstall, start, stop
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		switch cmd {
		case "install":
			err = s.Install()
			if err != nil {
				log.Fatalf("Error Installing Seervice: %v", err)
			}
			log.Println("Service Sucessfully Installed")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatalf("Error stopping service: %v", err)
			}
			log.Println("Service Successfully Uninstalled")
			return
		case "start":
			err = s.Start()
			if err != nil {
				log.Fatalf("Error starting service: %v", err)
			}
			log.Println("Servicio Successfully Started")
			return
		case "stop":
			err = s.Stop()
			if err != nil {
				log.Fatalf("Error stopping service: %v", err)
			}
			log.Println("Servicio Sucessfully Stopped")
			return
		}
	}

	// Correr como servicio o interactivo
	if err = s.Run(); err != nil {
		log.Fatal(err)
	}
}
