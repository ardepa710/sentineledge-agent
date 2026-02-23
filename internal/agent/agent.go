package agent

import (
	"log"
	"time"

	"github.com/sentineledge/agent/internal/communicator"
	"github.com/sentineledge/agent/internal/executor"
	"github.com/sentineledge/agent/pkg/models"
	"github.com/spf13/viper"
)

type Agent struct {
	config *Config
	comm   *communicator.Communicator
}

func New(cfg *Config) *Agent {
	// Si no tenemos token, registrarse primero
	if cfg.AgentToken == "" || cfg.AgentID == "" {
		log.Println("Sin token/ID — registrando agente...")

		resp, err := communicator.Register(cfg.ServerURL, cfg.TenantID, cfg.APIKey)
		if err != nil {
			log.Fatalf("No se pudo registrar el agente: %v", err)
		}

		cfg.AgentID = resp.ID
		cfg.AgentToken = resp.Token

		// Guardar token y ID para próximas ejecuciones
		viper.Set("AgentID", resp.ID)
		viper.Set("AgentToken", resp.Token)
		if err := viper.WriteConfig(); err != nil {
			log.Printf("Advertencia: no se pudo guardar config: %v", err)
		}
	}

	comm := communicator.New(cfg.ServerURL, cfg.AgentToken, cfg.AgentID)
	return &Agent{config: cfg, comm: comm}
}

func (a *Agent) Run() {
	log.Printf("Agente iniciado — ID: %s", a.config.AgentID)
	log.Printf("Servidor: %s", a.config.ServerURL)
	log.Printf("Poll cada %d segundos", a.config.PollInterval)

	ticker := time.NewTicker(time.Duration(a.config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Poll inmediato al arrancar
	a.tick()

	for range ticker.C {
		a.tick()
	}
}

func (a *Agent) tick() {
	commands, err := a.comm.PollCommands()
	if err != nil {
		log.Printf("Error in poll: %v", err)
		return
	}

	if len(commands) == 0 {
		return
	}

	log.Printf("%d command(s) recieved", len(commands))

	for _, cmd := range commands {
		cmdCopy := cmd
		go func() {
			a.executeCommand(cmdCopy)
		}()
	}
}

func (a *Agent) executeCommand(cmd models.Command) {
	log.Printf("Running job %s — type: %s", cmd.ID, cmd.Type)

	// Usar ID del comando como JobID para el resultado
	cmdForExecutor := models.Command{
		ID:      cmd.ID,
		Type:    cmd.Type,
		Payload: cmd.Payload,
		Timeout: cmd.Timeout,
	}

	result := executor.Execute(cmdForExecutor)
	result.JobID = cmd.ID

	if err := a.comm.ReportResult(result); err != nil {
		log.Printf("Error reporting job %s: %v", cmd.ID, err)
	}
}
