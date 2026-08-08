package tui

import (
	"strings"
	"testing"
)

func TestWelcomeGroupsUseProviderNeutralRouteLabels(t *testing.T) {
	for _, group := range (Model{}).welcomeGroups() {
		for _, item := range group.Items {
			label := strings.ToLower(item.Label + " " + item.Hint)
			for _, forbidden := range []string{
				strings.Join([]string{"son", "net"}, ""),
				strings.Join([]string{"op", "us"}, ""),
				strings.Join([]string{"hai", "ku"}, ""),
			} {
				if strings.Contains(label, forbidden) {
					t.Errorf("welcome label contains forbidden model alias %q: %q", forbidden, label)
				}
			}
		}
	}
}

func TestWelcomeOptionsExposeOnlySupportedLifecycleActions(t *testing.T) {
	got := welcomeOptions()
	want := []WelcomeOption{
		WelcomeInstall,
		WelcomeSync,
		WelcomeBackups,
		WelcomeUpgrade,
		WelcomeUpgradeSync,
		WelcomeQuit,
	}
	if len(got) != len(want) {
		t.Fatalf("len(welcomeOptions()) = %d, want %d", len(got), len(want))
	}
	for i, option := range want {
		if got[i] != option {
			t.Errorf("welcomeOptions()[%d] = %v, want %v", i, got[i], option)
		}
	}

	for _, retired := range []WelcomeOption{
		WelcomeModelConfig,
		WelcomeProfiles,
		WelcomeAgentBuilder,
		WelcomeOpenCodeModels,
	} {
		for _, option := range got {
			if option == retired {
				t.Errorf("retired welcome option %v is reachable", retired)
			}
		}
	}
}
