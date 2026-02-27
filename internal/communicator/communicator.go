package communicator

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/go-resty/resty/v2"
	"github.com/sentineledge/agent/pkg/models"
)

type Communicator struct {
	client  *resty.Client
	agentID string
}

type RegisterRequest struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	TenantID string `json:"tenant_id"`
	APIKey   string `json:"api_key"`
}

type RegisterResponse struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Message string `json:"message"`
}

func New(serverURL, token, agentID string) *Communicator {
	client := resty.New().
		SetBaseURL(serverURL).
		SetHeader("Content-Type", "application/json").
		SetRetryCount(3)

	if token != "" {
		client.SetHeader("Authorization", "Bearer "+token)
	}

	return &Communicator{
		client:  client,
		agentID: agentID,
	}
}

// Register registra el agente en el servidor y retorna id y token
func Register(serverURL, tenantID, apiKey string) (*RegisterResponse, error) {
	hostname, _ := os.Hostname()

	client := resty.New().SetBaseURL(serverURL)

	req := RegisterRequest{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Version:  "0.1.0",
		TenantID: tenantID,
		APIKey:   apiKey,
	}

	var resp RegisterResponse
	r, err := client.R().
		SetBody(req).
		SetResult(&resp).
		Post("/agents/register")

	if err != nil {
		return nil, fmt.Errorf("Error registering agent: %w", err)
	}

	if r.StatusCode() != 200 {
		return nil, fmt.Errorf("Server rejected registration with code %d: %s", r.StatusCode(), r.String())
	}

	log.Printf("Agent successfully registered. ID: %s", resp.ID)
	return &resp, nil
}

// PollCommands pregunta al servidor si hay comandos pendientes
func (c *Communicator) PollCommands() ([]models.Command, error) {
	var commands []models.Command

	resp, err := c.client.R().
		SetResult(&commands).
		Get(fmt.Sprintf("/commands/pending/%s", c.agentID))

	if err != nil {
		return nil, fmt.Errorf("error in poll: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("server responded %d", resp.StatusCode())
	}

	return commands, nil
}

// ReportResult envía el resultado de un comando al servidor
func (c *Communicator) ReportResult(result models.Result) error {
	resp, err := c.client.R().
		SetBody(result).
		Post("/commands/result")

	if err != nil {
		return fmt.Errorf("error reporting result: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("server rejected result with code %d", resp.StatusCode())
	}

	log.Printf("Job %s result reported successfully", result.JobID)
	return nil
}

// Heartbeat le dice al servidor que el agente sigue vivo
func (c *Communicator) Heartbeat() error {
	_, err := c.client.R().
		Post(fmt.Sprintf("/agents/%s/heartbeat", c.agentID))
	return err
}

// SendInventory envía el inventario del agente al servidor
func (c *Communicator) SendInventory(inv *models.Inventory) error {
	resp, err := c.client.R().
		SetBody(inv).
		Post("/agents/inventory")

	if err != nil {
		return fmt.Errorf("error sending inventory: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("server rejected inventory with code %d: %s", resp.StatusCode(), resp.String())
	}

	log.Printf("Inventory sent successfully — %d software items, %d disks, %d NICs",
		len(inv.Software), len(inv.Disks), len(inv.NICs))
	return nil
}
