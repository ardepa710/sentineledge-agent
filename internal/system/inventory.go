package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sentineledge/agent/pkg/models"
)

func CollectInventory(agentID, hostname string) (*models.Inventory, error) {
	inv := &models.Inventory{
		AgentID:  agentID,
		Hostname: hostname,
		OS:       runtime.GOOS,
	}

	if runtime.GOOS == "windows" {
		if err := collectWindows(inv); err != nil {
			return nil, fmt.Errorf("windows inventory error: %w", err)
		}
	} else {
		if err := collectLinux(inv); err != nil {
			return nil, fmt.Errorf("linux inventory error: %w", err)
		}
	}

	return inv, nil
}

// ── Windows ──────────────────────────────────────────────────────────────

func collectWindows(inv *models.Inventory) error {
	script := `
$ErrorActionPreference = 'SilentlyContinue'

# CPU
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
# RAM
$ram = Get-CimInstance Win32_ComputerSystem
# BIOS
$bios = Get-CimInstance Win32_BIOS
# Computer
$comp = Get-CimInstance Win32_ComputerSystem
# Serial
$serial = Get-CimInstance Win32_BIOS
# Disks
$disks = Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3"
# NICs
$nics = Get-CimInstance Win32_NetworkAdapterConfiguration | Where-Object { $_.IPAddress -ne $null }
# Software (registry - more complete than Win32_Product)
$software = @()
$paths = @(
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
foreach ($path in $paths) {
    if (Test-Path $path) {
        $software += Get-ItemProperty $path |
            Where-Object { $_.DisplayName -ne $null -and $_.DisplayName -ne '' } |
            Select-Object DisplayName, DisplayVersion, Publisher, InstallDate
    }
}
$software = $software | Sort-Object DisplayName -Unique

$result = @{
    cpu = @{
        name           = $cpu.Name.Trim()
        number_of_cores = [int]$cpu.NumberOfCores
    }
    ram = @{
        total_physical_memory_gb = [math]::Round($ram.TotalPhysicalMemory / 1GB, 2)
    }
    bios = @{
        smbios_bios_version = $bios.SMBIOSBIOSVersion
        manufacturer        = $bios.Manufacturer
    }
    computer = @{
        manufacturer = $comp.Manufacturer.Trim()
        model        = $comp.Model.Trim()
    }
    serial = @{
        serial_number = $serial.SerialNumber
    }
    disks = @($disks | ForEach-Object {
        @{
            device_id = $_.DeviceID
            size_gb   = [math]::Round($_.Size / 1GB, 2)
            free_gb   = [math]::Round($_.FreeSpace / 1GB, 2)
        }
    })
    nics = @($nics | ForEach-Object {
        @{
            description  = $_.Description
            mac_address  = $_.MACAddress
            ip_addresses = @($_.IPAddress | Where-Object { $_ -ne $null })
        }
    })
    software = @($software | ForEach-Object {
        @{
            name         = $_.DisplayName
            version      = if ($_.DisplayVersion) { $_.DisplayVersion } else { "" }
            publisher    = if ($_.Publisher) { $_.Publisher } else { "" }
            install_date = if ($_.InstallDate) { $_.InstallDate } else { "" }
        }
    })
}

$result | ConvertTo-Json -Depth 5 -Compress
`

	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("powershell error: %w", err)
	}

	var raw struct {
		CPU      models.InventoryCPU        `json:"cpu"`
		RAM      models.InventoryRAM        `json:"ram"`
		BIOS     models.InventoryBIOS       `json:"bios"`
		Computer models.InventoryComputer   `json:"computer"`
		Serial   models.InventorySerial     `json:"serial"`
		Disks    []models.InventoryDisk     `json:"disks"`
		NICs     []models.InventoryNIC      `json:"nics"`
		Software []models.InventorySoftware `json:"software"`
	}

	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return fmt.Errorf("json parse error: %w\nOutput: %s", err, out)
	}

	inv.CPU = raw.CPU
	inv.RAM = raw.RAM
	inv.BIOS = raw.BIOS
	inv.Computer = raw.Computer
	inv.Serial = raw.Serial
	inv.Disks = raw.Disks
	inv.NICs = raw.NICs
	inv.Software = raw.Software

	return nil
}

// ── Linux ────────────────────────────────────────────────────────────────

func collectLinux(inv *models.Inventory) error {
	// CPU
	if out, err := runCmd("bash", "-c", "lscpu | grep 'Model name' | cut -d: -f2 | xargs"); err == nil {
		inv.CPU.Name = strings.TrimSpace(out)
	}
	if out, err := runCmd("bash", "-c", "lscpu | grep '^CPU(s):' | awk '{print $2}'"); err == nil {
		fmt.Sscanf(strings.TrimSpace(out), "%d", &inv.CPU.NumberOfCores)
	}

	// RAM
	if out, err := runCmd("bash", "-c", "grep MemTotal /proc/meminfo | awk '{print $2}'"); err == nil {
		var kb int64
		fmt.Sscanf(strings.TrimSpace(out), "%d", &kb)
		inv.RAM.TotalPhysicalMemoryGB = float64(kb) / 1024 / 1024
	}

	// Serial / Computer
	if out, err := runCmd("bash", "-c", "cat /sys/class/dmi/id/product_serial 2>/dev/null || echo unknown"); err == nil {
		inv.Serial.SerialNumber = strings.TrimSpace(out)
	}
	if out, err := runCmd("bash", "-c", "cat /sys/class/dmi/id/sys_vendor 2>/dev/null || echo unknown"); err == nil {
		inv.Computer.Manufacturer = strings.TrimSpace(out)
	}
	if out, err := runCmd("bash", "-c", "cat /sys/class/dmi/id/product_name 2>/dev/null || echo unknown"); err == nil {
		inv.Computer.Model = strings.TrimSpace(out)
	}

	// Hostname from OS
	if hostname, err := os.Hostname(); err == nil {
		inv.Hostname = hostname
	}

	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────

func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim BOM and whitespace
	result := strings.TrimSpace(string(out))
	result = strings.TrimPrefix(result, "\xef\xbb\xbf")
	return result, nil
}

func runCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
