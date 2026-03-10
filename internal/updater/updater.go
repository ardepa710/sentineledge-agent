package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	VersionURL  = "https://saapi.ardepa.site/version"
	ServiceName = "SentinelEdgeAgent"
	InstallDir  = `C:\Program Files\SentinelEdge`
)

type VersionInfo struct {
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	DownloadURL string `json:"download_url"`
}

func Update() error {
	log.Println("Fetching version info...")

	// 1. Obtener version info desde la API
	info, err := getVersionInfo()
	if err != nil {
		return fmt.Errorf("failed to get version info: %w", err)
	}
	log.Printf("Target version: %s hash: %s", info.Version, info.Hash[:8])

	exePath := filepath.Join(InstallDir, "sentineledge-agent.exe")
	tmpPath := filepath.Join(InstallDir, "sentineledge-agent-update.exe")

	// 2. Descargar nuevo exe
	log.Printf("Downloading from %s", info.DownloadURL)
	if err := download(info.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	log.Println("Download complete")

	// 3. Verificar hash
	log.Println("Verifying integrity...")
	if err := verifyHash(tmpPath, info.Hash); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("integrity check failed: %w", err)
	}
	log.Println("Hash verified OK")

	// 4. Script PowerShell para reemplazar y reiniciar
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

	// 5. Ejecutar script en background
	log.Println("Launching update script...")
	cmd := exec.Command("powershell.exe",
		"-NonInteractive", "-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update script: %w", err)
	}

	time.Sleep(500 * time.Millisecond)
	log.Println("Update initiated — service will restart with new version")
	return nil
}

func getVersionInfo() (*VersionInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(VersionURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	if info.Hash == "" || info.DownloadURL == "" {
		return nil, fmt.Errorf("incomplete version info from API")
	}
	return &info, nil
}

func verifyHash(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s got %s", expectedHash[:8], actual[:8])
	}
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
