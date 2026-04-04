package agent

import (
	"testing"
	"time"

	"github.com/sentineledge/agent/pkg/models"
)

// mockComm is a test double for the agentComm interface.
type mockComm struct {
	heartbeatCalled int
}

func (m *mockComm) PollCommands() ([]models.Command, error)    { return nil, nil }
func (m *mockComm) ReportResult(r models.Result) error         { return nil }
func (m *mockComm) SendInventory(inv *models.Inventory) error  { return nil }
func (m *mockComm) Heartbeat() error {
	m.heartbeatCalled++
	return nil
}

func TestSendHeartbeat_CallsHeartbeat(t *testing.T) {
	mock := &mockComm{}
	a := &Agent{config: &Config{}, comm: mock}

	a.sendHeartbeat()

	if mock.heartbeatCalled != 1 {
		t.Errorf("expected Heartbeat() called once, got %d", mock.heartbeatCalled)
	}
}

func TestRun_SendsHeartbeatOnStartup(t *testing.T) {
	mock := &mockComm{}
	// PollInterval=3600 so poll ticker won't fire during this short test
	a := &Agent{config: &Config{PollInterval: 3600}, comm: mock}

	go a.Run()

	// Allow goroutines launched by Run() to execute
	time.Sleep(100 * time.Millisecond)

	if mock.heartbeatCalled < 1 {
		t.Errorf("expected Heartbeat() called at least once on startup, got %d", mock.heartbeatCalled)
	}
}

func TestRun_SendsHeartbeatPeriodically(t *testing.T) {
	mock := &mockComm{}
	// HeartbeatInterval=1 (second) so ticker fires during this test
	a := &Agent{config: &Config{PollInterval: 3600, HeartbeatInterval: 1}, comm: mock}

	go a.Run()

	// Wait long enough for at least 2 heartbeat ticks (startup + 1 periodic)
	time.Sleep(1500 * time.Millisecond)

	if mock.heartbeatCalled < 2 {
		t.Errorf("expected Heartbeat() called at least twice (startup + periodic), got %d", mock.heartbeatCalled)
	}
}
