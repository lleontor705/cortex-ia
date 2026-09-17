import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
const workspace = path.resolve('/isolated/work');
const context = { agent: 'implement', sessionID: 'child', directory: workspace };

async function harness() {
  const requests = [], calls = [], spawned = [];
  const files = new Map();
  const mod = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    env: { HOME: '/isolated/home', USERPROFILE: '/isolated/home' },
    mockPluginSDK: { tool: Object.assign(value => value, { schema }) },
    fs: {
      existsSync: () => false, readFileSync: () => '', mkdirSync: () => {}, rmSync: () => {},
      mkdtempSync: () => path.resolve('/isolated/request'),
      writeFileSync: (file, data) => { files.set(file, data); if (path.basename(file) === 'request.json') requests.push(JSON.parse(data)); },
    },
    childProcess: {
      execSync: () => { throw new Error('No optional terminal installed'); },
      execFileSync: (file, raw) => {
        const args = Array.from(raw); calls.push(args);
        if (file === 'herdr') throw new Error('No Herdr installed');
        if (args[0] === 'version') return '{}';
        if (args[0] === 'delegate' && args[1] === 'policy') return JSON.stringify({ schema_version: 1,
          role: args[3], external_enabled: true, reason: 'external_enabled' });
        if (args[0] === 'work' && args[1] === 'status') return '{"task_id":"work-real","status":"in_progress"}';
        if (args[0] === 'delegate' && args[1] === 'create') {
          assert.ok(files.has(args[3]), 'request must exist before acceptance');
          return '{"job_id":"synthetic-job","status":"queued"}';
        }
        throw new Error('Unexpected subprocess request: ' + args.join(' '));
      },
      spawn: (...args) => {
        spawned.push(args);
        const child = { once: (event, callback) => { if (event === 'spawn') queueMicrotask(callback); return child; }, unref() {} };
        return child;
      },
    },
  });
  const plugin = await mod.instantiate({ client: { session: { get: async ({ path: p }) => ({ data: {
    id: p.id, ...(p.id === 'child' ? { parentID: 'root' } : {}),
  } }) } } });
  return { plugin, requests, calls, spawned, tool: plugin.tool.cortex_ia_delegate_start };
}

const request = { role: 'implement', task_id: 'work-real', objective: 'Fix the scoped synthetic file',
  workspace_strategy: 'current_workspace', allowed_files: ['src/example.go'], acceptance_checks: ['go test ./synthetic'] };

test('actual bridge forwards canonical workload and exact authority scope before mocked acceptance', async () => {
  for (const policy of [undefined, 'strict', 'flexible', 'unbounded']) {
    const h = await harness();
    try {
      const result = JSON.parse(await h.tool.execute({ ...request, workload_policy: policy }, context));
      assert.equal(result.delegated, true);
      assert.equal(result.execution_mode, 'direct_cli');
      assert.equal(h.requests.length, 1);
      const sent = h.requests[0];
      assert.equal(sent.workload_policy, policy ?? 'flexible');
      assert.equal(sent.role, request.role);
      assert.equal(sent.task_id, request.task_id);
      assert.equal(sent.workspace, workspace);
      assert.deepEqual(sent.allowed_files, request.allowed_files);
      assert.equal(sent.opencode_session_id, 'child');
      assert.equal(sent.opencode_root_session_id, 'root');
      assert.equal(sent.model, undefined, 'bridge must not override the user model');
      assert.equal(h.spawned.length, 1);
      assert.match(h.tool.description, /CORTEX_IA_AGY_AUTH=gemini.*GEMINI_API_KEY/);
    } finally { await h.plugin.dispose(); }
  }
});

test('invalid workload rejects before policy, request persistence or job acceptance', async () => {
  const h = await harness();
  try {
    for (const policy of [null, '', 'unknown', 1, false, {}]) {
      const result = JSON.parse(await h.tool.execute({ ...request, workload_policy: policy }, context));
      assert.equal(result.delegated, false);
      assert.equal(result.error.code, 'DELEGATION_REQUEST_INVALID');
    }
    assert.equal(h.calls.length, 0);
    assert.equal(h.requests.length, 0);
    assert.equal(h.spawned.length, 0);
    await assert.rejects(h.tool.execute({ ...request, role: 'reviewer' }, context), /BRIDGE_ROLE_DENIED/);
  } finally { await h.plugin.dispose(); }
});
