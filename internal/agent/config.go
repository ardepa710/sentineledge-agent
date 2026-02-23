package agent

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	ServerURL    string
	AgentToken   string
	AgentID      string
	PollInterval int
	TenantID     string
	APIKey       string
}

func LoadConfig() *Config {
	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("SE")
	viper.AutomaticEnv()

	viper.SetDefault("PollInterval", 30)
	viper.SetDefault("ServerURL", "http://localhost:8000")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("No se encontró archivo de config, usando defaults")
	}

	return &Config{
		ServerURL:    viper.GetString("ServerURL"),
		AgentToken:   viper.GetString("AgentToken"),
		AgentID:      viper.GetString("AgentID"),
		PollInterval: viper.GetInt("PollInterval"),
		TenantID:     viper.GetString("TenantID"),
		APIKey:       viper.GetString("APIKey"),
	}
}
