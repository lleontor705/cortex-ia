package hostbridge

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// agentShellMarker is the environment variable the Go CLI reads to refuse
// authority verbs. Shell commands may not mention it at all.
const agentShellMarker = "cortex_ia_agent_shell"

// shellWorkVerbs mirrors the CLI's agentShellDeniedWorkVerbs plus the legacy
// archive verb. A verb added to the CLI must be added here too.
var shellWorkVerbs = map[string]bool{
	"claim": true, "controller-renew": true, "renew": true, "lease": true, "reserve": true,
	"file-reserve": true, "lease-renew": true, "release": true, "release-all": true,
	"transition": true, "approve": true, "retry": true, "decompose": true, "recover": true,
	"reconcile": true, "prune": true, "degrade": true, "revise": true, "review-refresh": true,
	"archive": true,
}

var shellBoardVerbs = map[string]bool{"archive": true, "unarchive": true, "delete": true}

var (
	shellSplitter    = strings.NewReplacer(";", " ", "&", " ", "|", " ", "(", " ", ")", " ", "`", " ", "\n", " ", "\r", " ", "\t", " ")
	shellStripper    = strings.NewReplacer("'", "", `"`, "", `\`, "", "{", "", "}", "")
	globAlternatives = strings.NewReplacer("{", " ", "}", " ", ",", " ")
	sqliteDatabaseRE = regexp.MustCompile(`sqlite3\b[^|;&]*delegation\.db`)
)

// normalizeShell lowercases a command and drops quoting and escapes so that
// spellings such as 'work' or Approve compare equal to the plain verb.
func normalizeShell(command string) string {
	return strings.ToLower(shellStripper.Replace(command))
}

func shellTokens(command string) []string {
	return strings.Fields(shellSplitter.Replace(normalizeShell(command)))
}

// isCortexInvocation accepts cortex-ia by bare name, by path, and by a
// `NAME=cortex-ia` assignment.
func isCortexInvocation(token string) bool {
	if i := strings.LastIndex(token, "="); i >= 0 {
		token = token[i+1:]
	}
	if i := strings.LastIndex(token, "/"); i >= 0 {
		token = token[i+1:]
	}
	return token == "cortex-ia"
}

// shellInvokesAuthority finds `cortex-ia [global flags] work|board <verb>` at
// any token position, so wrappers such as env, sh -c or backticks cannot hide
// it. A $variable in the program position is treated as a possible invocation.
// Verbs are compared as whole tokens, so approve-x does not match approve.
func shellInvokesAuthority(tokens []string) bool {
	for i, tok := range tokens {
		if !isCortexInvocation(tok) && !strings.HasPrefix(tok, "$") {
			continue
		}
		j := i + 1
		for j < len(tokens) && strings.HasPrefix(tokens[j], "-") {
			j++
		}
		if j+1 >= len(tokens) {
			continue
		}
		switch tokens[j] {
		case "work":
			if shellWorkVerbs[tokens[j+1]] {
				return true
			}
		case "board":
			if shellBoardVerbs[tokens[j+1]] {
				return true
			}
		}
	}
	return false
}

// shellDenyReason returns why a shell command must be refused, or "" when it
// may run.
func shellDenyReason(command string) string {
	if strings.Contains(normalizeShell(command), agentShellMarker) {
		return "the agent-shell marker is set by cortex-ia and may not be read or changed from a shell"
	}
	if shellInvokesAuthority(shellTokens(command)) {
		return "work-authority changes go through the cortex_ia_* tools, not the shell"
	}
	if sqliteDatabaseRE.MatchString(normalizeShell(command)) {
		return "work-authority changes go through the cortex_ia_* tools, not the shell"
	}
	return ""
}

// isEnvFile reports whether rel names a dotenv-style secret file, ignoring case.
func isEnvFile(rel string) bool {
	base := strings.ToLower(path.Base(filepath.ToSlash(rel)))
	return base == ".env" || base == ".envrc" ||
		strings.HasPrefix(base, ".env.") || strings.HasPrefix(base, ".env-") ||
		strings.HasSuffix(base, ".env")
}

// envFilterDenied reports whether a Glob or Grep file filter could select an
// env file. It is deliberately conservative: any "env" substring denies, and a
// dot-prefixed pattern that matches ".env" denies. A false deny only routes
// the orchestrator to investigate.
func envFilterDenied(filter string) bool {
	lower := strings.ToLower(filter)
	if strings.Contains(lower, "env") {
		return true
	}
	for _, alt := range strings.Fields(globAlternatives.Replace(lower)) {
		last := alt[strings.LastIndex(alt, "/")+1:]
		matched, err := path.Match(last, ".env")
		if err != nil || (strings.HasPrefix(last, ".") && matched) {
			return true
		}
	}
	return false
}
