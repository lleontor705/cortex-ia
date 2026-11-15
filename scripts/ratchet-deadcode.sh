#!/usr/bin/env bash
#
# Dead-code ratchet for cortex-ia.
#
# Counts functions defined by this module's own packages that the Go linker
# drops from the linked cmd/cortex-ia binary. The measurement is a pure
# toolchain symbol diff: every text symbol in the package archives (all
# defined functions) minus every text symbol in the binary (linked
# functions). Inlining is disabled on both sides (-gcflags=all=-l) so that
# inlined functions are not miscounted as dead; the resulting number is a
# stable regression signal, not an absolute dead-code audit.
#
# Advisory by contract: this script always exits 0, drift or not. `--regen`
# recomputes the baseline and prints a unified diff, leaving the committed
# baseline untouched; add `--confirm` to atomically replace it.

set -uo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/ratchet-deadcode.sh [--regen] [--confirm]

  (no flags)   Report drift against the committed baseline (advisory, exit 0).
  --regen      Recompute the baseline and print a unified diff; keep it untouched.
  --confirm    With --regen, replace the committed baseline with the recomputed one.
EOF
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BASELINE="${ROOT_DIR}/.cortex-ia/ratchet/deadcode-baseline.txt"
MAIN_PKG="./cmd/cortex-ia"
GC_FLAGS="-gcflags=all=-l"
LIST_LIMIT=40

regen=0
confirm=0
for arg in "$@"; do
	case "$arg" in
		--regen) regen=1 ;;
		--confirm) confirm=1 ;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			echo "ratchet-deadcode: unknown argument '${arg}'" >&2
			usage >&2
			;;
	esac
done

cd "$ROOT_DIR" || exit 0

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

infra_skip() {
	echo "ratchet-deadcode: SKIPPED (${1}; advisory infrastructure error)"
	exit 0
}

modpath="$(go list -m -f '{{.Path}}' 2>/dev/null)"
[ -n "$modpath" ] || infra_skip "cannot resolve module path"

if ! go list -export -f '{{.Export}}' "${GC_FLAGS}" ./... >"${workdir}/archives.txt" 2>"${workdir}/list.log"; then
	infra_skip "go list failed"
fi

go build "${GC_FLAGS}" -o "${workdir}/cortex-ia" "${MAIN_PKG}" 2>"${workdir}/build.log"
if [ ! -x "${workdir}/cortex-ia" ]; then
	infra_skip "build failed"
fi

while IFS= read -r archive; do
	[ -n "$archive" ] && go tool nm "$archive" 2>/dev/null
done <"${workdir}/archives.txt" |
	awk '$2 ~ /^[Tt]$/ {print $3}' |
	grep -F "${modpath}/" |
	LC_ALL=C sort -u >"${workdir}/defined.txt"

go tool nm "${workdir}/cortex-ia" 2>/dev/null |
	awk '$2 ~ /^[Tt]$/ {print $3}' |
	LC_ALL=C sort -u >"${workdir}/linked.txt"

comm -23 "${workdir}/defined.txt" "${workdir}/linked.txt" >"${workdir}/current.txt"

if [ -f "$BASELINE" ]; then
	LC_ALL=C sort -u "$BASELINE" >"${workdir}/baseline.txt"
else
	: >"${workdir}/baseline.txt"
fi

current_count="$(wc -l <"${workdir}/current.txt" | tr -d ' ')"
baseline_count="$(wc -l <"${workdir}/baseline.txt" | tr -d ' ')"

if [ "$regen" -eq 1 ]; then
	if [ ! -f "$BASELINE" ]; then
		echo "ratchet-deadcode: no committed baseline; would record ${current_count} dead functions"
	elif diff -q "$BASELINE" "${workdir}/current.txt" >/dev/null 2>&1; then
		echo "ratchet-deadcode: baseline already current (${current_count} dead functions)"
	else
		echo "ratchet-deadcode: proposed baseline diff (committed -> current):"
		diff -u "$BASELINE" "${workdir}/current.txt" 2>/dev/null || true
	fi
	if [ "$confirm" -eq 1 ]; then
		mkdir -p "$(dirname "$BASELINE")"
		cp "${workdir}/current.txt" "$BASELINE"
		chmod 644 "$BASELINE"
		echo "ratchet-deadcode: baseline replaced (${current_count} dead functions)"
	else
		echo "ratchet-deadcode: --regen only; committed baseline left untouched (re-run with --confirm to write)"
	fi
	exit 0
fi

if [ "$current_count" -eq "$baseline_count" ]; then
	echo "ratchet-deadcode: OK (${current_count} dead functions, matches baseline)"
	exit 0
fi

delta=$((current_count - baseline_count))
echo "ratchet-deadcode: DRIFT detected (advisory, merge is not blocked)"
printf '  baseline: %s   current: %s   delta: %+d\n' "$baseline_count" "$current_count" "$delta"

newly_dead="${workdir}/newly-dead.txt"
revived="${workdir}/revived.txt"
comm -13 "${workdir}/baseline.txt" "${workdir}/current.txt" >"$newly_dead"
comm -23 "${workdir}/baseline.txt" "${workdir}/current.txt" >"$revived"

report_list() {
	local label="$1" file="$2" total
	total="$(wc -l <"$file" | tr -d ' ')"
	[ "$total" -gt 0 ] || return 0
	echo "  ${label} (${total}):"
	head -n "$LIST_LIMIT" "$file" | sed 's/^/    /'
	[ "$total" -le "$LIST_LIMIT" ] || echo "    ... and $((total - LIST_LIMIT)) more"
}

report_list "newly dead" "$newly_dead"
report_list "now reachable" "$revived"
echo "  run 'scripts/ratchet-deadcode.sh --regen' to review a new baseline"

exit 0
