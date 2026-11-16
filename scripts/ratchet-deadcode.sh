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
# Two symbols are excluded from the defined set before diffing, because the
# linker's dead-code pass decides their fate by dynamic dispatch or by the
# compiler's own codegen rather than by a function the module wrote and nothing calls:
#   1. Value-receiver wrappers: for a value method T.M the compiler also emits
#      pkg.(*T).M so pointers satisfy interfaces. Go forbids declaring both forms,
#      so when pkg.T.M is defined, pkg.(*T).M is generated and is not counted.
#   2. Interface-dispatched methods: a method whose name is declared by an
#      interface in this module may be kept or dropped at run time, which the
#      symbol table cannot tell from a static dead method. Such names are not
#      counted. Limit: interfaces from the standard library (Error, String, ...)
#      are not scanned, so methods satisfying only those stay counted.
#   3. Compiler-generated type routines (type:.eq.*, type:.hash.*): equality and
#      hash functions the compiler emits per comparable type. No source function
#      exists for them.
#
# Advisory by contract: this script always exits 0, drift or not. `--regen`
# recomputes the baseline and prints a unified diff, leaving the committed
# baseline untouched; add `--confirm` to atomically replace it.

set -uo pipefail
# sort and comm must share one collation; a locale-aware comm against C-sorted
# input makes unmatched lines appear as false drift.
export LC_ALL=C

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

cat >"${workdir}/ifacenames.go" <<'GO'
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Prints every method name declared by an interface in the module, one per line.
// Test files are skipped: fakes there must not exempt production methods.
func main() {
	root := os.Args[1]
	fset := token.NewFileSet()
	names := map[string]bool{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if it, ok := n.(*ast.InterfaceType); ok && it.Methods != nil {
				for _, field := range it.Methods.List {
					for _, name := range field.Names {
						names[name.Name] = true
					}
				}
			}
			return true
		})
		return nil
	})
	for name := range names {
		fmt.Println(name)
	}
}
GO

go run "${workdir}/ifacenames.go" "$ROOT_DIR" >"${workdir}/iface-raw.txt" 2>"${workdir}/iface.log" ||
	infra_skip "interface method scan failed"
sort -u "${workdir}/iface-raw.txt" >"${workdir}/iface-methods.txt"

# Method symbols are pkg.T.M or pkg.(*T).M after the last slash; the method name is
# the final dot-separated part. Wrappers are pkg.(*T).M with pkg.T.M also defined.
LC_ALL=C awk '
	FNR == NR { iface[$0] = 1; next }
	{ syms[++n] = $0; present[$0] = 1 }
	END {
		for (i = 1; i <= n; i++) {
			sym = syms[i]
			if (index(sym, "type:") == 1) continue
			tail = sym
			sub(/^.*\//, "", tail)
			pre = substr(sym, 1, length(sym) - length(tail))
			k = split(tail, parts, ".")
			if (k < 3) { print sym; continue }
			if (parts[k] in iface) continue
			if (match(tail, /\.\(\*[^()]+\)\./)) {
				value = pre substr(tail, 1, RSTART - 1) "." substr(tail, RSTART + 3, RLENGTH - 5) "." parts[k]
				if (value in present) continue
			}
			print sym
		}
	}' "${workdir}/iface-methods.txt" "${workdir}/defined.txt" | LC_ALL=C sort -u >"${workdir}/counted.txt"

comm -23 "${workdir}/counted.txt" "${workdir}/linked.txt" >"${workdir}/current.txt"

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
