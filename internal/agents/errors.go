package agents

import "errors"

var (
	ErrAgentNotFound    = errors.New("agent not found in registry")
	ErrAgentNotDetected = errors.New("agent not detected on this system")
)
