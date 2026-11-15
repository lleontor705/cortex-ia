#!/usr/bin/env node
// Prompt-budget ratchet for cortex-ia (advisory in CI).
//
// Measures the instruction plane under internal/assets: role prompts (always
// loaded together with AGENTS.md), SKILL.md bodies, and shared contracts.
// Tokens are estimated as characters / 4 (agent-writing-contract.md, Skill Body
// Budget). Imperative counts are capitalised MUST/NEVER/... words per file.
//
// Usage:
//   node scripts/prompt-budget.mjs           print the table, compare with baseline
//   node scripts/prompt-budget.mjs --regen   rewrite the baseline and print the table
//
// Exit 1 when a token metric grows more than 10% over baseline, an imperative
// count rises above baseline, or a SKILL.md body exceeds the 1000-token hard max.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ASSETS = path.join(ROOT, 'internal', 'assets');
const BASELINE = path.join(ROOT, '.cortex-ia', 'ratchet', 'prompt-budget.json');

const ROLES = ['orchestrator', 'discovery', 'investigate', 'planner', 'implement', 'reviewer'];
const SKILL_HARD_MAX = 1000;
const GROWTH_TOLERANCE = 0.1;
// opencode2-knowledge is a reference index, not an always-loaded procedure; its
// size is intentional. It stays tracked for growth but is exempt from the hard max.
const SKILL_BUDGET_ALLOWLIST = new Set(['opencode2-knowledge']);

const IMPERATIVE_RE = /\b(MUST|NEVER|ALWAYS|STRICTLY|MANDATORY|CRITICAL|IMMEDIATELY)\b/g;

export function stripFrontmatter(text) {
  const m = /^---\r?\n[\s\S]*?\r?\n---[ \t]*(?:\r?\n|$)/.exec(text);
  return m ? text.slice(m[0].length) : text;
}

export function estimateTokens(text) {
  return Math.ceil(text.length / 4);
}

export function countImperatives(text) {
  return (text.match(IMPERATIVE_RE) || []).length;
}

function measure(text, { frontmatter = false } = {}) {
  const body = frontmatter ? stripFrontmatter(text) : text;
  return { tokens: estimateTokens(body), imperatives: countImperatives(text) };
}

function readText(rel) {
  return fs.readFileSync(path.join(ASSETS, rel), 'utf8');
}

function listSkills() {
  const dir = path.join(ASSETS, 'skills');
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .filter((e) => e.isDirectory() && !e.name.startsWith('_') && fs.existsSync(path.join(dir, e.name, 'SKILL.md')))
    .map((e) => e.name)
    .sort();
}

function listShared() {
  const dir = path.join(ASSETS, 'skills', '_shared');
  return fs
    .readdirSync(dir)
    // Skip AppleDouble (._*) sidecars that exFAT volumes leave behind.
    .filter((f) => f.endsWith('.md') && !f.startsWith('.'))
    .sort();
}

export function collectMetrics() {
  const agentsMd = measure(readText('AGENTS.md'));
  const roles = {};
  for (const role of ROLES) {
    const body = measure(readText(path.join('agents', `${role}.md`)), { frontmatter: true });
    roles[role] = {
      body: body.tokens,
      always: body.tokens + agentsMd.tokens,
      imperatives: body.imperatives,
    };
  }
  const skills = {};
  for (const name of listSkills()) {
    const m = measure(readText(path.join('skills', name, 'SKILL.md')), { frontmatter: true });
    skills[name] = { body: m.tokens, imperatives: m.imperatives };
  }
  const shared = {};
  for (const file of listShared()) {
    const m = measure(readText(path.join('skills', '_shared', file)));
    shared[file] = { tokens: m.tokens, imperatives: m.imperatives };
  }
  return {
    schema: 1,
    agentsMd: { tokens: agentsMd.tokens, imperatives: agentsMd.imperatives },
    roles,
    skills,
    shared,
  };
}

// Flattens metrics into rows keyed by a stable id, so baseline and current compare 1:1.
function flatten(metrics) {
  const rows = new Map();
  rows.set('AGENTS.md', { tokens: metrics.agentsMd.tokens, imperatives: metrics.agentsMd.imperatives });
  for (const [role, v] of Object.entries(metrics.roles)) {
    rows.set(`role:${role}`, { tokens: v.always, body: v.body, imperatives: v.imperatives });
  }
  for (const [name, v] of Object.entries(metrics.skills)) {
    rows.set(`skill:${name}`, { tokens: v.body, imperatives: v.imperatives });
  }
  for (const [file, v] of Object.entries(metrics.shared)) {
    rows.set(`shared:${file}`, { tokens: v.tokens, imperatives: v.imperatives });
  }
  return rows;
}

export function evaluate(metrics, baseline) {
  const findings = [];
  for (const [name, v] of Object.entries(metrics.skills)) {
    if (v.body > SKILL_HARD_MAX && !SKILL_BUDGET_ALLOWLIST.has(name)) {
      findings.push(`skill ${name}: body ${v.body} tokens exceeds hard max ${SKILL_HARD_MAX}`);
    }
  }
  if (!baseline) {
    return findings;
  }
  const base = flatten(baseline);
  for (const [id, cur] of flatten(metrics)) {
    const prev = base.get(id);
    if (!prev) continue;
    if (cur.tokens > prev.tokens * (1 + GROWTH_TOLERANCE)) {
      findings.push(`${id}: tokens ${prev.tokens} -> ${cur.tokens} (grew more than 10%)`);
    }
    if (cur.imperatives > prev.imperatives) {
      findings.push(`${id}: imperatives ${prev.imperatives} -> ${cur.imperatives}`);
    }
  }
  return findings;
}

function delta(cur, prev) {
  if (prev === undefined) return 'new';
  const d = cur - prev;
  return d === 0 ? '0' : `${d > 0 ? '+' : ''}${d}`;
}

export function renderTable(metrics, baseline) {
  const base = baseline ? flatten(baseline) : new Map();
  const lines = [];
  const row = (label, cols, id) => {
    const prev = id ? base.get(id) : undefined;
    lines.push(
      [label.padEnd(36), ...cols.map((c) => String(c).padStart(9))].join(' ') +
        (baseline && id ? `  d-tok ${delta(cols[0], prev?.tokens)}` : ''),
    );
  };
  const head = (title, cols) => {
    lines.push('', title, [''.padEnd(36), ...cols.map((c) => c.padStart(9))].join(' '));
  };
  head('Role prompts (always-loaded = body + AGENTS.md)', ['always', 'body', 'imperat']);
  for (const [role, v] of Object.entries(metrics.roles)) {
    row(role, [v.always, v.body, v.imperatives], `role:${role}`);
  }
  head('AGENTS.md', ['tokens', 'imperat']);
  row('internal/assets/AGENTS.md', [metrics.agentsMd.tokens, metrics.agentsMd.imperatives], 'AGENTS.md');
  head('SKILL.md bodies (budget 1000)', ['body', 'imperat']);
  for (const [name, v] of Object.entries(metrics.skills)) {
    const flag = v.body > SKILL_HARD_MAX ? (SKILL_BUDGET_ALLOWLIST.has(name) ? ' (allowlisted)' : ' OVER') : '';
    row(name + flag, [v.body, v.imperatives], `skill:${name}`);
  }
  head('Shared contracts (_shared/*.md)', ['tokens', 'imperat']);
  for (const [file, v] of Object.entries(metrics.shared)) {
    row(file, [v.tokens, v.imperatives], `shared:${file}`);
  }
  return lines.join('\n');
}

function main(argv) {
  const regen = argv.includes('--regen');
  const metrics = collectMetrics();
  let baseline = null;
  if (!regen && fs.existsSync(BASELINE)) {
    baseline = JSON.parse(fs.readFileSync(BASELINE, 'utf8'));
  }
  console.log(renderTable(metrics, baseline));

  if (regen) {
    fs.mkdirSync(path.dirname(BASELINE), { recursive: true });
    fs.writeFileSync(BASELINE, `${JSON.stringify(metrics, null, 2)}\n`);
    console.log(`\nprompt-budget: baseline written to ${path.relative(ROOT, BASELINE)}`);
    return 0;
  }
  if (!baseline) {
    console.log('\nprompt-budget: no baseline; run node scripts/prompt-budget.mjs --regen');
  }
  const findings = evaluate(metrics, baseline);
  if (findings.length === 0) {
    console.log('\nprompt-budget: OK (within baseline and budget)');
    return 0;
  }
  console.log('\nprompt-budget: DRIFT');
  for (const f of findings) console.log(`  - ${f}`);
  return 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  process.exitCode = main(process.argv.slice(2));
}
