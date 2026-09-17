import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { loadPluginSource } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
async function harness(initial) {
  let job = initial;
  const closed = [];
  const calls = [];
  const source = fs.readFileSync('internal/assets/plugins/herdr-bridge.ts', 'utf8') +
    '\nObject.assign(CortexDelegationBridge, { rememberJobPane });';
  const mod = await loadPluginSource(source, {
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    fs: { existsSync: () => true, readFileSync: () => '', mkdirSync: () => {}, writeFileSync: () => {} },
    childProcess: { execFileSync: (_file, args) => {
      calls.push(args);
      if (args[0] === 'pane' && args[1] === 'close') closed.push(args[2]);
      if (args[0] === 'delegate' && args[1] === 'result') return JSON.stringify({ status: job.status });
      return JSON.stringify(job);
    } },
  });
  const plugin = await mod.instantiate({ client: {} });
  mod.exports.rememberJobPane('job', 'pane');
  calls.length = 0;
  return { tools: plugin.tool, closed, calls, set: value => { job = value; } };
}

test('cancellation stays pending and preserves pane until worker acknowledgement', async () => {
  const h = await harness({ job_id: 'job', status: 'running', cancellation_requested: true });
  const pending = JSON.parse(await h.tools.cortex_ia_delegation_cancel.execute({ job_id: 'job' },
    { agent: 'orchestrator', sessionID: 'session' }));
  assert.equal(pending.status, 'running');
  assert.equal(pending.cancellation_requested, true);
  assert.equal(h.closed.length, 0);
  const result = JSON.parse(await h.tools.cortex_ia_delegation_result.execute({ job_id: 'job' }));
  assert.equal(result.completed, false);
  assert.equal(result.cancellation_requested, true);
  assert.equal(h.closed.length, 0);
  h.set({ job_id: 'job', status: 'cancelled' });
  const confirmed = JSON.parse(await h.tools.cortex_ia_delegation_cancel.execute({ job_id: 'job' },
    { agent: 'orchestrator', sessionID: 'session' }));
  assert.equal(confirmed.status, 'cancelled');
  assert.deepEqual(h.closed, ['pane']);
});

test('lost cancellation reports reconciliation without closing resources or claiming completion', async () => {
  const h = await harness({ job_id: 'job', status: 'lost', cancellation_requested: true, reconciliation_required: true });
  for (const name of ['cortex_ia_delegation_wait', 'cortex_ia_delegation_result']) {
    const result = JSON.parse(await h.tools[name].execute({ job_id: 'job', timeout_seconds: 1 }));
    assert.equal(result.completed, false);
    assert.equal(result.action, 'RECONCILE_TERMINATION');
    assert.equal(h.closed.length, 0);
  }
});

test('unknown result state cannot close a possibly active pane', async () => {
  const h = await harness({});
  await h.tools.cortex_ia_delegation_result.execute({ job_id: 'job' });
  assert.equal(h.closed.length, 0);
});

test('reconciliation requires the orchestrator host identity and forwards only intent', async () => {
  const h = await harness({ job_id: 'job', status: 'lost', reconciled: true });
  const tool = h.tools.cortex_ia_delegation_reconcile;
  const args = { job_id: 'job', reason: ' inspected boot ', session_id: 'spoofed', boot_at: 'spoofed' };
  for (const context of [undefined, {agent:'implement',sessionID:'s'}, {agent:'orchestrator'}]) {
    await assert.rejects(tool.execute(args, context), /BRIDGE_ROLE_DENIED/);
  }
  for (const reason of ['', ' ', 'x'.repeat(1025)]) {
    await assert.rejects(tool.execute({...args,reason}, {agent:'orchestrator',sessionID:'host_session'}), /reason/);
  }
  assert.equal(h.calls.length, 0);
  const receipt = JSON.parse(await tool.execute(args, {agent:'orchestrator',sessionID:'host_session'}));
  assert.equal(receipt.status, 'lost');
  assert.equal(receipt.reconciled, true);
  assert.deepEqual(Array.from(h.calls.find(args => args[0] === 'delegate')), ['delegate','reconcile','job','--reason','inspected boot','--session-id','host_session']);
  assert.deepEqual(h.closed, []);
});

test('reconciled lost jobs remain failures and never close reused panes', async () => {
  const h = await harness({ job_id: 'job', status: 'lost', termination_reconciled: true });
  for (const name of ['cortex_ia_delegation_wait','cortex_ia_delegation_result']) {
    const result = JSON.parse(await h.tools[name].execute({job_id:'job',timeout_seconds:1}));
    assert.equal(result.status, 'lost');
    assert.equal(result.completed, false);
    assert.equal(result.action, 'EXPLICIT_RETRY');
    assert.deepEqual(h.closed, []);
  }
});
