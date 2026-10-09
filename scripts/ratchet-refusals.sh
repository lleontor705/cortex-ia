#!/usr/bin/env bash
#
# ratchet-refusals — pin CLI refusal/error strings to a committed baseline.
#
# Extracts the leading string literal of every errors.New(...) / fmt.Errorf(...)
# call in the CLI-facing Go packages, normalizes it, and reports any baseline
# string that was removed or altered. Advisory only: it never exits non-zero.
#
# Usage:
#   scripts/ratchet-refusals.sh                  report drift against the baseline
#   scripts/ratchet-refusals.sh --regen          emit a candidate baseline + diff
#   scripts/ratchet-refusals.sh --regen --confirm   replace the baseline
#
# Regen never swaps the committed baseline until --confirm is given, so the
# unified diff is always reviewable before commit.
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LABEL="ratchet-refusals"
BASELINE_REL=".cortex-ia/ratchet/refusals-baseline.txt"
BASELINE="${REPO_ROOT}/${BASELINE_REL}"
SOURCE_DIRS=(internal/app internal/install internal/delegation)

# errors.New("...") / fmt.Errorf("...") — first literal only, single line. The
# CLI packages keep refusal messages on one line, so no grammar parser is needed.
MARKER='(errors\.New|fmt\.Errorf)\("[^"]*"'

MODE="check"
CONFIRM=0
for arg in "$@"; do
  case "$arg" in
    --regen) MODE="regen" ;;
    --confirm) CONFIRM=1 ;;
    --help|-h) MODE="help" ;;
    *) printf '[%s] ignoring unknown argument: %s\n' "$LABEL" "$arg" >&2 ;;
  esac
done

print_usage() {
  cat <<EOF
$LABEL — pin CLI refusal/error strings to a committed baseline (advisory).

Usage:
  scripts/$LABEL.sh                    report drift against the baseline (exit 0)
  scripts/$LABEL.sh --regen            emit candidate baseline + unified diff
  scripts/$LABEL.sh --regen --confirm  replace the baseline after review

Sources:  ${SOURCE_DIRS[*]}
Baseline: $BASELINE_REL
EOF
}

if [ "$MODE" = "help" ]; then
  print_usage
  exit 0
fi

# Emits "repo-relative/path.go<TAB>normalized message" for every refusal literal.
extract_candidate() {
  local out="$1"
  (
    cd "$REPO_ROOT" || exit 0
    for dir in "${SOURCE_DIRS[@]}"; do
      [ -d "$dir" ] || continue
      grep -rHnoE --include='*.go' --exclude='*_test.go' "$MARKER" "$dir"
    done
  ) | awk '
    {
      p = index($0, ":")
      if (p == 0) next
      path = substr($0, 1, p - 1)
      rest = substr($0, p + 1)
      q = index(rest, ":")
      if (q == 0) next
      hit = substr(rest, q + 1)
      dq = index(hit, "\"")
      if (dq == 0) next
      msg = substr(hit, dq + 1)
      sub(/"$/, "", msg)
      gsub(/[[:space:]]+/, " ", msg)
      sub(/^ /, "", msg)
      sub(/ $/, "", msg)
      if (msg == "") next
      printf "%s\t%s\n", path, msg
    }
  ' | LC_ALL=C sort -u > "$out"
}

write_candidate_file() {
  local candidate="$1" composed="$2"
  {
    printf '# %s baseline — normalized CLI refusal/error strings.\n' "$LABEL"
    printf '# Sources: %s (Go, excluding *_test.go).\n' "${SOURCE_DIRS[*]}"
    printf '# Regenerate: scripts/%s.sh --regen --confirm\n' "$LABEL"
    printf '# Format: <repo-relative go file>\\t<normalized message>\n'
    cat "$candidate"
  } > "$composed"
}

run_check() {
  local candidate report removed_locs
  candidate="$(mktemp)"
  report="$(mktemp)"
  removed_locs="$(mktemp)"
  extract_candidate "$candidate"

  if [ ! -f "$BASELINE" ]; then
    printf '[%s] no baseline at %s — run: scripts/%s.sh --regen --confirm\n' \
      "$LABEL" "$BASELINE_REL" "$LABEL"
    rm -f "$candidate" "$report" "$removed_locs"
    return 0
  fi

  awk -F'\t' '
    FILENAME == ARGV[1] { cur[$2] = $1; next }
    /^#/ { next }
    NF < 2 { next }
    { base[$2] = $1 }
    END {
      for (m in base) if (!(m in cur)) printf "REMOVED\t%s\t%s\n", base[m], m
      for (m in cur) if (!(m in base)) printf "NEW\t%s\t%s\n", cur[m], m
    }
  ' "$candidate" "$BASELINE" | LC_ALL=C sort > "$report"

  local removed_count new_count
  removed_count="$(awk -F'\t' '$1=="REMOVED"' "$report" | wc -l | tr -d ' ')"
  new_count="$(awk -F'\t' '$1=="NEW"' "$report" | wc -l | tr -d ' ')"

  if [ "$removed_count" -gt 0 ]; then
    printf '[%s] DRIFT: %s refusal string(s) removed or altered vs %s:\n' \
      "$LABEL" "$removed_count" "$BASELINE_REL"
    awk -F'\t' '$1=="REMOVED" { printf "  REMOVED  %s  ::  %s\n", $2, $3 }' "$report"
    printf '[%s] advisory only — non-zero exit is never used for drift\n' "$LABEL"
  else
    printf '[%s] no drift (%s baseline strings intact)\n' \
      "$LABEL" "$(grep -vcE '^#|^$' "$BASELINE" 2>/dev/null || printf '?')"
  fi

  if [ "$new_count" -gt 0 ]; then
    printf '[%s] %s new string(s) not yet in baseline (informational):\n' "$LABEL" "$new_count"
    awk -F'\t' '$1=="NEW" { printf "  NEW      %s  ::  %s\n", $2, $3 }' "$report"
  fi

  rm -f "$candidate" "$report" "$removed_locs"
  return 0
}

run_regen() {
  local candidate composed diffout
  candidate="$(mktemp)"
  composed="$(mktemp)"
  diffout="$(mktemp)"
  extract_candidate "$candidate"
  write_candidate_file "$candidate" "$composed"

  if [ -f "$BASELINE" ]; then
    if diff -u "$BASELINE" "$composed" > "$diffout" 2>&1; then
      printf '[%s] candidate baseline is identical to %s\n' "$LABEL" "$BASELINE_REL"
    else
      printf '[%s] candidate baseline diff vs %s:\n' "$LABEL" "$BASELINE_REL"
      cat "$diffout"
    fi
  else
    printf '[%s] no committed baseline yet; candidate preview:\n' "$LABEL"
    cat "$composed"
  fi

  if [ "$CONFIRM" -eq 1 ]; then
    mkdir -p "$(dirname "$BASELINE")"
    cp "$composed" "${BASELINE}.tmp" && mv "${BASELINE}.tmp" "$BASELINE"
    printf '[%s] wrote %s (%s entries)\n' \
      "$LABEL" "$BASELINE_REL" "$(grep -vcE '^#|^$' "$BASELINE")"
  else
    printf '[%s] baseline untouched — re-run with --confirm to write it\n' "$LABEL"
  fi

  rm -f "$candidate" "$composed" "$diffout"
  return 0
}

if [ "$MODE" = "regen" ]; then
  run_regen
else
  run_check
fi

exit 0
