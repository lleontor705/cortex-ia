import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const context = { agent: 'implement', sessionID: 'submission-test', directory: '/isolated/work' };

async function harness() {
  const calls = [];
  let failure = false;
  const mod = await loadPluginFile('internal/assets/plugins/cortex-work.ts', {
    env: { HOME: home, USERPROFILE: home },
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    fs: { existsSync: p => p === executable, readFileSync: () => '', mkdirSync: () => {}, writeFileSync: () => {} },
    childProcess: { execFileSync: (_file, args, opts) => {
      calls.push({ args: Array.from(args), input: opts?.input });
      if (args[1] === 'status') return JSON.stringify({ status: 'in_progress', claim: { owner: 'opencode-session:submission-test', expires_at: new Date(Date.now() + 60000).toISOString() } });
      if (args[1] === 'claim') return JSON.stringify({ claim_token: 'synthetic-secret', reserved_files: [{ path: 'a.go', lease_token: 'synthetic-lease' }] });
      if (args[1] === 'transition') {
        if (failure) throw new Error('synthetic transaction failure');
        const value = key => args[args.indexOf(key) + 1];
        return JSON.stringify({ task_id: 'task', status: value('--to'), revision: 3,
          submission: { summary: value('--summary'), verdict: value('--verdict'), evidence_refs: [value('--evidence-ref')], changed_files: [value('--changed-file')] } });
      }
      return '{}';
    } },
  });
  return { tools: (await mod.instantiate({ client: {} })).tool, calls, fail: value => { failure = value; } };
}

test('actual bridge forwards every receipt field and trusts only atomic service delivery', async () => {
  const h = await harness();
  await h.tools.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context);
  const fields = { summary: 'checked exact bytes', verdict: 'PASS', evidence_refs: ['go test synthetic'], changed_files: ['a.go'] };
  const output = await h.tools.cortex_ia_work_transition.execute({ task_id: 'task', to: 'in_review', revision: 2, ...fields }, context);
  assert.deepEqual(JSON.parse(output).submission, fields);
  const call = h.calls.find(x => x.args[1] === 'transition');
  assert.equal(call.input, 'synthetic-secret');
  assert.equal(call.args.includes('synthetic-secret'), false);
  assert.equal(output.includes('synthetic-secret'), false);
  assert.equal(h.calls.some(x => x.args[1] === 'release-all' || x.args[1] === 'approve'), false);
});

test('failed service delivery keeps local claim usable for explicit retry', async () => {
  const h = await harness();
  await h.tools.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context);
  h.fail(true);
  await assert.rejects(h.tools.cortex_ia_work_transition.execute({ task_id: 'task', to: 'blocked' }, context), /synthetic transaction failure/);
  h.fail(false);
  await h.tools.cortex_ia_work_transition.execute({ task_id: 'task', to: 'blocked' }, context);
  assert.equal(h.calls.filter(x => x.args[1] === 'transition').length, 2);
});
