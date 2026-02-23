package agent

import (
	"log"
	"os"
	"path/filepath"

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
	// Buscar agent.yaml en el mismo directorio que el ejecutable
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
