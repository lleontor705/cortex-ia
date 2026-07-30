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
