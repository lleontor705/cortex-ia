import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const AGENTS_DIR = 'internal/assets/agents';

const AGENTS = ['orchestrator', 'discovery', 'investigate', 'planner', 'implement', 'reviewer'];
const MINIONS = ['discovery', 'investigate', 'planner', 'implement', 'reviewer'];
const SUBAGENT_ALLOWLIST = ['discovery', 'investigate', 'planner', 'implement', 'reviewer'];
const EXPECTED_STEPS = {
  orchestrator: 500,
  discovery: 100,
  investigate: 150,
  planner: 150,
  implement: 250,
  reviewer: 150,
};
const RETIRED_V1_ACTIONS = new Set(['write_to_file', 'apply_patch']);

function readAgent(name) {
  const file = path.join(AGENTS_DIR, `${name}.md`);
  const text = fs.readFileSync(file, 'utf8');
  const match = text.match(/^---\r?\n([\s\S]*?)\r?\n---/);
  assert.ok(match, `${file}: missing YAML frontmatter delimited by ---`);
  return { file, text, frontmatter: match[1] };
}

function unquote(value) {
  return value.trim().replace(/^["']|["']$/g, '');
}

function parsePermissions(frontmatter) {
  const entries = [];
  let current = null;
  for (const rawLine of frontmatter.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === '' || line.startsWith('#')) continue;
    const action = line.match(/^-\s*action:\s*(.+)$/);
    if (action) {
      current = { action: unquote(action[1]) };
      entries.push(current);
      continue;
    }
    if (!current) continue;
    const resource = line.match(/^resource:\s*(.+)$/);
    if (resource) {
      current.resource = unquote(resource[1]);
      continue;
    }
    const effect = line.match(/^effect:\s*(.+)$/);
    if (effect) current.effect = unquote(effect[1]);
  }
  return entries.filter(
    (entry) => entry.action !== undefined && entry.resource !== undefined && entry.effect !== undefined,
  );
}

function permissionsFor(name) {
  const { file, frontmatter } = readAgent(name);
  return { file, permissions: parsePermissions(frontmatter) };
}

function findPermission(permissions, action, resource) {
  return permissions.find((entry) => entry.action === action && entry.resource === resource);
}

test('TestREQ_PERM_001 question action denies the five minions and allows the orchestrator', () => {
  const orchestrator = readAgent('orchestrator');
  const question = findPermission(parsePermissions(orchestrator.frontmatter), 'question', '*');
  assert.ok(question, `${orchestrator.file}: missing "question resource:*" permission entry`);
  assert.equal(
    question.effect,
    'allow',
    `${orchestrator.file}: invariant question-allow violated, effect is "${question.effect}"`,
  );

  for (const name of MINIONS) {
    const { file, permissions } = permissionsFor(name);
    const entry = findPermission(permissions, 'question', '*');
    assert.ok(entry, `${file}: missing "question resource:*" permission entry`);
    assert.equal(
      entry.effect,
      'deny',
      `${file}: invariant question-deny violated for minion, effect is "${entry.effect}"`,
    );
  }
});

test('TestREQ_PERM_002 orchestrator subagent allowlist is exactly the five native controllers', () => {
  const { file, frontmatter } = readAgent('orchestrator');
  const entries = parsePermissions(frontmatter).filter((entry) => entry.action === 'subagent');

  const denyAll = entries.filter((entry) => entry.resource === '*' && entry.effect === 'deny');
  assert.equal(denyAll.length, 1, `${file}: invariant subagent-deny-umbrella violated, expected one subagent * deny`);

  const allowAll = entries.filter((entry) => entry.resource === '*' && entry.effect === 'allow');
  assert.equal(allowAll.length, 0, `${file}: invariant general-dispatch violated, subagent * must never be allowed`);

  const allowed = entries
    .filter((entry) => entry.effect === 'allow' && entry.resource !== '*')
    .map((entry) => entry.resource)
    .sort();
  assert.deepEqual(
    allowed,
    [...SUBAGENT_ALLOWLIST].sort(),
    `${file}: invariant subagent-allowlist violated, allow list is [${allowed.join(', ')}]`,
  );
});

test('TestREQ_PERM_003 cortex permission graph has one deny umbrella and only namespaced allows', () => {
  for (const name of AGENTS) {
    const { file, permissions } = permissionsFor(name);

    const cortexUmbrella = permissions.filter((entry) => entry.action === 'cortex_*');
    assert.equal(
      cortexUmbrella.filter((entry) => entry.effect === 'deny').length,
      1,
      `${file}: invariant cortex-deny-umbrella violated, expected exactly one cortex_* deny`,
    );
    assert.equal(
      cortexUmbrella.filter((entry) => entry.effect === 'allow').length,
      0,
      `${file}: invariant cortex-deny-umbrella violated, cortex_* must never be allowed`,
    );

    const bridgeUmbrella = permissions.filter((entry) => entry.action === 'cortex_ia_*');
    assert.equal(
      bridgeUmbrella.filter((entry) => entry.effect === 'deny').length,
      1,
      `${file}: invariant cortex-ia-umbrella violated, expected exactly one cortex_ia_* deny`,
    );
    assert.equal(
      bridgeUmbrella.filter((entry) => entry.effect === 'allow').length,
      0,
      `${file}: invariant cortex-ia-umbrella violated, cortex_ia_* must never be allowed`,
    );

    const allows = permissions.filter((entry) => entry.effect === 'allow' && entry.action.startsWith('cortex_'));
    const bare = allows.filter(
      (entry) => !entry.action.startsWith('cortex_cortex_') && !entry.action.startsWith('cortex_ia_'),
    );
    assert.deepEqual(
      bare.map((entry) => entry.action),
      [],
      `${file}: invariant bare-cortex-allow violated, bare cortex_<tool> allows are forbidden`,
    );

    assert.ok(
      allows.some((entry) => entry.action.startsWith('cortex_cortex_')),
      `${file}: invariant cortex-graph-shape violated, expected at least one cortex_cortex_* allow`,
    );
    assert.ok(
      allows.some((entry) => entry.action.startsWith('cortex_ia_')),
      `${file}: invariant cortex-graph-shape violated, expected at least one cortex_ia_* bridge allow`,
    );
  }
});

test('TestREQ_PERM_004 no agent declares retired v1 actions', () => {
  for (const name of AGENTS) {
    const { file, permissions } = permissionsFor(name);
    for (const entry of permissions) {
      assert.ok(
        !RETIRED_V1_ACTIONS.has(entry.action),
        `${file}: invariant no-v1-actions violated, retired action "${entry.action}" is present`,
      );
    }
  }
});

test('TestREQ_PERM_005 no agent carries retired request.body.temperature blocks', () => {
  for (const name of AGENTS) {
    const { file, text } = readAgent(name);
    assert.doesNotMatch(
      text,
      /request\.body\.temperature/,
      `${file}: invariant no-temperature-block violated, request.body.temperature is retired to the transport plugin`,
    );
  }
});

test('TestREQ_PERM_006 steps are present with the shipped per-role values', () => {
  for (const name of AGENTS) {
    const { file, frontmatter } = readAgent(name);
    const match = frontmatter.match(/^steps:\s*(\d+)\s*$/m);
    assert.ok(match, `${file}: invariant steps-present violated, no top-level steps value found`);
    assert.equal(
      Number(match[1]),
      EXPECTED_STEPS[name],
      `${file}: invariant steps-value violated, expected ${EXPECTED_STEPS[name]} got ${match[1]}`,
    );
  }
});
