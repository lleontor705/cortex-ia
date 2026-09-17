import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { loadPluginFile, loadPluginSource } from './harness-plugin-loader.mjs';

const bridgePath = 'internal/assets/plugins/herdr-bridge.ts';
const guardPath = 'internal/assets/plugins/cortex-lease-guard.ts';
const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const schema = new Proxy(() => schema, { get: () => schema });
const sdk = { tool: Object.assign(definition => definition, { schema }) };

function options(leased = [], symlink = '') {
  const calls = [];
  return {
    calls, cwd: root, env: { USERPROFILE: home, HOME: home }, mockPluginSDK: sdk,
    fs: {
      existsSync: p => p === executable,
      readFileSync: () => '',
      realpathSync: p => p,
      lstatSync: p => ({ isSymbolicLink: () => p === symlink }),
      mkdirSync: () => {}, writeFileSync: () => {},
    },
    childProcess: {
      execFileSync: (_file, args) => {
        calls.push(args);
        if (args[1] === 'verify-lease') {
          const targets = args.flatMap((v, i) => v === '--path' ? [args[i + 1]] : []);
          return JSON.stringify({ valid: targets.every(t => leased.includes(t)),
            owner: 'opencode-session:test', task_id: 'task-1',
            expires_at: new Date(Date.now() + 60000).toISOString() });
        }
        return '{}';
      },
    },
  };
}

test('actual bridge initializes every declared tool, including reviewer reads', async () => {
  const opts = options();
  const mod = await loadPluginFile(bridgePath, opts);
  const bridge = await mod.instantiate({ client: {} });
  const source = fs.readFileSync(bridgePath, 'utf8');
  const declared = [...source.matchAll(/^\s+(cortex_ia_\w+): tool\(/gm)].map(m => m[1]).sort();
  assert.deepEqual(Object.keys(bridge.tool).sort(), declared);
  for (const capability of ['approvals', 'fingerprint']) {
    await bridge.tool[`cortex_ia_work_${capability}`].execute({ task_id: 'task-1' }, {});
    assert.ok(opts.calls.some(args => args[0] === 'work' && args[1] === capability));
  }
  await assert.rejects(bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-1' },
    { agent: 'reviewer', sessionID: 'test' }), /BRIDGE_ROLE_DENIED/);
});

test('an unclassified declaration fails closed at bridge initialization', async () => {
  const source = fs.readFileSync(bridgePath, 'utf8').replace(
    'cortex_ia_content_hash: tool(', 'cortex_ia_unclassified_probe: tool(');
  const mod = await loadPluginSource(source, options());
  await assert.rejects(mod.instantiate({ client: {} }), /BRIDGE_POLICY_UNCLASSIFIED/);
});

test('all formerly exempt paths require leases before writes', async () => {
  const mod = await loadPluginFile(guardPath, options());
  const hook = (await mod.instantiate({ directory: root }))['tool.execute.before'];
  for (const filePath of ['.git/config', '.cortex-ia/policy.json', 'scratch/probe.ts',
    '.tmp/probe.ts', 'tmp/probe.ts', '.gemini/settings.json', 'audit.log', 'artifact.tmp']) {
    await assert.rejects(hook({ tool: 'write', sessionID: 'test' }, { args: { filePath } }), /LEASE_REQUIRED/);
  }
});

test('outside and symlink targets reject before consulting lease authority', async () => {
  const opts = options([], path.join(root, 'linked'));
  const mod = await loadPluginFile(guardPath, opts);
  const hook = (await mod.instantiate({ directory: root }))['tool.execute.before'];
  for (const filePath of ['../../outside/scratch/policy.json', '../outside/audit.log',
    path.resolve(root, '../outside/.git/config'), 'linked/file.ts']) {
    await assert.rejects(hook({ tool: 'write', sessionID: 'test' }, { args: { filePath } }), /LEASE_CHECK_FAILED/);
  }
  assert.equal(opts.calls.length, 0);
});

test('contained leased targets pass, and mixed patches verify every target', async () => {
  const opts = options(['src/main.ts', 'scratch/probe.ts']);
  const mod = await loadPluginFile(guardPath, opts);
  const hook = (await mod.instantiate({ directory: root }))['tool.execute.before'];
  for (const filePath of ['src/main.ts', path.join(root, 'scratch/probe.ts')]) {
    await hook({ tool: 'write', sessionID: 'test' }, { args: { filePath } });
  }
  await assert.rejects(hook({ tool: 'apply_patch', sessionID: 'test' }, { args: {
    patchText: '*** Begin Patch\n*** Update File: src/main.ts\n@@\n-a\n+b\n*** Add File: audit.log\n+entry\n*** End Patch',
  } }), /LEASE_REQUIRED/);
  const last = opts.calls.at(-1);
  assert.deepEqual(Array.from(last.slice(last.indexOf('--path'))), ['--path', 'audit.log', '--path', 'src/main.ts']);
});
