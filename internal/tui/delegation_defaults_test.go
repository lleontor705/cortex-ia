package tui

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestDelegationRoleToggleSetsUnattendedPermissions(t *testing.T) {
	for cursor, name := range delegationRoles {
		for _, bypass := range []bool{false, true} {
			m := model{delegationCfg: delegation.NormalConfig()}
			role := m.delegationCfg.Roles[name]
			role.SkipPermissions = bypass
			m.delegationCfg.Roles[name] = role
			for _, enabled := range []bool{true, false, true} {
				m = m.toggleDelegationItem(cursor + 3)
				got := m.delegationCfg.Roles[name]
				if got.Delegate != enabled || got.SkipPermissions != enabled {
					t.Fatalf("role %s toggle=%v must set matching unattended permissions: %+v", name, enabled, got)
				}
				if err := m.delegationCfg.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
