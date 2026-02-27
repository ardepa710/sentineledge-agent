package agent

import (
	"log"
	"os"
	"path/filepath"

	"github.com/sentineledge/agent/internal/vault"
	"github.com/spf13/viper"
)

type Config struct {
	ServerURL         string
	AgentToken        string
	AgentID           string
	Hostname          string
	PollInterval      int
	TenantID          string
	APIKey            string
	VaultURL          string
	VaultClientID     string
	VaultClientSecret string
}

func LoadConfig() *Config {
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		viper.AddConfigPath(exeDir)
	}

	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("SE")
	viper.AutomaticEnv()

	viper.SetDefault("PollInterval", 30)
	viper.SetDefault("ServerURL", "https://saapi.ardepa.site")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("No config file found, using defaults")
	}

	cfg := &Config{
		ServerURL:         viper.GetString("ServerURL"),
		AgentID:           viper.GetString("AgentID"),
		PollInterval:      viper.GetInt("PollInterval"),
		TenantID:          viper.GetString("TenantID"),
		APIKey:            viper.GetString("APIKey"),
		VaultURL:          viper.GetString("VaultURL"),
		VaultClientID:     viper.GetString("VaultClientID"),
		VaultClientSecret: viper.GetString("VaultClientSecret"),
	}

	// Si hay Vault configurado, obtener el token desde Vaultwarden
	if cfg.VaultURL != "" && cfg.VaultClientID != "" && cfg.VaultClientSecret != "" {
		log.Println("Vault configured — loading token from Vaultwarden...")
		vc := vault.NewClient(cfg.VaultURL, cfg.VaultClientID, cfg.VaultClientSecret)

		// El nombre del secret en Vault es "AGENT_TOKEN_<AgentID>" o "AGENT_APIKEY" para registro
		if cfg.AgentID != "" {
			token, err := vc.GetSecret("AGENT_TOKEN_" + cfg.AgentID)
			if err != nil {
				log.Printf("Warning: could not load token from vault: %v", err)
				log.Println("Falling back to agent.yaml token")
				cfg.AgentToken = viper.GetString("AgentToken")
			} else {
				log.Println("Token loaded from Vaultwarden successfully")
				cfg.AgentToken = token
			}
		} else {
			// Sin AgentID aún — necesita registrarse, cargar APIKey desde vault
			apiKey, err := vc.GetSecret("AGENT_APIKEY")
			if err != nil {
				log.Printf("Warning: could not load APIKey from vault: %v", err)
				cfg.APIKey = viper.GetString("APIKey")
			} else {
				log.Println("APIKey loaded from Vaultwarden")
				cfg.APIKey = apiKey
			}
		}
	} else {
		// Sin Vault — usar valores del agent.yaml directamente
		cfg.AgentToken = viper.GetString("AgentToken")
		log.Println("No Vault configured — using agent.yaml credentials")
	}

	if hostname, err := os.Hostname(); err == nil {
		cfg.Hostname = hostname
	}
	return cfg
}
