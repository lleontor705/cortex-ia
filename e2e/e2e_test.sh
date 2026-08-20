#!/usr/bin/env bash
# E2E test suite for cortex-ia
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

CORTEX_IA="$(resolve_binary)"
if [ -z "$CORTEX_IA" ]; then
    log_fail "cortex-ia binary not found (checked repo root, ~/cortex-ia, and PATH)"
    print_summary
    exit 1
fi

# run_cmd NAME CMD...
# Runs CMD silently, logs pass/fail by exit code.
run_cmd() {
    local name="$1"
    shift
    if eval "$@" >/dev/null 2>&1; then
        log_pass "$name"
    else
        log_fail "$name (exit $?)"
    fi
}

echo "=== cortex-ia E2E Tests ==="
echo "Platform: $(uname -s)/$(uname -m)"
echo "Home: $HOME"
echo "Binary: $CORTEX_IA"
echo ""

# --- Version ---
log_test "Version"
run_cmd "version" "$CORTEX_IA version | grep -q cortex-ia"

# --- Detect ---
log_test "Detect"
run_cmd "detect-runs" "$CORTEX_IA detect"

# --- Install (dry-run) ---
log_test "Install (dry-run)"
run_cmd "install-dryrun" "$CORTEX_IA install --dry-run --preset full"
run_cmd "install-dryrun-no-files" "[ ! -d \"$HOME/.cortex-ia/skills\" ]"

# --- Install (real) ---
log_test "Install"
setup_fake_agent_binary "claude"

run_cmd "install-full" "$CORTEX_IA install --preset full"
assert_file_exists "$HOME/.cortex-ia/state.json" "state.json"
assert_file_exists "$HOME/.cortex-ia/cortex-ia.lock" "lockfile"
assert_dir_exists "$HOME/.cortex-ia/skills" "skills dir"
assert_file_exists "$HOME/.cortex-ia/skills/_shared/cortex-convention.md" "convention file"
assert_file_exists "$HOME/.cortex-ia/prompts/orchestrator.md" "orchestrator prompt"
assert_file_exists "$HOME/.cortex-ia/skills/bootstrap/SKILL.md" "bootstrap skill"
assert_file_exists "$HOME/.cortex-ia/skills/implement/SKILL.md" "implement skill"
assert_file_count_min "$HOME/.cortex-ia/skills" "SKILL.md" 19 "skill count >= 19"

# Verify convention refs are absolute, not relative
run_cmd "no-relative-refs" "! grep -r '../_shared/cortex-convention.md' $HOME/.cortex-ia/skills/*/SKILL.md"
assert_file_contains "$HOME/.cortex-ia/skills/bootstrap/SKILL.md" \
    ".cortex-ia/skills/_shared/cortex-convention.md" \
    "bootstrap uses absolute convention ref"

# --- Doctor ---
log_test "Doctor"
run_cmd "doctor-pass" "$CORTEX_IA doctor"

# --- Idempotency ---
log_test "Idempotency"
FIRST_LOCK=$(cat "$HOME/.cortex-ia/cortex-ia.lock")
run_cmd "reinstall" "$CORTEX_IA install --preset full"
SECOND_LOCK=$(cat "$HOME/.cortex-ia/cortex-ia.lock")
run_cmd "idempotent-agents" "echo '$FIRST_LOCK' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"installed_agents\"])' | diff - <(echo '$SECOND_LOCK' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"installed_agents\"])')"

# --- Repair ---
log_test "Repair"
rm -f "$HOME/.cortex-ia/skills/bootstrap/SKILL.md"
run_cmd "doctor-detects-missing" "! $CORTEX_IA doctor"
run_cmd "repair" "$CORTEX_IA repair"
assert_file_exists "$HOME/.cortex-ia/skills/bootstrap/SKILL.md" "bootstrap restored after repair"
run_cmd "doctor-pass-after-repair" "$CORTEX_IA doctor"

# --- Rollback ---
log_test "Rollback"
run_cmd "rollback" "$CORTEX_IA rollback"

# --- Config ---
log_test "Config"
run_cmd "config" "$CORTEX_IA config | grep -q 'Agents:'"

# --- List ---
log_test "List"
run_cmd "list-agents" "$CORTEX_IA list agents"
run_cmd "list-components" "$CORTEX_IA list components | grep -q cortex"
run_cmd "list-backups" "$CORTEX_IA list backups"
run_cmd "list-profiles" "$CORTEX_IA list profiles"
run_cmd "list-skills" "$CORTEX_IA list skills"

# --- New CLI commands smoke ---
log_test "New CLI commands"

# GGA switcher: --list and --provider must exit 0 and update the config file.
run_cmd "gga-list" "$CORTEX_IA gga --list | grep -q anthropic"
run_cmd "gga-set-anthropic" "$CORTEX_IA gga --provider anthropic"
assert_file_exists "$HOME/.config/gga/config" "gga config file"
assert_file_contains "$HOME/.config/gga/config" 'PROVIDER="anthropic"' "gga config has anthropic provider"

# Profiles: list when empty, then create + apply round-trip.
run_cmd "profiles-list-empty" "$CORTEX_IA profiles list"
run_cmd "profiles-create" "$CORTEX_IA profiles create cheap:openai/gpt-4o-mini"
run_cmd "profiles-list-cheap" "$CORTEX_IA profiles list | grep -q cheap"
run_cmd "profiles-set-phase" "$CORTEX_IA profiles set cheap:sdd-design:anthropic/claude-opus-4"
run_cmd "profiles-delete" "$CORTEX_IA profiles delete cheap"

# Agent Builder: list (empty registry is fine) + remove of unknown returns non-zero.
run_cmd "agent-builder-list" "$CORTEX_IA agent-builder list"
run_cmd "agent-builder-remove-missing" "! $CORTEX_IA agent-builder remove definitely-not-installed"

# Uninstall dry-run must not write anything destructive.
run_cmd "uninstall-dry-run" "$CORTEX_IA uninstall --component persona --dry-run"
assert_file_exists "$HOME/.cortex-ia/state.json" "state.json still present after dry-run"

# --- Update ---
log_test "Update"
run_cmd "update-check" "$CORTEX_IA update"

# --- Summary ---
print_summary
