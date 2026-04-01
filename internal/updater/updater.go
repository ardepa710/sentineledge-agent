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
	"strconv"
	"strings"
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

// isNewer reports whether candidate is strictly greater than current.
// Version format: vYYYY.MM.DD-N (leading 'v' is optional).
func isNewer(candidate, current string) bool {
	parse := func(v string) (year, month, day, build int, ok bool) {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, "-", 2)
		if len(parts) != 2 {
			return
		}
		date := strings.Split(parts[0], ".")
		if len(date) != 3 {
			return
		}
		var err error
		if year, err = strconv.Atoi(date[0]); err != nil {
			return
		}
		if month, err = strconv.Atoi(date[1]); err != nil {
			return
		}
		if day, err = strconv.Atoi(date[2]); err != nil {
			return
		}
		if build, err = strconv.Atoi(parts[1]); err != nil {
			return
		}
		ok = true
		return
	}
	cy, cm, cd, cb, cok := parse(candidate)
	ry, rm, rd, rb, rok := parse(current)
	if !cok || !rok {
		return false
	}
	if cy != ry {
		return cy > ry
	}
	if cm != rm {
		return cm > rm
	}
	if cd != rd {
		return cd > rd
	}
	return cb > rb
}

func Update(currentVersion string) error {
	log.Println("Fetching version info...")

	// 1. Obtener version info desde la API
	info, err := getVersionInfo()
	if err != nil {
		return fmt.Errorf("failed to get version info: %w", err)
	}
	log.Printf("Target version: %s hash: %s", info.Version, info.Hash[:8])

	exePath := filepath.Join(InstallDir, "sentineledge-agent.exe")
	tmpPath := filepath.Join(InstallDir, "sentineledge-agent-update.exe")

	// 2. Validar origen de la URL antes de descargar (FINDING-02)
	if err := validateDownloadURL(info.DownloadURL); err != nil {
		return fmt.Errorf("download_url rejected: %w", err)
	}

	// 3. Descargar nuevo exe
	log.Printf("Downloading from %s", info.DownloadURL)
	if err := download(info.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	log.Println("Download complete")

	// 4. Verificar hash
	log.Println("Verifying integrity...")
	if err := verifyHash(tmpPath, info.Hash); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("integrity check failed: %w", err)
	}
	log.Println("Hash verified OK")

	// 5. Script PowerShell para reemplazar y reiniciar
	script := fmt.Sprintf(`
Start-Sleep -Seconds 2
Stop-Service "%s" -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Move-Item -Force "%s" "%s"
Start-Sleep -Seconds 1
Start-Service "%s"
`, ServiceName, tmpPath, exePath, ServiceName)

	// FINDING-06: Usar nombre aleatorio para evitar TOCTOU con nombre fijo
	f, err := os.CreateTemp("", "se_update_*.ps1")
	if err != nil {
		return fmt.Errorf("failed to create temp script: %w", err)
	}
	scriptPath := f.Name()
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(scriptPath)
		return fmt.Errorf("failed to write update script: %w", err)
	}
	f.Close()

	// 6. Ejecutar script en background
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

// validateDownloadURL rechaza cualquier URL que no provenga de dominios confiables (FINDING-02).
func validateDownloadURL(rawURL string) error {
	allowed := []string{
		"https://github.com/ardepa710/sentineledge-agent/",
		"https://objects.githubusercontent.com/",
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(rawURL, prefix) {
			return nil
		}
	}
	return fmt.Errorf("download_url %q is not in the allowed domain list", rawURL)
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
