package app

import "testing"

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name    string
		ldflags string
		want    string
	}{
		{"ldflags set", "v1.2.3", "v1.2.3"},
		{"ldflags with whitespace", "  v1.0.0  ", "v1.0.0"},
		{"ldflags dev falls through", "dev", "dev"},
		{"empty ldflags falls through", "", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveVersion(tt.ldflags)
			if got != tt.want {
				t.Errorf("ResolveVersion(%q) = %q, want %q", tt.ldflags, got, tt.want)
			}
		})
	}
}
