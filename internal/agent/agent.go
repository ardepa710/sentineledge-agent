package agent

import (
	"log"
	"time"

	"github.com/sentineledge/agent/internal/communicator"
	"github.com/sentineledge/agent/internal/executor"
	"github.com/sentineledge/agent/internal/system"
	"github.com/sentineledge/agent/internal/vault"
	"github.com/sentineledge/agent/pkg/models"
	"github.com/spf13/viper"
)

type agentComm interface {
	PollCommands() ([]models.Command, error)
	ReportResult(models.Result) error
	SendInventory(*models.Inventory) error
	Heartbeat() error
}

type Agent struct {
	config *Config
	comm   agentComm
}

func (a *Agent) sendHeartbeat() {
	if err := a.comm.Heartbeat(); err != nil {
		log.Printf("Heartbeat error: %v", err)
	}
}

const maxConcurrentCommands = 5

func New(cfg *Config) *Agent {
	if cfg.AgentToken == "" || cfg.AgentID == "" {
		log.Println("No token/ID — registering agent...")

		resp, err := communicator.Register(cfg.ServerURL, cfg.TenantID, cfg.APIKey)
		if err != nil {
			log.Fatalf("Agent cannot be registered: %v", err)
		}

		cfg.AgentID = resp.ID
		cfg.AgentToken = resp.Token

		// Si hay Vault configurado, guardar token en Vaultwarden
		if cfg.VaultURL != "" && cfg.VaultClientID != "" && cfg.VaultClientSecret != "" {
			vc := vault.NewClient(cfg.VaultURL, cfg.VaultClientID, cfg.VaultClientSecret)
			err := vc.StoreSecret(
				"AGENT_TOKEN_"+resp.ID,
				resp.Token,
				cfg.VaultOrgID,
				cfg.VaultColAgents,
			)
			if err != nil {
				log.Printf("Warning: could not store token in vault: %v — saving to agent.yaml", err)
			} else {
				log.Println("Token stored in Vaultwarden successfully")
			}
			// Solo guardar AgentID en agent.yaml, nunca el token
			viper.Set("AgentID", resp.ID)
			viper.Set("PollInterval", cfg.PollInterval)
			viper.Set("ServerURL", cfg.ServerURL)
			viper.Set("TenantID", cfg.TenantID)
			viper.Set("VaultURL", cfg.VaultURL)
			viper.Set("VaultClientID", cfg.VaultClientID)
			viper.Set("VaultClientSecret", cfg.VaultClientSecret)
			viper.WriteConfig()
		} else {
			// Sin Vault — guardar en agent.yaml como antes
			viper.Set("AgentID", resp.ID)
			viper.Set("PollInterval", cfg.PollInterval)
			viper.Set("ServerURL", cfg.ServerURL)
			viper.Set("TenantID", cfg.TenantID)
			viper.Set("VaultURL", cfg.VaultURL)
			viper.Set("VaultClientID", cfg.VaultClientID)
			viper.Set("VaultClientSecret", cfg.VaultClientSecret)
			viper.WriteConfig()
		}
	}

	comm := communicator.New(cfg.ServerURL, cfg.AgentToken, cfg.AgentID)
	return &Agent{config: cfg, comm: comm}
}

func (a *Agent) Run() {
	log.Printf("Agent initialized — ID: %s", a.config.AgentID)
	log.Printf("Server: %s", a.config.ServerURL)
	log.Printf("Poll every %d seconds", a.config.PollInterval)

	// Poll inmediato al arrancar
	a.tick()

	// Inventory al arrancar
	go a.collectAndSendInventory()

	// Heartbeat inmediato al arrancar
	go a.sendHeartbeat()

	// Ticker para poll de comandos
	pollTicker := time.NewTicker(time.Duration(a.config.PollInterval) * time.Second)
	defer pollTicker.Stop()

	// Ticker para inventory cada 24 horas
	inventoryTicker := time.NewTicker(24 * time.Hour)
	defer inventoryTicker.Stop()

	// Ticker para heartbeat — configurable, default 60s
	// La API marca offline si last_seen > 120s, así que 60s permite tolerar 1 heartbeat perdido
	hbInterval := a.config.HeartbeatInterval
	if hbInterval <= 0 {
		hbInterval = 60 // 1 minute default
	}
	heartbeatTicker := time.NewTicker(time.Duration(hbInterval) * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			a.tick()
		case <-inventoryTicker.C:
			go a.collectAndSendInventory()
		case <-heartbeatTicker.C:
			go a.sendHeartbeat()
		}
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

	log.Printf("%d command(s) received", len(commands))

	sem := make(chan struct{}, maxConcurrentCommands)
	for _, cmd := range commands {
		cmdCopy := cmd
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
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

func (a *Agent) collectAndSendInventory() {
	log.Println("Collecting inventory...")
	inv, err := system.CollectInventory(a.config.AgentID, a.config.Hostname)
	if err != nil {
		log.Printf("Inventory collection error: %v", err)
		return
	}
	if err := a.comm.SendInventory(inv); err != nil {
		log.Printf("Inventory send error: %v", err)
		return
	}
}
