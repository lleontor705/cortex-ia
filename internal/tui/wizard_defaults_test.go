package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestWizardDelegationSetsUnattendedPermissions(t *testing.T) {
	for cursor, name := range delegationRoles {
		for _, bypass := range []bool{false, true} {
			m := model{delegationCfg: delegation.NormalConfig(), wizardCursor: cursor}
			r := m.delegationCfg.Roles[name]
			r.SkipPermissions = bypass
			m.delegationCfg.Roles[name] = r
			for _, enabled := range []bool{true, false, true} {
				updated, cmd := m.updateWizardRoles(tea.KeyMsg{Type: tea.KeyTab})
				m = updated.(model)
				if cmd != nil {
					t.Fatal("role selection must not launch installation")
				}
				got := m.opts.DelegationConfig.Roles[name]
				if got.Delegate != enabled || got.SkipPermissions != enabled {
					t.Fatalf("role %s enabled=%v must set matching unattended permissions: %+v", name, enabled, got)
				}
				if err := m.delegationCfg.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestWizardDelegationDisabledClearsUnattendedPermissions(t *testing.T) {
	m := model{delegationCfg: delegation.DefaultDelegationConfig(false), wizardCursor: 1}
	for name, role := range m.delegationCfg.Roles {
		role.SkipPermissions = true
		m.delegationCfg.Roles[name] = role
	}
	updated, _ := m.updateWizardDelegation(tea.KeyMsg{Type: tea.KeyEnter})
	// The returned planning command is deliberately not executed.
	m = updated.(model)
	if m.delegationCfg.DelegationEnabled {
		t.Fatal("delegation must be disabled")
	}
	for name, role := range m.opts.DelegationConfig.Roles {
		if role.Delegate || role.CLI != "native" || role.SkipPermissions {
			t.Fatalf("role %s must be native without unattended permissions: %+v", name, role)
		}
	}
}
