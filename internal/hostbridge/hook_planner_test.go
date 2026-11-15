package hostbridge

import "testing"

func TestDecidePlannerWritesOnlyItsPlanFile(t *testing.T) {
	deps := Deps{Leases: fakeLeases{valid: true}}
	cases := []struct {
		name   string
		target string
		want   Verdict
	}{
		{"plan file passes", "/repo/docs/plans/board-1.md", PassThrough},
		{"relative plan file passes", "docs/plans/board-1.md", PassThrough},
		{"subdirectory denied", "/repo/docs/plans/sub/x.md", Deny},
		{"traversal denied", "/repo/docs/plans/../x.md", Deny},
		{"sibling docs denied", "/repo/docs/other.md", Deny},
		{"source file denied", "/repo/src/main.go", Deny},
		{"outside workspace denied", "/elsewhere/docs/plans/board-1.md", Deny},
		{"uppercase name denied", "/repo/docs/plans/Board-1.md", Deny},
		{"non-markdown denied", "/repo/docs/plans/board-1.txt", Deny},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustDecide(t, claudeInput("planner", "Write", map[string]any{"file_path": tc.target}), deps)
			if d.Verdict != tc.want {
				t.Fatalf("verdict = %v, want %v (reason %q)", d.Verdict, tc.want, d.Reason)
			}
		})
	}
}
