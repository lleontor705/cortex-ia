import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import { createIsolatedSandbox, loadPluginFile, transpileTS } from './harness-plugin-loader.mjs';

const PLUGINS = 'internal/assets/plugins/';
const [WORK, TELEMETRY, FENCE, LEASE, SKILLS] = ['cortex-work.ts', 'cortex-tool-telemetry.ts', 'cortex-permission-fence.ts', 'cortex-lease-guard.ts', 'cortex-skill-discovery.ts'].map(f => PLUGINS + f);
const schema = new Proxy(() => schema, { get: () => schema });
const context = { agent: 'implement', sessionID: 'controller', directory: '/isolated/work' };
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const tempDir = prefix => fs.mkdtempSync(path.join(os.tmpdir(), prefix));

async function heartbeat() {
  let now = Date.parse('2026-01-01T00:00:00Z'), next = 0;
  const timers = new Map(), calls = [], controls = { renewFailures: 0 };
  const expiry = () => new Date(now + 900000).toISOString();
  const sandbox = createIsolatedSandbox({
    env: { HOME: home }, virtualFiles: { [path.join(home, 'go', 'bin', 'cortex-ia')]: 'binary' },
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    childProcess: { execFileSync: (_file, args) => {
      calls.push(Array.from(args));
      if (args[0] === 'version') return '';
      if (args[1] === 'claim') return JSON.stringify({ claim_token: 'private-claim', reserved_files: [{ path: 'a.go', lease_token: 'private-a' }] });
      if (args[1] === 'status') return JSON.stringify({ status: 'in_progress', allowed_files: ['a.go'], claim: { owner: 'opencode-session:controller', expires_at: expiry() } });
      if (args[1] === 'controller-renew') {
        if (controls.renewFailures > 0) { controls.renewFailures--; throw new Error('transient renewal failure'); }
        return JSON.stringify({ task_id: 'task', owner: 'opencode-session:controller', lease_count: 1, expires_at: expiry() });
      }
      return '{}';
    } },
  });
  sandbox.Date = class extends Date { constructor(...args) { super(...(args.length ? args : [now])); } static now() { return now; } };
  sandbox.setTimeout = (callback, delay) => { const id = ++next; timers.set(id, { at: now + delay, callback }); return id; };
  sandbox.clearTimeout = id => timers.delete(id);
  vm.runInContext(transpileTS(fs.readFileSync(WORK, 'utf8')), sandbox);
  const ctx = { client: { session: { status: async () => ({ data: { controller: { type: 'busy' } } }) } } };
  const definition = sandbox.exports.CortexDelegationBridge;
  const plugin = await (typeof definition === 'function' ? definition(ctx) : definition.setup(ctx));
  const flush = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); };
  const advance = async ms => {
    const target = now + ms;
    for (let entry; (entry = [...timers].filter(([, t]) => t.at <= target).sort((a, b) => a[1].at - b[1].at)[0]);) {
      now = entry[1].at; timers.delete(entry[0]); void entry[1].callback(); await flush();
    }
    now = target; await flush();
  };
  await plugin.tool.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context);
  return {
    advance, controls, plugin,
    status: async () => JSON.parse(await plugin.tool.cortex_ia_work_status.execute({ task_id: 'task' }, context)),
    event: pending => plugin.event({ event: pending }),
    renewals: () => calls.filter(c => c[1] === 'controller-renew').length,
  };
}

test('an unattributed session.error does not stop maintenance; a matching one does', async () => {
  const h = await heartbeat();
  await h.event({ type: 'session.error', properties: { error: 'boom' } });
  await h.event({ type: 'session.error', properties: { sessionID: 'unrelated', error: 'boom' } });
  await h.advance(30000);
  assert.equal(h.renewals(), 1);
  assert.equal((await h.status()).maintenance.active, true);
  await h.event({ type: 'session.error', properties: { sessionID: 'controller', error: 'boom' } });
  assert.deepEqual((await h.status()).maintenance, { active: false, reason: 'session.error' });
  await h.advance(120000);
  assert.equal(h.renewals(), 1);
  await h.plugin.dispose();
});

test('a transient renewal failure is retried and recorded, then clears on success', async () => {
  const h = await heartbeat();
  h.controls.renewFailures = 1;
  await h.advance(30000);
  assert.equal(h.renewals(), 1, 'the first attempt failed');
  await h.advance(2000);
  assert.equal(h.renewals(), 2, 'the retry after backoff ran');
  assert.deepEqual((await h.status()).maintenance, { active: true, reason: null });
  await h.plugin.dispose();
});

test('persistent renewal failure stops after the retry budget with the reason in compact status', async () => {
  const h = await heartbeat();
  h.controls.renewFailures = 99;
  await h.advance(120000);
  const status = await h.status();
  assert.deepEqual(status.maintenance, { active: false, reason: 'maintenance_failed_manual_reconciliation_required' });
  assert.equal(JSON.stringify(status).includes('private-'), false, 'compact status never carries tokens');
  await h.plugin.dispose();
});

test('claim_token and lease_token are redacted in JSON, key=value and CLI-flag forms', async () => {
  const mod = await loadPluginFile(TELEMETRY, { cwd: home, env: { HOME: home }, childProcess: { execFileSync: () => executable, execFile: () => undefined } });
  const cases = [
    ['{"claim_token":"DEMOCLAIM1"}', 'DEMOCLAIM1'],
    ['{"lease_token": "DEMOLEASE1", "x": 1}', 'DEMOLEASE1'],
    ['claim_token=DEMOCLAIM2 next', 'DEMOCLAIM2'],
    ['cortex-ia work reserve --lease-token DEMOLEASE2 --path a.go', 'DEMOLEASE2'],
    ['--claim-token=DEMOCLAIM3', 'DEMOCLAIM3'],
    ['lease-token: DEMOLEASE3', 'DEMOLEASE3'],
  ];
  for (const [input, secret] of cases) {
    const out = mod.exports.redactSecrets(input);
    assert.equal(out.includes(secret), false, `${input} -> ${out}`);
    assert.match(out, /\[REDACTED:token\]/);
  }
});

test('tool error reports keep secrets out of argv and redact the stdin details', async () => {
  const argvCalls = [], stdin = [];
  const mod = await loadPluginFile(TELEMETRY, {
    cwd: home, env: { HOME: home },
    fs: { existsSync: p => p === executable, realpathSync: p => p, lstatSync: () => ({ isSymbolicLink: () => false }), readFileSync: () => '', writeFileSync: () => {}, mkdirSync: () => {} },
    childProcess: {
      execFileSync: () => executable,
      execFile: (_file, args) => { argvCalls.push(args); return { stdin: { on() {}, end(data) { stdin.push(data); } } }; },
    },
  });
  const plugin = await mod.instantiate({ directory: path.join(home, 'project') });
  await plugin['tool.execute.after'](
    { tool: 'edit', sessionID: 'ses-redact', callID: 'call-r', args: {} },
    { output: 'Error: write refused: lease_token=DEMOLEASE9 and claim_token: DEMOCLAIM9' }
  );
  assert.equal(argvCalls.length, 1);
  const argv = argvCalls[0].join(' ');
  assert.equal(argv.includes('DEMOLEASE9') || argv.includes('DEMOCLAIM9'), false);
  assert.equal(argvCalls[0][argvCalls[0].indexOf('--details') + 1], '@stdin');
  assert.equal(stdin.length, 1);
  assert.equal(stdin[0].includes('DEMOLEASE9') || stdin[0].includes('DEMOCLAIM9'), false);
  assert.match(stdin[0], /\[REDACTED:token\]/);
  await plugin.dispose();
});

async function fenceEvaluate(directory, role) {
  const mod = await loadPluginFile(FENCE, { cwd: directory, env: { HOME: home, USERPROFILE: home }, childProcess: { execFileSync: () => { throw new Error('no cortex-ia'); } } });
  let evaluate;
  await mod.instantiate({ location: { directory }, permission: { hook: async (_name, fn) => { evaluate = fn; } } });
  return (tool, input) => evaluate({ tool, session: { role }, input });
}

test('multiedit and patch are denied for read-only roles, and every patch target is checked', async () => {
  const dir = tempDir('cortex-fence-');
  try {
    const readOnly = await fenceEvaluate(dir, 'investigate');
    assert.equal((await readOnly('multiedit', { filePath: 'a.go' })).effect, 'deny');
    const addFile = '*** Begin Patch\n*** Add File: notes.txt\n+hello\n*** End Patch';
    assert.equal((await readOnly('apply_patch', { patchText: addFile })).effect, 'deny');
    assert.equal((await readOnly('patch', { patchText: addFile })).effect, 'deny');

    const writer = await fenceEvaluate(dir, 'implement');
    const movesIntoEnv = '*** Begin Patch\n*** Update File: ok.go\n*** Move to: .env\n*** End Patch';
    assert.equal((await writer('apply_patch', { patchText: movesIntoEnv })).effect, 'deny');
    assert.equal((await writer('apply_patch', { patchText: 'not a patch' })).effect, 'deny');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('the discovery .gitignore exception covers only the root file in the permission fence and lease guard', async () => {
  const dir = tempDir('cortex-gitignore-');
  try {
    const evaluate = await fenceEvaluate(dir, 'discovery');
    assert.equal((await evaluate('edit', { filePath: '.gitignore' }))?.effect, undefined);
    assert.equal((await evaluate('edit', { filePath: 'sub/.gitignore' })).effect, 'deny');

    const leaseMod = await loadPluginFile(LEASE, {
      fs, cwd: dir, env: { HOME: dir, USERPROFILE: dir },
      childProcess: { execFileSync: () => { throw new Error('no cortex-ia'); } },
    });
    let before;
    await leaseMod.instantiate({ location: { directory: dir }, tool: { hook: async (_name, fn) => { before = fn; } } });
    const edit = filePath => ({ tool: 'edit', sessionID: 'ses-gitignore', session: { role: 'discovery' }, input: { filePath } });
    await assert.doesNotReject(before(edit('.gitignore')));
    await assert.rejects(before(edit('sub/.gitignore')), /LEASE_REQUIRED/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('a skill name present under skills/ and .agents/skills/ resolves by precedence without throwing', async () => {
  const root = tempDir('cortex-dup-'), homeDir = tempDir('cortex-dup-home-');
  const write = (dir, description) => {
    fs.mkdirSync(path.join(dir, 'dup'), { recursive: true });
    fs.writeFileSync(path.join(dir, 'dup', 'SKILL.md'), `---\nname: dup\ndescription: ${description}\n---\n`);
  };
  try {
    write(path.join(root, 'skills'), 'Project copy');
    write(path.join(root, '.agents', 'skills'), 'Agents copy');
    const mod = await loadPluginFile(SKILLS, { fs, cwd: root, env: { HOME: homeDir, USERPROFILE: homeDir } });
    const opts = { directory: root, hostSkillsDirs: [], embeddedSkillsDirs: [] };
    assert.doesNotThrow(() => mod.exports.getAllDiscoveredSkills(opts));
    assert.equal(mod.exports.resolveSkill('dup', opts).description, 'Project copy');
    await assert.doesNotReject(mod.instantiate({ directory: root }));
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
    fs.rmSync(homeDir, { recursive: true, force: true });
  }
});
