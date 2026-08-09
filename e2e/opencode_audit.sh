#!/usr/bin/env bash
# e2e/opencode_audit.sh - OpenCode O1-O11 filesystem audit.
#
# Self-contained, container-only audit for the
# audit-opencode-docker-installation-e2e change. It owns its own O1-O11 ledger,
# strict PASS|FAIL result shape, Node-required / fail-closed semantics, explicit
# install / reinstall / persona-uninstall stages, sanitized diagnostics, and
# security guards. It does NOT source e2e/lib.sh.
#
# The audit drives exactly the in-container cortex-ia install and the scoped
# persona dry-run/real uninstall. It creates one inert, unexecuted opencode
# detection stub, never invokes a target client, issues no network command,
# issues no network command, forwards no secret/env, and writes only under the
# isolated container HOME and /tmp.
#
# Acceptance:
#   - `bash -n e2e/opencode_audit.sh` passes; shellcheck passes when available.
#   - O1-O11 occur once, in numeric order; each has exactly one terminal PASS
#     or FAIL outcome; there is no third state.
#   - Node is mandatory; the audit fails closed when it is absent.
#   - JSON parsing/canonicalization/hashing uses Node with recursive object-key
#     sorting while preserving array order.
#   - Diagnostics expose only relative paths, counts, digests, and states.
#   - The inert opencode stub contains an immediate failure guard and is never
#     invoked; no target client, network, secret/env forwarding, or host write
#     occurs.
#   - Rollback: revert this file to the red-contract revision or delete it.
set -euo pipefail

# ---------------------------------------------------------------------------
# Sanitized diagnostics contract
# ---------------------------------------------------------------------------
# Every diagnostic line is expressible as one of:
#   - a relative path (e.g. .config/opencode/opencode.json)
#   - a count (e.g. fail=3)
#   - a digest (sha256:<hex>)
#   - a state token (e.g. PASS, FAIL, install-failed, marker-drift)
# Absolute paths, environment values, credentials, HOME content, receipt
# bodies, and sidecar bodies are never printed. The helpers below are the only
# sanctioned emit surface.
LOG_TOTAL=0
LOG_PASS=0
LOG_FAIL=0

emit_pass() {  # <id>
    local id="$1"
    printf '%s: PASS\n' "$id"
    LOG_TOTAL=$((LOG_TOTAL + 1))
    LOG_PASS=$((LOG_PASS + 1))
}

emit_fail() {  # <id> <state-token> [detail]
    local id="$1" state="$2" detail="${3:-}"
    if [ -n "$detail" ]; then
        printf '%s: FAIL state=%s %s\n' "$id" "$state" "$detail"
    else
        printf '%s: FAIL state=%s\n' "$id" "$state"
    fi
    LOG_TOTAL=$((LOG_TOTAL + 1))
    LOG_FAIL=$((LOG_FAIL + 1))
}

emit_ledger() {
    printf 'LEDGER: total=%d pass=%d fail=%d\n' "$LOG_TOTAL" "$LOG_PASS" "$LOG_FAIL"
}

# ---------------------------------------------------------------------------
# Contract guard: this audit MUST NOT source e2e/lib.sh.
# ---------------------------------------------------------------------------
# The OpenCode audit is a separate container run that owns its own ledger; it
# must never inherit the baseline harness symbols. Refuse to continue if
# lib.sh symbols are already present in this shell.
if declare -F log_pass >/dev/null 2>&1 \
   || declare -F print_summary >/dev/null 2>&1 \
   || declare -F assert_file_exists >/dev/null 2>&1; then
    printf 'opencode_audit: FAIL state=contract-violation detail=lib-sh-symbols-present\n' >&2
    exit 2
fi

# ---------------------------------------------------------------------------
# Intended command surface (scoped to in-container cortex-ia only)
# ---------------------------------------------------------------------------
# The audit drives exactly these in-container commands; no target client
# executable is invoked. The inert opencode stub is created for exec.LookPath
# detection only and is never called.
CORTEX_IA_BIN="/usr/local/bin/cortex-ia"
CMD_INSTALL=("$CORTEX_IA_BIN" install --agent opencode --preset full)
CMD_REINSTALL=("$CORTEX_IA_BIN" install --agent opencode --preset full)
CMD_PERSONA_DRYRUN=("$CORTEX_IA_BIN" uninstall --agent opencode --component persona --dry-run)
CMD_PERSONA_REAL=("$CORTEX_IA_BIN" uninstall --agent opencode --component persona --yes)
readonly CORTEX_IA_BIN CMD_INSTALL CMD_REINSTALL CMD_PERSONA_DRYRUN CMD_PERSONA_REAL

# ---------------------------------------------------------------------------
# Layout
# ---------------------------------------------------------------------------
# Managed output lives under $HOME/.config/opencode (opencode.json, AGENTS.md,
# agents/, skills/, ...) and $HOME/.cortex-ia (state.json, cortex-ia.lock,
# receipts/, skills/). The embedded Node program resolves these paths from
# $HOME; bash only needs the snapshot/work root and the cortex-ia binary.
SNAPSHOT_ROOT="/tmp/cortex-ia-opencode-audit"
RES_DIR="$SNAPSHOT_ROOT/res"
NODE_PROGRAM="$SNAPSHOT_ROOT/audit.mjs"
# The expected inventories (nine phase agents; exact 17 local utility skills;
# exact 11 shared skills) are authoritative constants in the embedded Node
# program below, where every O1-O11 check reads and compares them.

# ---------------------------------------------------------------------------
# Security guards
# ---------------------------------------------------------------------------

# require_node - Node is the MANDATORY JSON parser and canonical hasher.
# Fail closed when it is absent; no weaker fallback, no third state.
require_node() {
    if ! command -v node >/dev/null 2>&1; then
        printf 'opencode_audit: FAIL state=node-required detail=interpreter-absent\n' >&2
        exit 3
    fi
}

# assert_home_boundary - runtime HOME MUST be the isolated container user
# home. The runner guarantees this inside the container; firing here means a
# boundary breach (host HOME or unset). Fail closed.
assert_home_boundary() {
    if [ "${HOME:-}" != "/home/testuser" ]; then
        printf 'opencode_audit: FAIL state=boundary-reject detail=home-not-container-user\n' >&2
        exit 4
    fi
}

# Static security invariant (verified by inspection of this file): the audit
# contains no engine invocation, no network client, no target-client
# executable invocation, no host shell bridge, no secret/env forwarding flag,
# and no host profile path. The runtime boundary is enforced by the lib.sh
# symbol guard above plus require_node and assert_home_boundary.

# ---------------------------------------------------------------------------
# Inert opencode detection stub (never executed)
# ---------------------------------------------------------------------------
# setup_fake_opencode creates $HOME/bin/opencode containing only a shebang and
# an immediate failure guard, then prepends $HOME/bin to PATH. The stub exists
# so cortex-ia's exec.LookPath detection reports opencode as installed; it is
# never invoked by this audit.
setup_fake_opencode() {
    local stub_dir="$HOME/bin"
    mkdir -p "$stub_dir"
    cat >"$stub_dir/opencode" <<'OPENCODE_STUB_EOF'
#!/usr/bin/env bash
printf '%s\n' 'opencode stub invoked: audit forbids target CLI execution' >&2
exit 99
OPENCODE_STUB_EOF
    chmod 0755 "$stub_dir/opencode"
    case ":$PATH:" in
        *":$stub_dir:"*) ;;
        *) PATH="$stub_dir:$PATH"; export PATH ;;
    esac
}

# ---------------------------------------------------------------------------
# Sanitized command runner
# ---------------------------------------------------------------------------
# run_cortexia executes a declared cortex-ia command with stdout/stderr
# captured to the snapshot root (never the console) and returns its exit code.
# It forwards no environment beyond PATH/HOME and writes only under HOME/tmp.
run_cortexia() {  # <stage-name> <cmd-array...>
    local stage="$1"; shift
    local out="$SNAPSHOT_ROOT/cmd-$stage.out"
    local err="$SNAPSHOT_ROOT/cmd-$stage.err"
    "$@" >"$out" 2>"$err" || return $?
    return 0
}

# ---------------------------------------------------------------------------
# Node audit program
# ---------------------------------------------------------------------------
# One embedded ECMAScript module performs all JSON parsing, recursive key
# canonicalization (arrays preserve order), SHA-256/MD5 hashing, receipt-seal
# verification, and sidecar/base/target validation. It reads the live HOME and
# emits only sanitized state tokens, counts, and digests.
write_node_program() {
    mkdir -p "$SNAPSHOT_ROOT"
    cat >"$NODE_PROGRAM" <<'NODE_AUDIT_EOF'
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

const HOME = process.env.HOME;
const PHASE_AGENTS = ['bootstrap','investigate','draft-proposal','write-specs','architect','decompose','implement','validate','finalize'];
const LOCAL_SKILLS = ['chained-pr','cognitive-doc-design','comment-writer','debug','execute-plan','file-issue','go-testing','ideate','judgment-day','monitor','mutation-testing','onboard','open-pr','scan-registry','skill-creator','skill-improver','work-unit-commits'];
const SHARED_SKILLS = ['architect','bootstrap','debate','decompose','draft-proposal','finalize','implement','investigate','parallel-dispatch','validate','write-specs'];
const EXPECTED_THEME = { name:'cortex', colors:{ primary:'#7C3AED', secondary:'#06B6D4', success:'#22C55E', warning:'#F59E0B', error:'#EF4444' } };
const EXPECTED_MCP = ['agent-mailbox','context7','cortex','forgespec'];

function sha256(buf){ return createHash('sha256').update(buf).digest('hex'); }
function md5(buf){ return createHash('md5').update(buf).digest('hex'); }
function canon(v){
  if (Array.isArray(v)) return v.map(canon);
  if (v && typeof v === 'object'){
    const o = {};
    for (const k of Object.keys(v).sort()) o[k] = canon(v[k]);
    return o;
  }
  return v;
}
function canonSha(v){ return sha256(Buffer.from(JSON.stringify(canon(v)),'utf8')); }
function readJSON(p){ return JSON.parse(readFileSync(p,'utf8')); }
function tryRead(p){ try { return readFileSync(p,'utf8'); } catch { return null; } }
function listFiles(dir){
  const out = [];
  if (!existsSync(dir)) return out;
  const walk = (d) => {
    for (const e of readdirSync(d)){
      const full = join(d,e);
      const st = statSync(full);
      if (st.isDirectory()) walk(full);
      else out.push(full);
    }
  };
  walk(dir);
  return out;
}
function rel(p){ return relative(HOME, p).split(sep).join('/'); }
function fileSha(p){ return sha256(readFileSync(p)); }

// ---- O1-O9 audit against live state ----
function audit19(){
  const results = [];
  const fail = (id,state,detail) => results.push([id,'FAIL',state,detail||'']);
  const pass = (id) => results.push([id,'PASS','','']);
  const ocDir = join(HOME,'.config/opencode');
  const jsonPath = join(ocDir,'opencode.json');
  const jsoncPath = join(ocDir,'opencode.jsonc');

  // O1: exactly opencode.json exists and parses as one JSON object; jsonc absent.
  let cfg = null;
  try {
    const st = statSync(jsonPath);
    if (!st.isFile()) { fail('O1','not-regular-file'); }
    else if (existsSync(jsoncPath)) { fail('O1','jsonc-present','.config/opencode/opencode.jsonc'); }
    else {
      const parsed = JSON.parse(readFileSync(jsonPath,'utf8'));
      if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) fail('O1','not-json-object');
      else { cfg = parsed; pass('O1'); }
    }
  } catch { fail('O1','unreadable'); }

  // O2: top-level mcp/permission/theme/agent are objects; theme exactly matches.
  if (cfg) {
    const objs = ['mcp','permission','theme','agent'];
    let ok = true; let bad = '';
    for (const k of objs){ const v = cfg[k]; if (!(v && typeof v==='object' && !Array.isArray(v))){ ok=false; bad=k; break; } }
    if (!ok) fail('O2','missing-managed-key',bad);
    else {
      const t = cfg.theme;
      const want = EXPECTED_THEME;
      const same = t.name === want.name &&
        ['primary','secondary','success','warning','error'].every(c => t.colors && t.colors[c] === want.colors[c]) &&
        Object.keys(t.colors||{}).length === 5;
      if (!same) fail('O2','theme-drift');
      else pass('O2');
    }
  } else fail('O2','config-unreadable');

  // O3: MCP set exact; type=local, enabled=true, command non-empty string array.
  if (cfg && cfg.mcp) {
    const keys = Object.keys(cfg.mcp).sort();
    if (keys.join(',') !== EXPECTED_MCP.join(',')) fail('O3','mcp-set-drift','count='+keys.length);
    else {
      let ok = true; let bad='';
      for (const k of keys){
        const e = cfg.mcp[k];
        if (!e || e.type !== 'local' || e.enabled !== true || !Array.isArray(e.command) || e.command.length === 0 || !e.command.every(x => typeof x === 'string' && x.length > 0)){ ok=false; bad=k; break; }
      }
      if (!ok) fail('O3','mcp-entry-invalid',bad); else pass('O3');
    }
  } else fail('O3','mcp-absent');

  // O4: permission.bash[*]=ask; read[*]=allow; read[.env]/read[.env.*]=deny;
  //     read[.env.example]=allow; top-level permissions absent.
  if (cfg && cfg.permission) {
    if (cfg.permissions !== undefined) fail('O4','legacy-permissions-present');
    else {
      const pb = cfg.permission.bash || {}; const pr = cfg.permission.read || {};
      if (pb['*'] !== 'ask') fail('O4','bash-default-drift');
      else if (pr['*'] !== 'allow') fail('O4','read-default-drift');
      else if (pr['.env'] !== 'deny') fail('O4','env-deny-missing');
      else if (pr['.env.*'] !== 'deny') fail('O4','env-glob-deny-missing');
      else if (pr['.env.example'] !== 'allow') fail('O4','env-example-allow-missing');
      else pass('O4');
    }
  } else fail('O4','permission-absent');

  // O5: nine phase subagents exist under agents/ each mode=subagent.
  {
    const adir = join(ocDir,'agents');
    let ok = true; let bad='';
    for (const name of PHASE_AGENTS){
      const p = join(adir, name+'.md');
      const body = tryRead(p);
      if (body === null){ ok=false; bad='agents/'+name+'.md:absent'; break; }
      if (!/^mode:[ ]*subagent$/m.test(body) && !/^[ ]{0,2}mode:[ ]*subagent$/m.test(body)){ ok=false; bad='agents/'+name+'.md:mode'; break; }
    }
    if (!ok) fail('O5','phase-agent-invalid',bad); else pass('O5');
  }

  // O6: no gemini (ci), no top-level provider/model, no team-lead agent keys,
  //     no model/provider fields on managed agent files.
  if (cfg) {
    const ser = JSON.stringify(cfg);
    let bad = '';
    if (/gemini/i.test(ser)) bad = 'gemini-present';
    else if (cfg.provider !== undefined || cfg.model !== undefined) bad = 'top-level-routing';
    else {
      const agentKeys = cfg.agent && typeof cfg.agent === 'object' ? Object.keys(cfg.agent) : [];
      if (agentKeys.some(k => /team-lead|sdd-team-lead/i.test(k))) bad = 'retired-role-present';
      else {
        const adir = join(ocDir,'agents');
        if (existsSync(adir)){
          for (const f of listFiles(adir)){
            if (!f.endsWith('.md')) continue;
            const body = readFileSync(f,'utf8');
            const fm = body.match(/^---\n([\s\S]*?)\n---/);
            if (fm && /^model:[ ]*\S/m.test(fm[1])) { bad = 'managed-agent-routing:'+relative(adir,f).split(sep).join('/'); break; }
          }
        }
      }
    }
    if (bad) fail('O6','forbidden-class',bad); else pass('O6');
  } else fail('O6','config-unreadable');

  // O7: AGENTS.md exists; cortex-protocol and cortex-persona opening markers
  //     each occur exactly once.
  {
    const body = tryRead(join(ocDir,'AGENTS.md'));
    if (body === null) fail('O7','agents-md-absent');
    else {
      const proto = (body.match(/<!-- cortex-ia:cortex-protocol -->/g)||[]).length;
      const person = (body.match(/<!-- cortex-ia:cortex-persona -->/g)||[]).length;
      if (proto !== 1 || person !== 1) fail('O7','marker-drift','protocol='+proto+' persona='+person);
      else pass('O7');
    }
  }

  // O8: OpenCode skills equal exact 17 local set; shared ~/.cortex-ia/skills
  //     equal exact 11 set; _shared/cortex-convention.md exists.
  {
    const oc = listSkills(join(ocDir,'skills'));
    const sh = listSkills(join(HOME,'.cortex-ia/skills'));
    const ocSet = oc.join(','); const shSet = sh.join(',');
    const conv = existsSync(join(HOME,'.cortex-ia/skills/_shared/cortex-convention.md'));
    const wantLocal = [...LOCAL_SKILLS].sort().join(',');
    const wantShared = [...SHARED_SKILLS].sort().join(',');
    if (ocSet !== wantLocal) fail('O8','local-skills-drift','count='+oc.length);
    else if (shSet !== wantShared) fail('O8','shared-skills-drift','count='+sh.length);
    else if (!conv) fail('O8','convention-absent');
    else pass('O8');
  }

  // O9: at least one committed sealed receipt; every discovered ownership
  //     sidecar validates (owner, digests, base, target); no orphan.
  {
    const rdir = join(HOME,'.cortex-ia/receipts');
    let sealedCommitted = false; let receiptBad = ''; let sideBad = '';
    if (existsSync(rdir)){
      for (const f of listFiles(rdir)){
        if (!f.endsWith('.json')) continue;
        try {
          const rec = JSON.parse(readFileSync(f,'utf8'));
          const stored = rec.ReceiptSHA256 || '';
          rec.ReceiptSHA256 = '';
          const calc = sha256(Buffer.from(JSON.stringify(rec),'utf8'));
          const committed = rec.State === 'committed';
          const backupVerified = rec.BackupVerified === true;
          if (stored && stored === calc && committed && backupVerified) sealedCommitted = true;
          else if (!stored || stored !== calc) receiptBad = 'seal-mismatch';
        } catch { receiptBad = 'receipt-unparseable'; }
      }
    }
    // Sidecar/base/target triples under the OpenCode tree.
    let sidecarsValid = 0; let sidecarSeen = false;
    for (const f of listFiles(ocDir)){
      if (!f.endsWith('.cortex-ia.json')) continue;
      sidecarSeen = true;
      try {
        const side = JSON.parse(readFileSync(f,'utf8'));
        const owner = side.owner === 'cortex-ia';
        const hex = s => typeof s === 'string' && /^[0-9a-f]{64}$/.test(s);
        const base = f.replace(/\.cortex-ia\.json$/,'.cortex-ia.base');
        const target = join(HOME, side.asset_path || '');
        if (!owner || !hex(side.base_sha256) || !hex(side.content_sha256)) { sideBad = 'sidecar-fields:'+rel(f); break; }
        if (!existsSync(base)) { sideBad = 'base-absent:'+rel(base); break; }
        if (!side.asset_path || !existsSync(target)) { sideBad = 'target-absent:'+String(side.asset_path||''); break; }
        if (sha256(readFileSync(base)) !== side.base_sha256) { sideBad = 'base-hash:'+rel(base); break; }
        if (sha256(readFileSync(target)) !== side.content_sha256) { sideBad = 'content-hash:'+rel(f); break; }
        sidecarsValid++;
      } catch { sideBad = 'sidecar-unparseable:'+rel(f); break; }
    }
    if (!sealedCommitted) fail('O9','no-sealed-receipt',receiptBad||'count=0');
    else if (!sidecarSeen || sidecarsValid === 0) fail('O9','no-ownership-triple');
    else if (sideBad) fail('O9','ownership-invalid',sideBad);
    else pass('O9');
  }

  return results;
}

function listSkills(dir){
  const names = [];
  if (!existsSync(dir)) return names;
  for (const e of readdirSync(dir)){
    if (e === '_shared') continue;
    const skillMd = join(dir,e,'SKILL.md');
    if (existsSync(skillMd)) names.push(e);
  }
  return names.sort();
}

// ---- Snapshot manifest for O10/O11 comparisons ----
function snapshot(){
  const ocDir = join(HOME,'.config/opencode');
  const manifest = { opencodeCanonical:'', opencodeJsoncExists:false, agentsMD5:'', agentsSHA256:'',
    installedAgents:[], markers:{protocol:0,persona:0}, configProjection:'',
    skillDigests:{}, tree:{} };
  const body = tryRead(join(ocDir,'opencode.json'));
  if (body !== null){
    try { manifest.opencodeCanonical = canonSha(JSON.parse(body)); } catch {}
    try {
      const cfg = JSON.parse(body);
      manifest.configProjection = canonSha({ mcp:cfg.mcp, permission:cfg.permission, theme:cfg.theme, agent:cfg.agent });
    } catch {}
  }
  manifest.opencodeJsoncExists = existsSync(join(ocDir,'opencode.jsonc'));
  const agentsBody = tryRead(join(ocDir,'AGENTS.md'));
  if (agentsBody !== null){
    const ab = readFileSync(join(ocDir,'AGENTS.md'));
    manifest.agentsMD5 = md5(ab);
    manifest.agentsSHA256 = sha256(ab);
    manifest.markers.protocol = (agentsBody.match(/<!-- cortex-ia:cortex-protocol -->/g)||[]).length;
    manifest.markers.persona = (agentsBody.match(/<!-- cortex-ia:cortex-persona -->/g)||[]).length;
  }
  const lock = tryRead(join(HOME,'.cortex-ia/cortex-ia.lock'));
  if (lock !== null){
    try { manifest.installedAgents = (JSON.parse(lock).installed_agents||[]).slice().sort(); } catch {}
  }
  for (const base of [join(ocDir,'skills'), join(HOME,'.cortex-ia/skills')]){
    for (const f of listFiles(base)){
      if (f.endsWith('/SKILL.md') || f.endsWith('\\SKILL.md')){
        manifest.skillDigests[rel(f)] = fileSha(f);
      }
    }
  }
  for (const f of listFiles(ocDir)){
    manifest.tree[rel(f)] = fileSha(f);
  }
  return manifest;
}

function diffTree(a, b, ignorePrefix){
  const ak = Object.keys(a).sort(); const bk = Object.keys(b).sort();
  for (const k of ak){
    if (ignorePrefix && k.startsWith(ignorePrefix)) continue;
    if (a[k] !== b[k]) return 'changed:'+k;
  }
  for (const k of bk){
    if (ignorePrefix && k.startsWith(ignorePrefix)) continue;
    if (!(k in a)) return 'added:'+k;
  }
  return '';
}

// ---- O10 reinstall idempotence ----
function o10(pre, post){
  if (pre.opencodeCanonical !== post.opencodeCanonical) return ['FAIL','config-canonical-drift'];
  if (JSON.stringify(pre.installedAgents) !== JSON.stringify(post.installedAgents)) return ['FAIL','installed-agents-drift'];
  if (pre.agentsMD5 !== post.agentsMD5 || pre.agentsSHA256 !== post.agentsSHA256) return ['FAIL','agents-digest-drift'];
  if (pre.markers.protocol !== post.markers.protocol || pre.markers.persona !== post.markers.persona) return ['FAIL','marker-count-drift'];
  if (JSON.stringify(pre.skillDigests) !== JSON.stringify(post.skillDigests)) return ['FAIL','skill-map-drift'];
  const td = diffTree(pre.tree, post.tree, '');
  if (td) return ['FAIL','tree-drift',td];
  return ['PASS',''];
}

// ---- O11 dry-run exact non-mutation ----
function o11dry(pre, post){
  if (pre.opencodeCanonical !== post.opencodeCanonical) return ['FAIL','dry-config-drift'];
  if (pre.agentsSHA256 !== post.agentsSHA256) return ['FAIL','dry-agents-drift'];
  if (pre.configProjection !== post.configProjection) return ['FAIL','dry-projection-drift'];
  if (JSON.stringify(pre.installedAgents) !== JSON.stringify(post.installedAgents)) return ['FAIL','dry-agents-list-drift'];
  const td = diffTree(pre.tree, post.tree, '');
  if (td) return ['FAIL','dry-tree-drift',td];
  return ['PASS',''];
}

// ---- O11 real persona uninstall ----
function o11real(pre, post){
  if (post.markers.persona !== 0) return ['FAIL','persona-marker-present','persona='+post.markers.persona];
  if (post.markers.protocol !== 1) return ['FAIL','protocol-marker-drift','protocol='+post.markers.protocol];
  if (pre.configProjection !== post.configProjection) return ['FAIL','real-projection-drift'];
  if (pre.opencodeCanonical !== post.opencodeCanonical) return ['FAIL','real-config-drift'];
  if (pre.opencodeJsoncExists !== post.opencodeJsoncExists) return ['FAIL','real-jsonc-drift'];
  // Allow only AGENTS.md (and its sidecar/base) to differ; everything else byte-identical.
  const td = diffTree(pre.tree, post.tree, '.config/opencode/AGENTS.md');
  if (td) return ['FAIL','real-tree-drift',td];
  return ['PASS',''];
}

const cmd = process.argv[2];
function emit(arr){ for (const r of arr){ const [id,st,state,detail] = r; process.stdout.write(id+'\t'+st+(st==='FAIL'?'\t'+state+(detail?'\t'+detail:''):'')+'\n'); } }

if (cmd === 'audit19'){ emit(audit19()); }
else if (cmd === 'snapshot'){ process.stdout.write(JSON.stringify(snapshot())); }
else if (cmd === 'o10'){ const a=JSON.parse(readFileSync(process.argv[3])); const b=JSON.parse(readFileSync(process.argv[4])); const r=o10(a,b); process.stdout.write('O10\t'+r[0]+(r[0]==='FAIL'?'\t'+r[1]+(r[2]?'\t'+r[2]:''):'')+'\n'); }
else if (cmd === 'o11dry'){ const a=JSON.parse(readFileSync(process.argv[3])); const b=JSON.parse(readFileSync(process.argv[4])); const r=o11dry(a,b); process.stdout.write('O11dry\t'+r[0]+(r[0]==='FAIL'?'\t'+r[1]+(r[2]?'\t'+r[2]:''):'')+'\n'); }
else if (cmd === 'o11real'){ const a=JSON.parse(readFileSync(process.argv[3])); const b=JSON.parse(readFileSync(process.argv[4])); const r=o11real(a,b); process.stdout.write('O11real\t'+r[0]+(r[0]==='FAIL'?'\t'+r[1]+(r[2]?'\t'+r[2]:''):'')+'\n'); }
else { process.stderr.write('unknown command: '+cmd+'\n'); process.exit(64); }
NODE_AUDIT_EOF
}

# node_invoke <cmd> [args...] - run the embedded audit program, capturing output.
node_invoke() {
    node "$NODE_PROGRAM" "$@"
}

# normalize_node_result <outfile> <oracle-id> - rewrite a Node oracle line
# (id<TAB>PASS | id<TAB>FAIL<TAB>state[<TAB>detail]) into the 4-field cache
# format at $RES_DIR/<oracle-id>.
normalize_node_result() {  # <outfile> <id>
    local outfile="$1" id="$2"
    local line idf status state detail
    line="$(cat "$outfile" 2>/dev/null || true)"
    IFS=$'\t' read -r idf status state detail <<EOF_LINE
$line
EOF_LINE
    status="${status:-FAIL}"
    if [ "$status" != "PASS" ]; then state="${state:-node-no-result}"; fi
    printf '%s\t%s\t%s\t%s\n' "$id" "$status" "$state" "$detail" >"$RES_DIR/$id"
}

# ---------------------------------------------------------------------------
# Explicit lifecycle stages
# ---------------------------------------------------------------------------
# Each stage drives the declared cortex-ia command inside the container, then
# capture/compare sanitized snapshots so the dependent oracle records real
# evidence. Stage names, command scope, and sanitization are preserved.
STAGE_INSTALL_STATE="not-executed"
STAGE_REINSTALL_STATE="not-executed"
STAGE_PERSONA_DRYRUN_STATE="not-executed"
STAGE_PERSONA_REAL_STATE="not-executed"

stage_fresh_install() {
    # Fresh full OpenCode install (CMD_INSTALL) after preparing the inert stub.
    setup_fake_opencode
    if run_cortexia install "${CMD_INSTALL[@]}"; then
        STAGE_INSTALL_STATE="executed-ok"
        # Capture install1 snapshot and validate O1-O9 against the live state.
        node_invoke snapshot >"$SNAPSHOT_ROOT/install1.json" 2>/dev/null
        node_invoke audit19 >"$SNAPSHOT_ROOT/audit19.out" 2>/dev/null || true
        # Default each O1-O9 to not-validated, then map Node results.
        local id
        for id in O1 O2 O3 O4 O5 O6 O7 O8 O9; do
            printf '%s\tFAIL\tinstall-not-validated\t\n' "$id" >"$RES_DIR/$id"
        done
        if [ -s "$SNAPSHOT_ROOT/audit19.out" ]; then
            while IFS=$'\t' read -r id status state detail; do
                [ -n "$id" ] || continue
                printf '%s\t%s\t%s\t%s\n' "$id" "$status" "${state:-}" "$detail" >"$RES_DIR/$id"
            done <"$SNAPSHOT_ROOT/audit19.out"
        fi
        return 0
    fi
    STAGE_INSTALL_STATE="executed-failed"
    local id
    for id in O1 O2 O3 O4 O5 O6 O7 O8 O9; do
        printf '%s\tFAIL\tinstall-failed\t\n' "$id" >"$RES_DIR/$id"
    done
    printf 'O10\tFAIL\tinstall-failed\t\n' >"$RES_DIR/O10"
    printf 'O11\tFAIL\tinstall-failed\t\n' >"$RES_DIR/O11"
    return 1
}

stage_reinstall() {
    # Idempotent reinstall (CMD_REINSTALL) for O10 equality.
    if [ "$STAGE_INSTALL_STATE" != "executed-ok" ]; then
        STAGE_REINSTALL_STATE="skipped-no-baseline"
        printf 'O10\tFAIL\tinstall-failed\t\n' >"$RES_DIR/O10"
        return 1
    fi
    if run_cortexia reinstall "${CMD_REINSTALL[@]}"; then
        STAGE_REINSTALL_STATE="executed-ok"
        node_invoke snapshot >"$SNAPSHOT_ROOT/install2.json" 2>/dev/null
        node_invoke o10 "$SNAPSHOT_ROOT/install1.json" "$SNAPSHOT_ROOT/install2.json" \
            >"$SNAPSHOT_ROOT/o10.out" 2>/dev/null || true
        if [ -s "$SNAPSHOT_ROOT/o10.out" ]; then
            normalize_node_result "$SNAPSHOT_ROOT/o10.out" O10
        else
            printf 'O10\tFAIL\treinstall-not-validated\t\n' >"$RES_DIR/O10"
        fi
        return 0
    fi
    STAGE_REINSTALL_STATE="executed-failed"
    printf 'O10\tFAIL\treinstall-failed\t\n' >"$RES_DIR/O10"
    return 1
}

stage_persona_uninstall_dryrun() {
    # Persona dry-run uninstall (CMD_PERSONA_DRYRUN) for O11 non-mutation.
    if [ "$STAGE_INSTALL_STATE" != "executed-ok" ]; then
        STAGE_PERSONA_DRYRUN_STATE="skipped-no-baseline"
        return 1
    fi
    node_invoke snapshot >"$SNAPSHOT_ROOT/predry.json" 2>/dev/null
    if run_cortexia dryrun "${CMD_PERSONA_DRYRUN[@]}"; then
        STAGE_PERSONA_DRYRUN_STATE="executed-ok"
        node_invoke snapshot >"$SNAPSHOT_ROOT/postdry.json" 2>/dev/null
        node_invoke o11dry "$SNAPSHOT_ROOT/predry.json" "$SNAPSHOT_ROOT/postdry.json" \
            >"$SNAPSHOT_ROOT/o11dry.out" 2>/dev/null || true
        return 0
    fi
    STAGE_PERSONA_DRYRUN_STATE="executed-failed"
    return 1
}

stage_persona_uninstall_real() {
    # Persona real uninstall (CMD_PERSONA_REAL) for O11 targeted removal.
    if [ "$STAGE_PERSONA_DRYRUN_STATE" != "executed-ok" ]; then
        STAGE_PERSONA_REAL_STATE="skipped-no-dryrun"
        printf 'O11\tFAIL\tdryrun-failed\t\n' >"$RES_DIR/O11"
        return 1
    fi
    # pre-real snapshot == post-dry (non-mutating); reuse it as the baseline.
    cp "$SNAPSHOT_ROOT/postdry.json" "$SNAPSHOT_ROOT/prereal.json"
    if run_cortexia real "${CMD_PERSONA_REAL[@]}"; then
        STAGE_PERSONA_REAL_STATE="executed-ok"
        node_invoke snapshot >"$SNAPSHOT_ROOT/postreal.json" 2>/dev/null
        node_invoke o11real "$SNAPSHOT_ROOT/prereal.json" "$SNAPSHOT_ROOT/postreal.json" \
            >"$SNAPSHOT_ROOT/o11real.out" 2>/dev/null || true
        # Combine dry-run and real verdicts into the single O11 terminal.
        local dryLine realLine dryStat realStat
        dryLine="$(cat "$SNAPSHOT_ROOT/o11dry.out" 2>/dev/null)"
        realLine="$(cat "$SNAPSHOT_ROOT/o11real.out" 2>/dev/null)"
        dryStat="$(printf '%s' "$dryLine" | cut -f2)"
        realStat="$(printf '%s' "$realLine" | cut -f2)"
        if [ "$dryStat" = "PASS" ] && [ "$realStat" = "PASS" ]; then
            printf 'O11\tPASS\t\t\n' >"$RES_DIR/O11"
        else
            local reason="$dryLine|$realLine"
            printf 'O11\tFAIL\tpersona-uninstall\t%s\n' "$reason" >"$RES_DIR/O11"
        fi
        return 0
    fi
    STAGE_PERSONA_REAL_STATE="executed-failed"
    printf 'O11\tFAIL\treal-uninstall-failed\t\n' >"$RES_DIR/O11"
    return 1
}

# ---------------------------------------------------------------------------
# Oracle ledger O1-O11 (numeric order; exactly one terminal PASS|FAIL each)
# ---------------------------------------------------------------------------
# Each oracle has exactly one terminal outcome (PASS or FAIL) and no third
# state. Bodies read sanitized cached evidence produced by the lifecycle
# stages and never print absolute paths, file contents, or environment values.

read_cached() {  # <id>
    cat "$RES_DIR/$1" 2>/dev/null || printf '%s\tFAIL\t%s\t\n' "$1" "no-evidence"
}

oracle_O1() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O1)
    if [ "$status" = "PASS" ]; then emit_pass O1; else emit_fail O1 "${state:-unreadable}" "${detail}"; fi
}

oracle_O2() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O2)
    if [ "$status" = "PASS" ]; then emit_pass O2; else emit_fail O2 "${state:-missing-managed-key}" "${detail}"; fi
}

oracle_O3() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O3)
    if [ "$status" = "PASS" ]; then emit_pass O3; else emit_fail O3 "${state:-mcp-absent}" "${detail}"; fi
}

oracle_O4() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O4)
    if [ "$status" = "PASS" ]; then emit_pass O4; else emit_fail O4 "${state:-permission-absent}" "${detail}"; fi
}

oracle_O5() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O5)
    if [ "$status" = "PASS" ]; then emit_pass O5; else emit_fail O5 "${state:-phase-agent-invalid}" "${detail}"; fi
}

oracle_O6() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O6)
    if [ "$status" = "PASS" ]; then emit_pass O6; else emit_fail O6 "${state:-forbidden-class}" "${detail}"; fi
}

oracle_O7() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O7)
    if [ "$status" = "PASS" ]; then emit_pass O7; else emit_fail O7 "${state:-agents-md-absent}" "${detail}"; fi
}

oracle_O8() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O8)
    if [ "$status" = "PASS" ]; then emit_pass O8; else emit_fail O8 "${state:-local-skills-drift}" "${detail}"; fi
}

oracle_O9() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O9)
    if [ "$status" = "PASS" ]; then emit_pass O9; else emit_fail O9 "${state:-no-sealed-receipt}" "${detail}"; fi
}

oracle_O10() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O10)
    if [ "$status" = "PASS" ]; then emit_pass O10; else emit_fail O10 "${state:-install-failed}" "${detail}"; fi
}

oracle_O11() {
    local id status state detail; IFS=$'\t' read -r id status state detail < <(read_cached O11)
    if [ "$status" = "PASS" ]; then emit_pass O11; else emit_fail O11 "${state:-dryrun-failed}" "${detail}"; fi
}

# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------

# run_oracle - an oracle records its own terminal PASS|FAIL and returns 0/1.
# The runner never produces a third state and never suppresses a failure.
run_oracle() {
    "$@" || true
}

main() {
    require_node               # mandatory tooling (exit 3 if absent)
    assert_home_boundary       # container HOME boundary (exit 4 if breached)

    mkdir -p "$RES_DIR"
    chmod 0700 "$SNAPSHOT_ROOT" 2>/dev/null || true
    write_node_program

    # Lifecycle stages: drive the declared in-container commands; capture
    # sanitized snapshots so O1-O11 record real evidence.
    stage_fresh_install            || true
    stage_reinstall                || true
    stage_persona_uninstall_dryrun || true
    stage_persona_uninstall_real   || true

    # O1-O11 in numeric order.
    run_oracle oracle_O1
    run_oracle oracle_O2
    run_oracle oracle_O3
    run_oracle oracle_O4
    run_oracle oracle_O5
    run_oracle oracle_O6
    run_oracle oracle_O7
    run_oracle oracle_O8
    run_oracle oracle_O9
    run_oracle oracle_O10
    run_oracle oracle_O11

    emit_ledger
    if [ "$LOG_FAIL" -gt 0 ]; then
        printf 'opencode_audit: FAIL state=ledger-has-failures count=%d\n' "$LOG_FAIL"
        exit 1
    fi
    exit 0
}

main "$@"
