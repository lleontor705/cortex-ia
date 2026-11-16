package hostbridge

import "testing"

func TestDecideOrchestratorReadScope(t *testing.T) {
	cases := []struct {
		name   string
		tool   string
		args   map[string]any
		want   Verdict
		reason string
	}{
		{"source file", "Read", map[string]any{"file_path": "/repo/src/x.go"}, PassThrough, ""},
		{"discovery doc", "Read", map[string]any{"file_path": "/repo/.cortex-ia/discovery.md"}, PassThrough, ""},
		{"relative path", "Read", map[string]any{"file_path": "internal/app/app.go"}, PassThrough, ""},
		{"env file", "Read", map[string]any{"file_path": "/repo/.env"}, Deny, "env files"},
		{"nested env variant", "Read", map[string]any{"file_path": "/repo/config/.env.production"}, Deny, "env files"},
		{"outside workspace", "Read", map[string]any{"file_path": "/etc/hosts"}, Deny, "only inside the workspace"},
		{"parent traversal", "Read", map[string]any{"file_path": "../secrets.txt"}, Deny, "only inside the workspace"},
		{"read without path", "Read", map[string]any{}, Deny, "only inside the workspace"},
		{"grep workspace", "Grep", map[string]any{"pattern": "func Decide"}, PassThrough, ""},
		{"grep subdirectory", "Grep", map[string]any{"pattern": "Decide", "path": "/repo/internal"}, PassThrough, ""},
		{"grep env glob", "Grep", map[string]any{"pattern": "KEY", "glob": "*.env"}, Deny, "env files"},
		{"grep env brace glob", "Grep", map[string]any{"pattern": "KEY", "glob": "*.{env,txt}"}, Deny, "env files"},
		{"grep outside workspace", "Grep", map[string]any{"pattern": "x", "path": "/etc"}, Deny, "only inside the workspace"},
		{"glob source", "Glob", map[string]any{"pattern": "**/*.go"}, PassThrough, ""},
		{"glob env pattern", "Glob", map[string]any{"pattern": "**/.env*"}, Deny, "env files"},
		{"glob outside workspace", "Glob", map[string]any{"pattern": "*", "path": "/Users"}, Deny, "only inside the workspace"},
		{"upper env file", "Read", map[string]any{"file_path": "/repo/.ENV"}, Deny, "env files"},
		{"mixed case env local", "Read", map[string]any{"file_path": "/repo/.Env.local"}, Deny, "env files"},
		{"suffix env file", "Read", map[string]any{"file_path": "/repo/config/app.env"}, Deny, "env files"},
		{"envrc file", "Read", map[string]any{"file_path": "/repo/.envrc"}, Deny, "env files"},
		{"dash env file", "Read", map[string]any{"file_path": "/repo/.env-local"}, Deny, "env files"},
		{"environment source name", "Read", map[string]any{"file_path": "/repo/src/environment.go"}, PassThrough, ""},
		{"grep upper env glob", "Grep", map[string]any{"pattern": "KEY", "glob": "*.ENV"}, Deny, "env files"},
		{"grep env wildcard glob", "Grep", map[string]any{"pattern": "KEY", "glob": "*env*"}, Deny, "env files"},
		{"grep dotfile glob", "Grep", map[string]any{"pattern": "KEY", "glob": "**/.*"}, Deny, "env files"},
		{"grep brace source glob", "Grep", map[string]any{"pattern": "KEY", "glob": "*.{go,ts}"}, PassThrough, ""},
		{"glob mixed case env", "Glob", map[string]any{"pattern": "**/.En*"}, Deny, "env files"},
		{"glob brace dotfile", "Glob", map[string]any{"pattern": "{.en*,x}"}, Deny, "env files"},
		{"glob dotgit", "Glob", map[string]any{"pattern": "**/.git*"}, PassThrough, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustDecide(t, claudeInput("orchestrator", tc.tool, tc.args), Deps{})
			assertVerdict(t, d, tc.want, tc.reason)
		})
	}
}
