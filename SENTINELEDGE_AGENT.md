# SentinelEdge Agent

## Descripción General

Agente Windows escrito en Go que se instala como Windows Service en los endpoints gestionados. Se encarga de registrarse con la API, enviar heartbeat periódico, hacer polling de comandos pendientes, ejecutar scripts PowerShell remotamente, recolectar inventory de hardware/software, y auto-actualizarse desde GitHub Releases.

---

## Infraestructura

| Componente | Detalle |
|---|---|
| Lenguaje | Go 1.21+ |
| Plataforma | Windows (amd64) |
| Instalación | `C:\Program Files\SentinelEdge\` |
| Ejecutable | `sentineledge-agent.exe` |
| Servicio | `SentinelEdgeAgent` (Windows Service) |
| Config | `C:\Program Files\SentinelEdge\agent.yaml` |
| Repo GitHub | `https://github.com/ardepa710/sentineledge-agent` |
| Releases | `https://github.com/ardepa710/sentineledge-agent/releases/latest` |

---

## Stack Tecnológico

- **Go** 1.21+
- `golang.org/x/sys/windows/svc` — Windows Service
- `github.com/spf13/viper` — configuración YAML
- PowerShell / WMI — recolección de inventory

---

## Estructura de Archivos

```
sentineledge-agent/
├── main.go                         ← Entry point, Windows Service wrapper
├── agent.yaml                      ← Config file (generado al instalar)
├── install.ps1                     ← Script de instalación
├── pkg/
│   └── models/
│       └── models.go               ← Structs: Command, Result, Inventory
├── internal/
│   ├── agent/
│   │   ├── agent.go                ← Lógica principal: Run(), tick(), inventory
│   │   └── config.go               ← Struct Config, LoadConfig()
│   ├── communicator/
│   │   └── communicator.go         ← HTTP client: Register, PollCommands, ReportResult, SendInventory
│   ├── executor/
│   │   └── executor.go             ← Ejecuta comandos PowerShell y tipo "update"
│   ├── system/
│   │   └── inventory.go            ← Recolecta hardware/software via WMI/PowerShell
│   ├── updater/
│   │   └── updater.go              ← Auto-update desde GitHub con verificación SHA256
│   └── vault/
│       └── vault.go                ← Cliente Vaultwarden para cargar/guardar token
```

---

## Flujo de Arranque

```
main.go
  └── Windows Service Manager
        └── agent.New(cfg)
              ├── Si no hay token → Register() en API
              │     └── Guarda AgentID + Token en Vaultwarden
              └── agent.Run()
                    ├── tick() inmediato (poll de comandos)
                    ├── collectAndSendInventory() en goroutine
                    ├── pollTicker cada 30s → tick()
                    └── inventoryTicker cada 24h → collectAndSendInventory()
```

---

## Configuración (agent.yaml)

```yaml
ServerURL: "https://saapi.ardepa.site"
TenantID: "tenant-sentineledge"
AgentID: "a98ef245-..."          # Se genera al registrar
PollInterval: 30                  # Segundos entre polls
VaultURL: "https://pwd.ardepa.site"
VaultClientID: "user.f50ad073-3d5a-4bdd-8ce7-a4fed752c1e8"
VaultClientSecret: ""    # supply via SE_VAULT_CLIENT_SECRET env var — never hardcode
# NOTA: El token NO se guarda en yaml, se carga de Vaultwarden
```

**Vaultwarden IDs del agente:**
- OrgID: `ebefd607-bd17-4a3f-aa01-4d1a28948ef5`
- ColAgentsID: `d0f075e6-65d2-4f13-935c-e4d7a3dce261`
- ColAPIID: `056e9be8-69ac-4e5e-95a8-bcbf803824a3`

---

## Communicator — HTTP Client

**Archivo:** `internal/communicator/communicator.go`

```go
type Communicator struct {
    serverURL string
    token     string
    agentID   string
}
```

**Métodos:**
- `Register(serverURL, tenantID, apiKey)` → POST `/agents/register`
- `PollCommands()` → GET `/commands/pending/{agent_id}` (Bearer token)
- `ReportResult(result)` → POST `/commands/result` (Bearer token)
- `SendInventory(inv)` → POST `/agents/inventory` (Bearer token)
- `Heartbeat()` → POST `/agents/{agent_id}/heartbeat` (Bearer token)

---

## Executor — Ejecución de Comandos

**Archivo:** `internal/executor/executor.go`

Soporta dos tipos de comandos:

### type: "powershell"
```go
exec.CommandContext(ctx,
    "powershell.exe",
    "-NonInteractive", "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-Command", cmd.Payload,
)
```
Timeout por defecto: 5 minutos. Retorna stdout, stderr, exit_code.

### type: "update"
Llama a `updater.Update()` — descarga, verifica hash e instala nueva versión.

---

## Inventory Collection

**Archivo:** `internal/system/inventory.go`

**CRÍTICO:** Usar `Get-WmiObject` — NO `Get-CimInstance` (tarda 2+ minutos).

### Estructura del Inventory
```go
type Inventory struct {
    AgentID   string
    Hostname  string
    OS        string
    CPU       CPUInfo
    RAM       RAMInfo
    BIOS      BIOSInfo
    Computer  ComputerInfo
    Serial    SerialInfo
    Disks     []DiskInfo
    NICs      []NICInfo
    Software  []SoftwareInfo
}
```

### Scripts PowerShell

**Script principal (hardware):**
```powershell
Get-WmiObject Win32_Processor | Select-Object -First 1 Name, NumberOfCores
Get-WmiObject Win32_PhysicalMemory | Measure-Object Capacity -Sum
Get-WmiObject Win32_BIOS | Select-Object SMBIOSBIOSVersion, Manufacturer
Get-WmiObject Win32_ComputerSystem | Select-Object Manufacturer, Model
Get-WmiObject Win32_BIOS | Select-Object SerialNumber
Get-WmiObject Win32_LogicalDisk | Where-Object DriveType -eq 3
Get-WmiObject Win32_NetworkAdapterConfiguration | Where-Object IPEnabled
```

**Script de software (corre en paralelo):**
```powershell
Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*
Get-ItemProperty HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*
```

**Ejecución paralela:**
```go
// Software corre en goroutine concurrente con hardware
go func() { /* software collection */ }()
out, err := runPowerShell(mainScript)
swRes := <-swCh  // esperar resultado de software
```

**Timeout:** 180 segundos con kill del proceso.

**clean_nulls:** Windows registry puede contener `\u0000` en nombres — se limpian antes de enviar.

---

## Auto-Update

**Archivo:** `internal/updater/updater.go`

### Flujo de Update
```
1. GET https://saapi.ardepa.site/version
   → Obtiene: version, hash (SHA256 minúsculas), download_url

2. Descarga exe desde download_url
   → https://github.com/.../releases/latest/download/sentineledge-agent.exe

3. Verifica SHA256 del exe descargado
   → Si no coincide: elimina archivo y retorna error

4. Genera script PowerShell temporal en %TEMP%\se_update.ps1:
   Stop-Service "SentinelEdgeAgent" -Force
   Move-Item -Force <nuevo_exe> <exe_actual>
   Start-Service "SentinelEdgeAgent"

5. Ejecuta script en background con cmd.Start() (no Wait)
6. Sleep 500ms y retorna (el servicio se reinicia solo)
```

### Constantes
```go
const (
    VersionURL  = "https://saapi.ardepa.site/version"
    ServiceName = "SentinelEdgeAgent"
    InstallDir  = `C:\Program Files\SentinelEdge`
)
```

### Verificación de Hash
```go
// IMPORTANTE: El hash debe estar en MINÚSCULAS en el endpoint /version
// SHA256 se compara con strings.ToLower() en ambos lados
actual := hex.EncodeToString(h.Sum(nil))  // ya viene en minúsculas
if actual != strings.ToLower(expectedHash) {
    return fmt.Errorf("hash mismatch")
}
```

---

## Instalación / Instalador

**Archivo:** `install.ps1`

```powershell
# Configuración del instalador
$ServerURL = "https://saapi.ardepa.site"
$TenantID = "tenant-sentineledge"
$VaultURL = "https://pwd.ardepa.site"
$VaultClientID = "user.f50ad073-3d5a-4bdd-8ce7-a4fed752c1e8"
$VaultClientSecret = "<REDACTED>"   # pass as -VaultClientSecret param, never hardcode

# Instala en C:\Program Files\SentinelEdge\
# Registra como Windows Service
# Crea agent.yaml con la configuración
```

---

## Compilación

```powershell
cd C:\proyectos\sentineledge-agent
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -ldflags "-X github.com/sentineledge/agent/internal/version.Version=v2026.04.01-1" -o sentineledge-agent.exe main.go
```

> La versión se inyecta en build time vía `-ldflags`. Sin ese flag, el agente reportará `"dev"`.
> Formato recomendado: `v{año}.{mes}.{dia}-{build}` (ej. `v2026.04.01-1`).

### Generar Hash para /version
```powershell
Get-FileHash ".\sentineledge-agent.exe" -Algorithm SHA256
# Copiar el hash en MINÚSCULAS al .env del VPS como AGENT_UPDATE_HASH
```

---

## Proceso de Release

1. Compilar exe
2. Generar hash SHA256 (en minúsculas)
3. Subir exe a GitHub Release con tag (ej. `v2026.03.10-1`)
4. Actualizar en VPS:
   ```bash
   # Editar .env del VPS
   AGENT_UPDATE_HASH=<nuevo_hash_en_minusculas>
   AGENT_UPDATE_VERSION=<nuevo_tag>
   docker compose restart api
   ```
5. Desde dashboard → **Update Agent** en cada endpoint

---

## Gestión del Servicio Windows

```powershell
# Ver estado
Get-Service SentinelEdgeAgent

# Detener (requiere Admin)
Stop-Service SentinelEdgeAgent -Force

# Iniciar
Start-Service SentinelEdgeAgent

# Reemplazar exe manualmente
Stop-Service SentinelEdgeAgent -Force
Copy-Item -Force ".\sentineledge-agent.exe" "C:\Program Files\SentinelEdge\sentineledge-agent.exe"
Start-Service SentinelEdgeAgent
```

---

## Seguridad

- Token almacenado en Vaultwarden, nunca en disco en texto plano
- Bearer token en todos los requests a la API
- Verificación SHA256 del exe antes de instalar updates
- Timeout de 180s en inventory y 5min en comandos

---

## Seguridad Pendiente

- [ ] **Whitelist de comandos** — solo permitir scripts pre-aprobados por hash
- [ ] **Rotación automática de tokens** — renovar token cada 30 días
- [ ] **Certificate pinning** — verificar certificado SSL del servidor

---

## Modelos Go

```go
// pkg/models/models.go

type Command struct {
    ID      string
    Type    string   // "powershell" | "update"
    Payload string
    Timeout int
}

type Result struct {
    JobID      string
    ExitCode   int
    Stdout     string
    Stderr     string
    Error      string
    FinishedAt time.Time
}
```

---

## Comandos Soportados

| Type | Payload | Descripción |
|---|---|---|
| `powershell` | Script PS1 | Ejecuta PowerShell arbitrario |
| `update` | "" (vacío) | Auto-update desde GitHub releases/latest |

## Integración N8N

Los workflows de N8N que envían comandos al agente deben:
1. Usar `Body Content Type: JSON`
2. Incluir header `x-api-key: <DASHBOARD_API_KEY>`
3. Usar `JSON.stringify()` para el payload PowerShell multilínea:

```json
{
  "agent_id": "{{ $json.sentinel_agent_id }}",
  "type": "powershell",
  "timeout": {{ $json.AgentTimeout }},
  "payload": {{ JSON.stringify($json.AgentCommand) }}
}
```
