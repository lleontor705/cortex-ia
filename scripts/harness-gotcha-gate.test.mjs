import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { createIsolatedSandbox, transpileTS } from './harness-plugin-loader.mjs';

const source = transpileTS(fs.readFileSync('internal/assets/plugins/cortex-work.ts', 'utf8'));
const schema = new Proxy(() => schema, { get: () => schema });
const context = { agent: 'implement', sessionID: 'controller', directory: '/isolated/work' };

// Virtualized cortex-ia CLI. `approvals` and `transition` responses are
// programmable per scenario so the gate can be exercised without a store.
async function harness(options = {}) {
  const calls = [];
  const sandbox = createIsolatedSandbox({
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    childProcess: { execFileSync: (file, args, opts) => {
      calls.push({ file, args: Array.from(args), input: opts?.input });
      const sub = args[1];
      if (args[0] === 'version') return '';
      if (sub === 'claim') return JSON.stringify({ claim_token: 'private-claim', reserved_files: [
        { path: 'a.go', lease_token: 'private-a' }] });
      if (sub === 'status') return JSON.stringify({ status: 'in_progress', claim: {
        owner: 'opencode-session:controller', expires_at: new Date(Date.now() + 900000).toISOString() } });
      if (sub === 'approvals') {
        if (options.approvals === 'throw') throw new Error('approvals lookup exploded');
        return options.approvals === undefined ? 'null' : options.approvals;
      }
      if (sub === 'transition') {
        if (options.transition === 'throw') throw new Error('transition rejected by CLI');
        const value = key => args[args.indexOf(key) + 1];
        return JSON.stringify({ task_id: 'task', status: value('--to'), leases: [] });
      }
      return '{}';
    } },
    globals: options.globals
  });
  vm.runInContext(source, sandbox);
  const definition = sandbox.exports.CortexDelegationBridge;
  const plugin = await (typeof definition === 'function' ? definition({}) : definition.setup({}));
  await plugin.tool.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context);
  return {
    plugin, calls,
    transitionCalls: () => calls.filter(c => c.args[1] === 'transition'),
    approvalsCalls: () => calls.filter(c => c.args[1] === 'approvals'),
  };
}

const FAIL_HISTORY = JSON.stringify([{ task_id: 'task', verdict: 'FAIL', reviewer: 'reviewer', created_at: '2026-01-01T00:00:00Z' }]);
const PASS_HISTORY = JSON.stringify([{ task_id: 'task', verdict: 'PASS', reviewer: 'reviewer', created_at: '2026-01-01T00:00:00Z' }]);

test('TestREQ_GOTCHA_001 FAIL history without gotcha evidence rejects before any transition call', async () => {
  const h = await harness({ approvals: FAIL_HISTORY });
  await assert.rejects(
    h.plugin.tool.cortex_ia_work_transition.execute(
      { task_id: 'task', to: 'in_review', verdict: 'PASS', evidence_refs: ['go test ./...'] }, context),
    /GOTCHA_EVIDENCE_REQUIRED/
  );
  assert.equal(h.transitionCalls().length, 0);
  assert.equal(h.approvalsCalls().length, 1);
  await h.plugin.dispose();
});

test('TestREQ_GOTCHA_002 FAIL history with a matching gotcha ref proceeds through the authorized CLI path', async () => {
  const h = await harness({ approvals: FAIL_HISTORY });
  const output = await h.plugin.tool.cortex_ia_work_transition.execute(
    { task_id: 'task', to: 'in_review', verdict: 'PASS', evidence_refs: ['gotchas/task', 'go test ./...'] }, context);
  assert.equal(JSON.parse(output).status, 'in_review');
  assert.equal(h.transitionCalls().length, 1);
  const call = h.transitionCalls()[0];
  assert.equal(call.args[call.args.indexOf('--evidence-ref') + 1], 'gotchas/task');
  assert.equal(call.args.includes('private-claim'), false);
  assert.equal(call.input, 'private-claim');
  await h.plugin.dispose();
});

test('TestREQ_GOTCHA_003 zero-FAIL histories are unaffected with or without evidence refs', async () => {
  for (const approvals of ['null', '[]', PASS_HISTORY, '{}']) {
    const h = await harness({ approvals });
    const output = await h.plugin.tool.cortex_ia_work_transition.execute({ task_id: 'task', to: 'in_review' }, context);
    assert.equal(JSON.parse(output).status, 'in_review', approvals);
    assert.equal(h.transitionCalls().length, 1, approvals);
    await h.plugin.dispose();
  }
});

test('TestREQ_GOTCHA_004 unverifiable approval history fails closed with zero transition calls', async () => {
  for (const approvals of ['throw', 'not-valid-json']) {
    const h = await harness({ approvals });
    await assert.rejects(
      h.plugin.tool.cortex_ia_work_transition.execute(
        { task_id: 'task', to: 'in_review', evidence_refs: ['gotchas/task'] }, context),
      /GOTCHA_EVIDENCE_UNVERIFIED/, approvals
    );
    assert.equal(h.transitionCalls().length, 0, approvals);
    await h.plugin.dispose();
  }
});

test('TestREQ_GOTCHA_005 malformed gotcha ref formats are rejected under FAIL history', async () => {
  for (const ref of ['gotchas/other-task', 'gotcha/task', 'gotchas/task/extra', 'gotchas/', ' gotchas/task', 'gotchas/task ']) {
    const h = await harness({ approvals: FAIL_HISTORY });
    await assert.rejects(
      h.plugin.tool.cortex_ia_work_transition.execute(
        { task_id: 'task', to: 'in_review', evidence_refs: [ref] }, context),
      /GOTCHA_EVIDENCE_REQUIRED/, JSON.stringify(ref)
    );
    assert.equal(h.transitionCalls().length, 0, JSON.stringify(ref));
    await h.plugin.dispose();
  }
});

test('TestREQ_GOTCHA_006 blocked_reason enum forwards as --blocked-reason on blocked transitions', async () => {
  const h = await harness();
  await h.plugin.tool.cortex_ia_work_transition.execute(
    { task_id: 'task', to: 'blocked', blocked_reason: 'authority_expired', summary: 'claim expired' }, context);
  assert.equal(h.transitionCalls().length, 1);
  const args = h.transitionCalls()[0].args;
  assert.equal(args[args.indexOf('--blocked-reason') + 1], 'authority_expired');
  assert.equal(h.approvalsCalls().length, 0);
  await h.plugin.dispose();
});
