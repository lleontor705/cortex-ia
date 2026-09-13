import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const pluginOpts = { fs, cwd: process.cwd(), env: process.env };

function createTempDir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

function writeSkill(baseDir, skillName, description) {
  const dir = path.join(baseDir, skillName);
  fs.mkdirSync(dir, { recursive: true });
  const file = path.join(dir, 'SKILL.md');
  fs.writeFileSync(file, '---\nname: ' + skillName + '\ndescription: "' + description + '"\n---\n# ' + skillName + '\n', 'utf-8');
  return file;
}

test('canonicalPath resolves OS-canonical identity and rejects invalid locators', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { canonicalPath } = mod.exports;

  const current = canonicalPath('.');
  assert.ok(path.isAbsolute(current));
  assert.ok(!current.includes('\\'));

  assert.throws(() => canonicalPath(''), /CONTEXT_RESOLUTION_ERROR: invalid locator path/);
  assert.throws(() => canonicalPath('   '), /CONTEXT_RESOLUTION_ERROR: invalid locator path/);
});

test('resolveLocator resolves explicit path and fails visibly on missing or invalid target', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { resolveLocator } = mod.exports;

  const tmp = createTempDir('cortex-ctx-loc-');
  try {
    const skillFile = writeSkill(tmp, 'explicit-tool', 'Explicitly loaded tool');
    const resolved = resolveLocator(skillFile);
    assert.equal(resolved.name, 'explicit-tool');
    assert.equal(resolved.origin, 'explicit');
    assert.equal(resolved.precedence, 0);
    assert.equal(resolved.description, 'Explicitly loaded tool');

    const nonExistent = path.join(tmp, 'ghost', 'SKILL.md');
    assert.throws(
      () => resolveLocator(nonExistent),
      /CONTEXT_RESOLUTION_ERROR: explicit locator not found or unreadable/
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('resolveSkill follows precedence: host-installed > repository > embedded-source', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { resolveSkill } = mod.exports;

  const tmpHost = createTempDir('cortex-host-');
  const tmpRepo = createTempDir('cortex-repo-');
  const tmpEmbedded = createTempDir('cortex-emb-');

  try {
    writeSkill(tmpHost, 'shared-skill', 'Host version');
    writeSkill(tmpRepo, 'shared-skill', 'Repo version');
    writeSkill(tmpEmbedded, 'shared-skill', 'Embedded version');

    const opts = {
      hostSkillsDirs: [tmpHost],
      repoSkillsDirs: [tmpRepo],
      embeddedSkillsDirs: [tmpEmbedded],
    };

    const hostRes = resolveSkill('shared-skill', opts);
    assert.equal(hostRes.origin, 'host-installed');
    assert.equal(hostRes.precedence, 1);
    assert.equal(hostRes.description, 'Host version');

    writeSkill(tmpRepo, 'repo-skill', 'Repo version only');
    writeSkill(tmpEmbedded, 'repo-skill', 'Embedded version fallback');

    const repoRes = resolveSkill('repo-skill', opts);
    assert.equal(repoRes.origin, 'repository');
    assert.equal(repoRes.precedence, 2);
    assert.equal(repoRes.description, 'Repo version only');

    writeSkill(tmpEmbedded, 'core-skill', 'Embedded version only');

    const embRes = resolveSkill('core-skill', opts);
    assert.equal(embRes.origin, 'embedded-source');
    assert.equal(embRes.precedence, 3);
    assert.equal(embRes.description, 'Embedded version only');

    assert.throws(
      () => resolveSkill('missing-skill', opts),
      /CONTEXT_RESOLUTION_ERROR: skill 'missing-skill' not found in any inventory/
    );
  } finally {
    fs.rmSync(tmpHost, { recursive: true, force: true });
    fs.rmSync(tmpRepo, { recursive: true, force: true });
    fs.rmSync(tmpEmbedded, { recursive: true, force: true });
  }
});

test('discoverInventory detects same-precedence collisions with candidate list', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { discoverInventory } = mod.exports;

  const dirA = createTempDir('cortex-col-a-');
  const dirB = createTempDir('cortex-col-b-');

  try {
    writeSkill(dirA, 'conflicting-skill', 'Version A');
    writeSkill(dirB, 'conflicting-skill', 'Version B');

    assert.throws(
      () => discoverInventory([dirA, dirB], 'repository', 2),
      (err) => {
        assert.match(err.message, /CONTEXT_RESOLUTION_ERROR: same-precedence collision for skill 'conflicting-skill'/);
        assert.match(err.message, /candidates/);
        return true;
      }
    );
  } finally {
    fs.rmSync(dirA, { recursive: true, force: true });
    fs.rmSync(dirB, { recursive: true, force: true });
  }
});

test('getAllDiscoveredSkills preserves inventory provenance and exposes metadata without content logging', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { getAllDiscoveredSkills } = mod.exports;

  const tmpHost = createTempDir('cortex-p-host-');
  const tmpRepo = createTempDir('cortex-p-repo-');

  try {
    writeSkill(tmpHost, 'host-a', 'Host skill description');
    writeSkill(tmpRepo, 'repo-b', 'Repo skill description');

    const all = getAllDiscoveredSkills({
      hostSkillsDirs: [tmpHost],
      repoSkillsDirs: [tmpRepo],
      embeddedSkillsDirs: [],
    });

    assert.equal(all.length, 2);
    const hostA = all.find((s) => s.name === 'host-a');
    const repoB = all.find((s) => s.name === 'repo-b');

    assert.ok(hostA);
    assert.equal(hostA.origin, 'host-installed');
    assert.equal(hostA.precedence, 1);
    assert.equal(hostA.description, 'Host skill description');
    assert.ok(!('content' in hostA));

    assert.ok(repoB);
    assert.equal(repoB.origin, 'repository');
    assert.equal(repoB.precedence, 2);
    assert.equal(repoB.description, 'Repo skill description');
    assert.ok(!('content' in repoB));
  } finally {
    fs.rmSync(tmpHost, { recursive: true, force: true });
    fs.rmSync(tmpRepo, { recursive: true, force: true });
  }
});

test('CortexSkillDiscoveryPlugin adds repository skills note to experimental.chat.system.transform', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-skill-discovery.ts', pluginOpts);
  const { CortexSkillDiscoveryPlugin } = mod.exports;

  const tmpRepo = createTempDir('cortex-plug-repo-');
  const skillsDir = path.join(tmpRepo, 'skills');
  writeSkill(skillsDir, 'project-tool', 'Project specific skill');

  try {
    const plugin = await CortexSkillDiscoveryPlugin({ directory: tmpRepo });
    const output = { system: ['Base system prompt'] };

    await plugin['experimental.chat.system.transform']({}, output);

    assert.equal(output.system.length, 1);
    assert.match(output.system[0], /Repository-Local Custom Skills Available/);
    assert.match(output.system[0], /project-tool/);
    assert.match(output.system[0], /Project specific skill/);
  } finally {
    fs.rmSync(tmpRepo, { recursive: true, force: true });
  }
});
