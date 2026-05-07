package kiro

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// AgentNotInstallableError is returned when a caller attempts to auto-install
// an agent that must be installed manually (e.g. desktop apps like Kiro IDE).
type AgentNotInstallableError struct {
	Agent model.AgentID
}

func (e AgentNotInstallableError) Error() string {
	return fmt.Sprintf("agent %q cannot be auto-installed; install it manually", e.Agent)
}
