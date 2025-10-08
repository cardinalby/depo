package domain

import "time"

type ComponentStatus string

const (
	StatusNonRunnable ComponentStatus = "non_runnable"
	StatusPending     ComponentStatus = "pending"
	StatusStarting    ComponentStatus = "starting"
	StatusReady       ComponentStatus = "ready"
	StatusClosing     ComponentStatus = "closing"
	StatusDone        ComponentStatus = "done"
)

type Component struct {
	ID         uint64          `json:"id"`
	Name       string          `json:"name"`
	DependsOn  []uint64        `json:"depends_on,omitempty"`
	StartError string          `json:"start_error,omitempty"`
	Delay      time.Duration   `json:"delay"`
	Status     ComponentStatus `json:"status"`
	DoneError  string          `json:"done_error,omitempty"`
}

type Graph struct {
	Components             []Component     `json:"components"`
	Status                 ComponentStatus `json:"status"`
	RunnerError            string          `json:"runner_error"`
	ShutDownOnNilRunResult bool            `json:"shut_down_on_nil_run_result"`
}
