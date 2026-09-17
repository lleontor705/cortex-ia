package delegation

import "testing"

func TestDelegationDefaultsEnableUnattendedPermissions(t *testing.T) {
	for _, herdr := range []bool{false, true} {
		cfg := DefaultDelegationConfig(herdr)
		if err := cfg.Validate(); err != nil {
			t.Fatal(err)
		}
		for role, settings := range cfg.Roles {
			if !settings.Delegate || settings.CLI != "agy" || !settings.SkipPermissions {
				t.Fatalf("role %s must delegate with unattended permissions: %+v", role, settings)
			}
		}
	}
}

func TestLoadPreservesExplicitPermissionSetting(t *testing.T) {
	for _, bypass := range []bool{false, true} {
		t.Run(map[bool]string{false: "permissions", true: "explicit_bypass"}[bypass], func(t *testing.T) {
			cfg := DefaultDelegationConfig(false)
			for name, role := range cfg.Roles {
				role.SkipPermissions = bypass
				cfg.Roles[name] = role
			}
			dir := t.TempDir()
			if err := Save(dir, cfg); err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			for name, role := range loaded.Roles {
				if role.SkipPermissions != bypass {
					t.Fatalf("role %s lost explicit permission setting", name)
				}
			}
		})
	}
}
