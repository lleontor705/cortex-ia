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
  const inputs = [];
  return {
    calls, inputs, cwd: root, env: { USERPROFILE: home, HOME: home }, mockPluginSDK: sdk,
    fs: {
      existsSync: p => p === executable,
      readFileSync: () => '',
      realpathSync: p => p,
      lstatSync: p => ({ isSymbolicLink: () => p === symlink }),
      mkdirSync: () => {}, writeFileSync: () => {},
    },
    childProcess: {
      execFileSync: (_file, args, opts) => {
        calls.push(args);
        inputs.push(opts?.input);
        if (args[1] === 'claim') return JSON.stringify({ claim_token: 'synthetic-claim', reserved_files: leased.map(path => ({ path, lease_token: 'synthetic-lease' })) });
        if (args[1] === 'status') return JSON.stringify({ status: 'in_progress', claim: { owner: 'opencode-session:test', expires_at: new Date(Date.now() + 60000).toISOString() } });
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

test('orchestrator is authorized to call work_approve for Tier 2 auto-approval', async () => {
  const opts = options();
  const mod = await loadPluginFile(bridgePath, opts);
  const bridge = await mod.instantiate({ client: {} });
  await bridge.tool.cortex_ia_work_approve.execute(
    { task_id: 'task-tier2', verdict: 'PASS', summary: 'Auto-approved low-risk change' },
    { agent: 'orchestrator', sessionID: 'test-orch' }
  );
  assert.ok(opts.calls.some(args => args[0] === 'work' && args[1] === 'approve' && args[2] === 'task-tier2'));

  for (const unauthorized of ['planner', 'investigate', 'implement']) {
    await assert.rejects(
      bridge.tool.cortex_ia_work_approve.execute(
        { task_id: 'task-tier2', verdict: 'PASS' },
        { agent: unauthorized, sessionID: 'test' }
      ),
      /BRIDGE_ROLE_DENIED/
    );
  }
});

test('inline conversion stays read-only and caller arguments cannot authorize output', async () => {
  const opts = options();
  const tools = (await (await loadPluginFile(bridgePath, opts)).instantiate({ client: {} })).tool;
  const reviewer = { agent: 'reviewer', sessionID: 'test', directory: root };
  await tools.cortex_ia_doc_convert.execute({ file_path: 'input.txt' }, reviewer);
  const conversion = opts.calls.find(args => args[0] === 'doc');
  assert.equal(conversion.includes('-o'), false);
  assert.equal(conversion.includes('--artifact-authority=@stdin'), false);
  await assert.rejects(tools.cortex_ia_doc_convert.execute({ file_path: 'input.txt', output_path: 'out.md', role: 'implement', sessionID: 'other', standalone: true }, reviewer), /BRIDGE_ROLE_DENIED/);
  await assert.rejects(tools.cortex_ia_diagram_render.execute({ spec_path: 'spec.json', output_path: 'out.html' }, reviewer), /BRIDGE_ROLE_DENIED/);
  assert.equal(opts.calls.filter(args => args[0] === 'doc').length, 1);
});

test('artifact tools pass only host-bound live authority over stdin', async () => {
  const opts = options(['out.md']);
  const tools = (await (await loadPluginFile(bridgePath, opts)).instantiate({ client: {} })).tool;
  const context = { agent: 'implement', sessionID: 'test', directory: root };
  await assert.rejects(tools.cortex_ia_doc_convert.execute({ file_path: 'input.txt', output_path: 'out.md' }, context), /LEASE_REQUIRED/);
  await tools.cortex_ia_work_claim.execute({ task_id: 'task-1', paths: ['out.md'] }, context);
  for (const name of ['cortex_ia_doc_convert', 'cortex_ia_diagram_render']) {
    const output = await tools[name].execute({ file_path: 'input.txt', spec_path: 'spec.json', output_path: 'out.md', role: 'reviewer', project: '/outside' }, context);
    const args = opts.calls.at(-1);
    assert.ok(args.includes('--artifact-authority=@stdin'));
    assert.equal(args.includes('--standalone'), false);
    assert.equal(JSON.stringify(args).includes('synthetic-'), false);
    assert.equal(output.includes('synthetic-'), false);
    const authority = JSON.parse(opts.inputs.at(-1));
    assert.equal(authority.session_id, 'test');
    assert.equal(authority.role, 'implement');
    assert.equal(authority.project, root);
    assert.equal(authority.path, 'out.md');
    assert.equal(authority.claim_token, 'synthetic-claim');
  }
  await assert.rejects(tools.cortex_ia_doc_convert.execute({ file_path: 'input.txt', output_path: '../out.md' }, context), /LEASE_CHECK_FAILED/);
});

test('native hook protects utility destinations but permits inline reading', async () => {
  const opts = options(['out.md'], path.join(root, 'linked'));
  const hook = (await (await loadPluginFile(guardPath, opts)).instantiate({ directory: root }))['tool.execute.before'];
  await hook({ tool: 'cortex_ia_doc_convert', sessionID: 'test' }, { args: { file_path: 'input.txt' } });
  assert.equal(opts.calls.length, 0);
  for (const tool of ['cortex_ia_doc_convert', 'cortex_ia_diagram_render']) {
    await hook({ tool, sessionID: 'test' }, { args: { output_path: 'out.md' } });
    for (const output_path of ['unleased.md', '../outside.md', 'linked/out.md']) {
      await assert.rejects(hook({ tool, sessionID: 'test' }, { args: { output_path } }), /LEASE_REQUIRED|LEASE_CHECK_FAILED/);
    }
  }
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
