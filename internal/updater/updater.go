package updater

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	DownloadURL = "https://github.com/ardepa710/sentineledge-agent/releases/latest/download/sentineledge-agent.exe"
	ServiceName = "SentinelEdgeAgent"
	InstallDir  = `C:\Program Files\SentinelEdge`
)

func Update() error {
	log.Println("Starting auto-update...")

	exePath := filepath.Join(InstallDir, "sentineledge-agent.exe")
	tmpPath := filepath.Join(InstallDir, "sentineledge-agent-update.exe")

	// 1. Descargar nuevo exe
	log.Printf("Downloading from %s", DownloadURL)
	if err := download(DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	log.Println("Download complete")

	// 2. Script PowerShell para reemplazar y reiniciar
	// Lo hacemos con un script separado porque no podemos reemplazar
	// el exe mientras está corriendo
	script := fmt.Sprintf(`
Start-Sleep -Seconds 2
Stop-Service "%s" -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Move-Item -Force "%s" "%s"
Start-Sleep -Seconds 1
Start-Service "%s"
`, ServiceName, tmpPath, exePath, ServiceName)

	scriptPath := filepath.Join(os.TempDir(), "se_update.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write update script: %w", err)
	}

	// 3. Ejecutar script en background y salir
	log.Println("Launching update script and restarting service...")
	cmd := exec.Command("powershell.exe",
		"-NonInteractive", "-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update script: %w", err)
	}

	// Dar tiempo al script para arrancar antes de que el servicio se detenga
	time.Sleep(500 * time.Millisecond)

	log.Println("Update initiated — service will restart with new version")
	return nil
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
