package models

import "time"

type Agent struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	Token    string `json:"token"`
}

type Command struct {
	ID      string `json:"id"` // la API retorna "id" no "job_id"
	AgentID string `json:"agent_id"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Timeout int    `json:"timeout"`
	Status  string `json:"status"`
}

type Result struct {
	JobID      string    `json:"job_id"`
	AgentID    string    `json:"agent_id"`
	ExitCode   int       `json:"exit_code"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	Error      string    `json:"error,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}
