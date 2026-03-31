#!/bin/bash
# E2E test suite for cortex-ia
set -euo pipefail

PASS=0
FAIL=0
TESTS=()

pass() { PASS=$((PASS+1)); echo "  [PASS] $1"; }
fail() { FAIL=$((FAIL+1)); echo "  [FAIL] $1: $2"; }

run_test() {
    local name="$1"
    shift
    TESTS+=("$name")
    if eval "$@" >/dev/null 2>&1; then
        pass "$name"
    else
        fail "$name" "exit code $?"
    fi
}

assert_file_exists() {
    [ -f "$1" ] || { echo "  Missing: $1"; return 1; }
}

assert_contains() {
    grep -q "$2" "$1" || { echo "  $1 does not contain '$2'"; return 1; }
}

echo "=== cortex-ia E2E Tests ==="
echo "Platform: $(uname -s)/$(uname -m)"
echo "Home: $HOME"
echo ""

# --- Version ---
echo "--- Version ---"
run_test "version" "cortex-ia version | grep -q cortex-ia"

# --- Detect ---
echo "--- Detect ---"
run_test "detect-runs" "cortex-ia detect"

# --- Install (dry-run) ---
echo "--- Install (dry-run) ---"
run_test "install-dryrun" "cortex-ia install --dry-run --preset full"
run_test "install-dryrun-no-files" "[ ! -d $HOME/.cortex-ia/skills ]"

# --- Install (real) ---
echo "--- Install ---"
# Create a fake claude-code binary so detect finds it
mkdir -p "$HOME/bin"
echo '#!/bin/bash' > "$HOME/bin/claude"
chmod +x "$HOME/bin/claude"
export PATH="$HOME/bin:$PATH"

run_test "install-full" "cortex-ia install --preset full"
run_test "state-exists" "assert_file_exists $HOME/.cortex-ia/state.json"
run_test "lock-exists" "assert_file_exists $HOME/.cortex-ia/cortex-ia.lock"
run_test "skills-dir" "[ -d $HOME/.cortex-ia/skills ]"
run_test "convention-file" "assert_file_exists $HOME/.cortex-ia/skills/_shared/cortex-convention.md"
run_test "orchestrator-prompt" "assert_file_exists $HOME/.cortex-ia/prompts/orchestrator.md"
run_test "bootstrap-skill" "assert_file_exists $HOME/.cortex-ia/skills/bootstrap/SKILL.md"
run_test "implement-skill" "assert_file_exists $HOME/.cortex-ia/skills/implement/SKILL.md"
run_test "skill-count" "[ $(ls $HOME/.cortex-ia/skills/*/SKILL.md | wc -l) -ge 19 ]"

# Verify convention refs are absolute, not relative
run_test "no-relative-refs" "! grep -r '../_shared/cortex-convention.md' $HOME/.cortex-ia/skills/*/SKILL.md"
run_test "absolute-convention-ref" "grep -q '.cortex-ia/skills/_shared/cortex-convention.md' $HOME/.cortex-ia/skills/bootstrap/SKILL.md"

# --- Doctor ---
echo "--- Doctor ---"
run_test "doctor-pass" "cortex-ia doctor"

# --- Idempotency ---
echo "--- Idempotency ---"
FIRST_LOCK=$(cat "$HOME/.cortex-ia/cortex-ia.lock")
run_test "reinstall" "cortex-ia install --preset full"
SECOND_LOCK=$(cat "$HOME/.cortex-ia/cortex-ia.lock")
# Locks should have same agent/component content (timestamps differ)
run_test "idempotent-agents" "echo '$FIRST_LOCK' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"installed_agents\"])' | diff - <(echo '$SECOND_LOCK' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"installed_agents\"])')"

# --- Repair ---
echo "--- Repair ---"
# Delete a managed file
rm -f "$HOME/.cortex-ia/skills/bootstrap/SKILL.md"
run_test "doctor-detects-missing" "! cortex-ia doctor"
run_test "repair" "cortex-ia repair"
run_test "repair-restored-file" "assert_file_exists $HOME/.cortex-ia/skills/bootstrap/SKILL.md"
run_test "doctor-pass-after-repair" "cortex-ia doctor"

# --- Rollback ---
echo "--- Rollback ---"
run_test "rollback" "cortex-ia rollback"

# --- Config ---
echo "--- Config ---"
run_test "config" "cortex-ia config | grep -q 'Agents:'"

# --- List ---
echo "--- List ---"
run_test "list-agents" "cortex-ia list agents"
run_test "list-components" "cortex-ia list components | grep -q cortex"
run_test "list-backups" "cortex-ia list backups"

# --- Update ---
echo "--- Update ---"
run_test "update-check" "cortex-ia update"

# --- Summary ---
echo ""
echo "=== Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo "Total:  $((PASS + FAIL))"

[ $FAIL -eq 0 ] || exit 1
