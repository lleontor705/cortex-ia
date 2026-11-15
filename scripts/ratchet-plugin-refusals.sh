#!/usr/bin/env bash
#
# ratchet-plugin-refusals — pin OpenCode plugin refusal/error strings to a baseline.
#
# Extracts the refusal/error string literals of the native OpenCode plugins under
# internal/assets/plugins: the message of every new Error(...) refusal, the reason
# of every tool-hook denial, and uppercase-named refusal/error constants. Literals
# are whitespace-normalized and any baseline string that was removed or altered is
# reported. Advisory only: it never exits non-zero.
#
# Usage:
#   scripts/ratchet-plugin-refusals.sh                  report drift against the baseline
#   scripts/ratchet-plugin-refusals.sh --regen          emit a candidate baseline + diff
#   scripts/ratchet-plugin-refusals.sh --regen --confirm   replace the baseline
#
# Regen never swaps the committed baseline until --confirm is given, so the
# unified diff is always reviewable before commit.
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LABEL="ratchet-plugin-refusals"
BASELINE_REL=".cortex-ia/ratchet/plugin-refusals-baseline.txt"
BASELINE="${REPO_ROOT}/${BASELINE_REL}"
SOURCE_DIR="internal/assets/plugins"

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
$LABEL — pin OpenCode plugin refusal/error strings to a committed baseline (advisory).

Usage:
  scripts/$LABEL.sh                    report drift against the baseline (exit 0)
  scripts/$LABEL.sh --regen            emit candidate baseline + unified diff
  scripts/$LABEL.sh --regen --confirm  replace the baseline after review

Sources:  $SOURCE_DIR/*.ts
Baseline: $BASELINE_REL
EOF
}

if [ "$MODE" = "help" ]; then
  print_usage
  exit 0
fi

# Emits "repo-relative/path.ts<TAB>literal-with-delimiters" for every refusal.
# new Error(...) literals may wrap onto the next line, so each file is flattened
# with newlines collapsed to spaces before the grep -o pass; tool-hook denials and
# uppercase-named constants are single-line and matched in place.
extract_candidate() {
  local out="$1" f
  : > "$out"
  (
    cd "$REPO_ROOT" || exit 0
    for f in "$SOURCE_DIR"/*.ts; do
      [ -f "$f" ] || continue
      tr '\n' ' ' < "$f" |
        grep -oE 'new Error\([[:space:]]*("([^"\\]|\\.)*"|`[^`]*`)' |
        awk -v p="$f" '{ sub(/^new Error\([[:space:]]*/, ""); print p "\t" $0 }'
    done
  ) >> "$out"

  (
    cd "$REPO_ROOT" || exit 0
    for f in "$SOURCE_DIR"/*.ts; do
      [ -f "$f" ] || continue
      grep -HnE 'effect: *"deny"|reason: *[`"]|const [A-Z][A-Z0-9_]* = "' "$f"
    done
  ) | awk '
    function raw(s, from,   i, n, ch, q, start) {
      n = length(s)
      start = 0
      q = ""
      for (i = from; i <= n; i++) {
        ch = substr(s, i, 1)
        if (ch == "\"" || ch == "`") { q = ch; start = i; break }
      }
      if (start == 0) return ""
      for (i = start + 1; i <= n; i++) {
        if (substr(s, i, 1) == q) return substr(s, start, i - start + 1)
      }
      return ""
    }
    {
      p = index($0, ":")
      if (p == 0) next
      path = substr($0, 1, p - 1)
      rest = substr($0, p + 1)
      c = index(rest, ":")
      if (c == 0) next
      content = substr(rest, c + 1)

      lit = ""
      if (content ~ /effect: *"deny"/) { pending[path] = 1; next }
      if (content ~ /new Error\(/) {
        match(content, /new Error\(/)
        lit = raw(content, RSTART + RLENGTH)
      } else if (content ~ /const [A-Z][A-Z0-9_]* = "/) {
        match(content, /=/)
        lit = raw(content, RSTART + RLENGTH)
      } else if (content ~ /reason:/) {
        if (!(path in pending)) next
        delete pending[path]
        match(content, /reason:/)
        lit = raw(content, RSTART + RLENGTH)
      } else next
      if (lit == "") next
      printf "%s\t%s\n", path, lit
    }
  ' >> "$out"

  awk -F'\t' '
    NF >= 2 {
      msg = substr($0, index($0, "\t") + 1)
      first = substr(msg, 1, 1)
      if (first == "\"" || first == "`") {
        if (substr(msg, length(msg), 1) == first) msg = substr(msg, 2, length(msg) - 2)
      }
      gsub(/[[:space:]]+/, " ", msg)
      sub(/^ /, "", msg)
      sub(/ $/, "", msg)
      if (msg == "") next
      printf "%s\t%s\n", $1, msg
    }
  ' "$out" | LC_ALL=C sort -u > "${out}.norm"
  mv "${out}.norm" "$out"
}

write_candidate_file() {
  local candidate="$1" composed="$2"
  {
    printf '# %s baseline — normalized plugin refusal/error strings.\n' "$LABEL"
    printf '# Sources: %s/*.ts.\n' "$SOURCE_DIR"
    printf '# Regenerate: scripts/%s.sh --regen --confirm\n' "$LABEL"
    printf '# Format: <repo-relative plugin file>\\t<normalized message>\n'
    cat "$candidate"
  } > "$composed"
}

run_check() {
  local candidate report
  candidate="$(mktemp)"
  report="$(mktemp)"
  extract_candidate "$candidate"

  if [ ! -f "$BASELINE" ]; then
    printf '[%s] no baseline at %s — run: scripts/%s.sh --regen --confirm\n' \
      "$LABEL" "$BASELINE_REL" "$LABEL"
    rm -f "$candidate" "$report"
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

  rm -f "$candidate" "$report"
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
