package version

// Version is set at build time via ldflags:
// go build -ldflags "-X github.com/sentineledge/agent/internal/version.Version=v2026.04.01-1" -o sentineledge-agent.exe
var Version = "dev"
