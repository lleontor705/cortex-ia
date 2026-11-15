#!/usr/bin/env bash
# =============================================================================
# cortex-report-hub-reconcile.sh  (Hermes-local, NO versionado en el repo)
#
# Reconciles the sanitized Railway report-hub snapshot against the GitHub
# issue tracker so the cron monitor never re-creates an issue that already
# exists or was already closed — and so each error group is reconciled
# against the repo that OWNS its cause (cortex vs cortex-ia).
#
# Usage:
#   cortex-report-hub-reconcile.sh [LOG_SOURCE_FILE]
#
# Environment Variables:
#   REPORT_HUB_LOG_SOURCE      - JSONL log file (or '-' for stdin); otherwise
#                                the snapshot tool queries Railway directly.
#   REPORT_HUB_SNAPSHOT_TOOL   - snapshot tool path (defaults to the committed
#                                tools/cortex-report-hub-snapshot.sh in the
#                                cortex-ia checkout).
#   REPORT_HUB_SINCE           - Railway window (e.g. 15m, 24h, 30d) used when
#                                neither a log source nor REPORT_HUB_LOG_SOURCE
#                                is given. Default 30d (the monitor's contract
#                                window); pass REPORT_HUB_SINCE=15m explicitly
#                                for a short ad-hoc window.
#   REPORT_HUB_ISSUES_JSON     - Existing issues dump (or '-' for stdin):
#                                  gh issue list --state all --limit 1000 \
#                                    --json number,title,state,url,body,labels
#                                Applied to the PRIMARY repo only (offline mode).
#   REPORT_HUB_REPO            - Primary repo (default: lleontor705/cortex-ia).
#   REPORT_HUB_REPOS           - Comma list of repos whose issues are fetched
#                                for dedupe (default: primary + cortex).
#                                Routing targets are always added automatically.
#   REPORT_HUB_ROUTING         - Comma-separated PREFIX=repo pairs deciding the
#                                target repo per error code (first match wins,
#                                else primary). Default:
#                                  ERR_CORTEX_=lleontor705/cortex
#                                i.e. memory-backend failures (cortex_save,
#                                memory writes) are reported to the `cortex`
#                                project, everything else to the primary repo.
#   REPORT_HUB_COALESCE_CODES  - Comma list of incident codes allowed to
#                                coalesce: when their exact signature is new
#                                but an OPEN issue for the same code class
#                                exists in the target repo, the group is
#                                reclassified to comment_open against that
#                                canonical issue instead of creating a new
#                                one. Default:
#                                  ERR_TASK_BLOCKED,ERR_TOOL_LEASE_REQUIRED,
#                                  ERR_SQLITE_TIMEOUT,ERR_TOOL_TIMEOUT
#
# Output: one stable JSON object on stdout, no timestamps:
#   {"schema":"cortex-report-hub-reconcile/v2","repo":"<primary>","repos":[...],
#    "routing":"...","issues_source":"gh|file|missing",
#    "labels_missing":[...],"labels_missing_by_repo":{...},
#    "totals":{...},"groups":[...]}
#
# Per group: signature, code, severity, action, source, task, count, ids,
# title, labels, target_repo, issue|null, decision.
#
# Decisions (fail closed) — evaluated ONLY against issues of the group's
# target_repo (markers in other repos never suppress a group; that is what
# lets a misrouted signature be re-reported to the correct project):
#   create       - no issue in the TARGET repo carries this signature.
#   comment_open - an OPEN issue in the TARGET repo carries the signature, or
#                  the signature is new (or only carried by CLOSED duplicates)
#                  while an allowlisted code's class issue is OPEN in the
#                  target repo (coalesced recurrence).
#   skip_closed  - a CLOSED issue in the TARGET repo carries the signature and
#                  no OPEN canonical applies (code not allowlisted, or none
#                  open in the target repo).
#   blocked      - issue state could not be read -> do not touch the tracker.
#
# Signature = sha256("<code>|<source>|<task>"), identical to the
# `<!-- cortex-report-hub:<signature> -->` markers already in the repositories.
#
# Pure read/aggregation: NO state writes to disk, DB, GitHub or Railway.
# =============================================================================

set -euo pipefail

SNAPSHOT_TOOL="${REPORT_HUB_SNAPSHOT_TOOL:-/Volumes/Archivos/github_repositories/lleontor705/cortex-ia/tools/cortex-report-hub-snapshot.sh}"
ISSUES_SOURCE="${REPORT_HUB_ISSUES_JSON:-}"
PRIMARY="${REPORT_HUB_REPO:-lleontor705/cortex-ia}"
ROUTING="${REPORT_HUB_ROUTING:-ERR_CORTEX_=lleontor705/cortex}"
REPOS="${REPORT_HUB_REPOS:-$PRIMARY,lleontor705/cortex}"

# Union: primary + explicit repos + every routing target (a target whose issues
# are never fetched could never match a marker -> infinite re-creation).
for _pair in ${ROUTING//,/ }; do
    _target="${_pair#*=}"
    [ -n "$_target" ] || continue
    case ",$REPOS," in
        *",$_target,"*) ;;
        *) REPOS="$REPOS,$_target" ;;
    esac
done
case ",$REPOS," in
    *",$PRIMARY,"*) ;;
    *) REPOS="$PRIMARY,$REPOS" ;;
esac

if [ ! -x "$SNAPSHOT_TOOL" ]; then
    echo "error: snapshot tool not found or not executable: $SNAPSHOT_TOOL" >&2
    exit 1
fi

# --- 1. sanitized snapshot (groups from Railway logs) -----------------------
# Default window = the monitor's contract window (30d). Without this pin a
# bare run falls back to the snapshot tool's 15m default and silently drops
# every report older than 15 minutes (false-negative groups=0).
REPORT_HUB_SINCE="${REPORT_HUB_SINCE:-30d}"
if [ -n "${1:-}" ] || [ -n "${REPORT_HUB_LOG_SOURCE:-}" ]; then
    SNAPSHOT_JSON="$("$SNAPSHOT_TOOL" ${1:+"$1"})"
elif [ -n "${REPORT_HUB_SINCE:-}" ]; then
    SNAPSHOT_JSON="$(railway logs \
        --project 8d1c83da-fcde-4fa9-b4ca-7dd4401bc89f \
        --environment dfcbefa5-4782-4265-90b2-926c7c2b513c \
        --service 5f4cd2a5-791d-4c60-8c61-fd94da2978ef \
        --since "$REPORT_HUB_SINCE" --json | "$SNAPSHOT_TOOL" -)"
else
    SNAPSHOT_JSON="$("$SNAPSHOT_TOOL")"
fi

# --- 2. existing issues per repo (open + closed) ---------------------------
# ISSUES_JSON shape: {"<owner/repo>":[issue,...], ...}
if [ -n "$ISSUES_SOURCE" ]; then
    if [ "$ISSUES_SOURCE" != "-" ] && [ ! -r "$ISSUES_SOURCE" ]; then
        echo "error: issues source not found: $ISSUES_SOURCE" >&2
        exit 1
    fi
    if [ "$ISSUES_SOURCE" = "-" ]; then
        _raw="$(cat)"
    else
        _raw="$(cat "$ISSUES_SOURCE")"
    fi
    # Offline dump is attributed to the primary repo only.
    ISSUES_JSON="$(PRIMARY="$PRIMARY" RAW="$_raw" python3 - <<'PYEOF'
import json, os
raw = os.environ.get("RAW", "") or "[]"
primary = os.environ.get("PRIMARY", "")
try:
    parsed = json.loads(raw)
except Exception:
    parsed = None
if not isinstance(parsed, list):
    parsed = []
print(json.dumps({primary: parsed}, separators=(",", ":")))
PYEOF
)"
    ISSUES_MODE="file"
else
    if ! command -v gh >/dev/null 2>&1; then
        ISSUES_JSON=""
        ISSUES_MODE="missing"
    else
        ISSUES_JSON="{"
        ISSUES_MODE="gh"
        _first=1
        _old_ifs="$IFS"
        IFS=','
        for _repo in $REPOS; do
            [ -n "$_repo" ] || continue
            if _arr="$(gh issue list --repo "$_repo" --state all --limit 1000 \
                    --json number,title,state,url,body,labels 2>/dev/null)"; then
                if [ "$_first" -eq 1 ]; then _first=0; else ISSUES_JSON="$ISSUES_JSON,"; fi
                ISSUES_JSON="$ISSUES_JSON\"$_repo\":$_arr"
            else
                # Fail closed: any unreadable repo blocks every decision.
                ISSUES_JSON=""
                ISSUES_MODE="missing"
                break
            fi
        done
        IFS="$_old_ifs"
        if [ "$ISSUES_MODE" = "gh" ]; then
            ISSUES_JSON="$ISSUES_JSON}"
        fi
    fi
fi

export SNAPSHOT_JSON ISSUES_JSON ISSUES_MODE PRIMARY ROUTING
python3 - <<'PYEOF'
import hashlib
import json
import os
import re
import sys

MARKER_RE = re.compile(r'<!--\s*cortex-report-hub:([0-9a-f]{64})\s*-->')
CLASS_TITLE_RE = re.compile(r'^\[report-hub\] ([^:]+):')
# Verification probes (SMOKE_*, PROBE_*, DEMO_*, MANUAL_*, DEFAULT_*) are never
# incidents: skipping them keeps the monitor from opening issues for its own or
# an operator's tests. Override with REPORT_HUB_PROBE_PREFIXES (comma list).
_PROBE_PREFIXES = [p.strip().upper() for p in
                   os.environ.get("REPORT_HUB_PROBE_PREFIXES",
                                  "SMOKE,PROBE,DEMO,MANUAL,DEFAULT").split(",") if p.strip()]
PROBE_CODE_RE = re.compile(r'^(?:' + "|".join(re.escape(p) for p in _PROBE_PREFIXES) + r')[A-Z0-9_]*$')
# Only routine operational classes may coalesce; a defect code must keep
# minting its own issue so severity regressions stay individually visible.
COALESCE_CODES = {c.strip().upper() for c in os.environ.get(
    "REPORT_HUB_COALESCE_CODES",
    "ERR_TASK_BLOCKED,ERR_TOOL_LEASE_REQUIRED,ERR_SQLITE_TIMEOUT,ERR_TOOL_TIMEOUT",
).split(",") if c.strip()}
SEVERITY_LABEL = {
    "critical": "severity:critical",
    "high": "severity:high",
    "medium": "severity:medium",
    "low": "severity:low",
}
PRIMARY = os.environ.get("PRIMARY", "")
ROUTING_RAW = os.environ.get("ROUTING", "")

# routing rules: (code_prefix, repo) in declaration order, first match wins
ROUTING_RULES = []
for pair in ROUTING_RAW.split(","):
    pair = pair.strip()
    if not pair or "=" not in pair:
        continue
    prefix, repo = pair.split("=", 1)
    prefix, repo = prefix.strip(), repo.strip()
    if prefix and repo:
        ROUTING_RULES.append((prefix, repo))


def target_repo(code):
    for prefix, repo in ROUTING_RULES:
        if code.startswith(prefix):
            return repo
    return PRIMARY


def fail(msg):
    sys.stderr.write(f"error: {msg}\n")
    sys.exit(1)


try:
    groups = json.loads(os.environ["SNAPSHOT_JSON"] or "[]")
except Exception as e:
    fail(f"snapshot is not valid JSON: {e}")
if not isinstance(groups, list):
    fail("snapshot is not a JSON array")

issues_mode = os.environ.get("ISSUES_MODE", "missing")
issues_raw = os.environ.get("ISSUES_JSON", "")
issues_by_repo = None
if issues_mode in ("gh", "file"):
    try:
        parsed = json.loads(issues_raw or "{}")
        if isinstance(parsed, dict):
            # tolerate legacy list dumps (attribution: primary)
            issues_by_repo = {k: v for k, v in parsed.items() if isinstance(v, list)}
        elif isinstance(parsed, list):
            issues_by_repo = {PRIMARY: parsed}
        else:
            issues_by_repo = None
    except Exception:
        issues_by_repo = None
if issues_by_repo is None:
    issues_mode = "missing"
    issues_by_repo = {}

# (repo, signature) -> issue (open wins over closed within the same repo)
by_signature = {}
# (repo, code) -> canonical OPEN issue whose title declares that incident class
by_code_open = {}
existing_labels = {}
for repo, issues in issues_by_repo.items():
    labels_seen = set()
    for issue in issues:
        if not isinstance(issue, dict):
            continue
        body = str(issue.get("body") or "")
        candidate = {
            "number": issue.get("number"),
            "state": str(issue.get("state") or "").upper(),
            "title": issue.get("title"),
            "url": issue.get("url"),
            "repo": repo,
        }
        for sig in MARKER_RE.findall(body):
            key = (repo, sig)
            current = by_signature.get(key)
            if current is None or (current["state"] != "OPEN" and candidate["state"] == "OPEN"):
                by_signature[key] = candidate
        if candidate["state"] == "OPEN":
            title_match = CLASS_TITLE_RE.match(str(issue.get("title") or ""))
            if title_match:
                by_code_open.setdefault((repo, title_match.group(1).strip()), candidate)
        for label in issue.get("labels") or []:
            name = label.get("name") if isinstance(label, dict) else str(label)
            if name:
                labels_seen.add(name)
    existing_labels[repo] = labels_seen

out_groups = []
totals = {
    "groups": 0, "create": 0, "comment_open": 0, "skip_closed": 0, "blocked": 0,
    "critical": 0, "high": 0, "medium": 0, "low": 0, "probe_skipped": 0,
}
labels_missing_by_repo = {}

for group in groups:
    if not isinstance(group, dict):
        continue
    code = str(group.get("code") or "")
    if PROBE_CODE_RE.match(code):
        totals["probe_skipped"] += 1
        continue
    source = str(group.get("source") or "")
    task = str(group.get("task") or "")
    severity = str(group.get("severity") or "low")
    signature = hashlib.sha256(f"{code}|{source}|{task}".encode()).hexdigest()
    t_repo = target_repo(code)
    match = by_signature.get((t_repo, signature))

    if match is None:
        if issues_mode == "missing":
            decision = "blocked"
        else:
            class_issue = by_code_open.get((t_repo, code))
            if class_issue is not None and code.upper() in COALESCE_CODES:
                # One canonical open issue per operational class beats one
                # issue per task signature: without this, every new task
                # re-creates a duplicate of the same recurring incident.
                decision = "comment_open"
                match = class_issue
            else:
                decision = "create"
    elif match["state"] == "OPEN":
        decision = "comment_open" if issues_mode != "missing" else "blocked"
    else:
        # A signature whose only carrier is CLOSED is normally a duplicate that
        # was folded into the class canonical (ERR_TASK_BLOCKED #129-#132 ->
        # #122). Suppressing it here would hide every recurrence of the class
        # behind the closed duplicate, so when the code is coalescable and an
        # OPEN canonical exists in the target repo the group is re-routed to it.
        class_issue = by_code_open.get((t_repo, code))
        if class_issue is not None and code.upper() in COALESCE_CODES:
            decision = "comment_open"
            match = class_issue
        else:
            decision = "skip_closed"

    severity_label = SEVERITY_LABEL.get(severity, SEVERITY_LABEL["low"])
    labels = ["bug", "source:railway-report-hub", severity_label]
    missing = labels_missing_by_repo.setdefault(t_repo, set())
    repo_labels = existing_labels.get(t_repo, set())
    for label in labels:
        if label not in repo_labels:
            missing.add(label)

    subject = task.strip() or source.strip() or "(sin detalle)"
    out_groups.append({
        "signature": signature,
        "code": code,
        "severity": severity,
        "action": str(group.get("action") or ""),
        "source": source,
        "task": task,
        "count": int(group.get("count") or 0),
        "ids": list(group.get("ids") or []),
        "title": f"[report-hub] {code}: {subject}",
        "labels": labels,
        "target_repo": t_repo,
        "issue": match,
        "decision": decision,
    })
    totals["groups"] += 1
    totals[decision] = totals.get(decision, 0) + 1
    if severity in ("critical", "high", "medium", "low"):
        totals[severity] += 1

out_groups.sort(key=lambda g: (g["signature"], g["code"], g["task"]))
all_missing = set()
for repo_missing in labels_missing_by_repo.values():
    all_missing |= repo_missing
result = {
    "schema": "cortex-report-hub-reconcile/v2",
    "repo": PRIMARY,
    "repos": sorted(issues_by_repo.keys()),
    "routing": ROUTING_RAW,
    "issues_source": issues_mode,
    "labels_missing": sorted(all_missing),
    "labels_missing_by_repo": {r: sorted(s) for r, s in sorted(labels_missing_by_repo.items())},
    "totals": totals,
    "groups": out_groups,
}
sys.stdout.write(json.dumps(result, sort_keys=True, separators=(",", ":")) + "\n")
PYEOF
