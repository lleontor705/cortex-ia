import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { createIsolatedSandbox, transpileTS } from './harness-plugin-loader.mjs';

const source = transpileTS(fs.readFileSync('internal/assets/plugins/cortex-work.ts', 'utf8'));
const schema = new Proxy(() => schema, { get: () => schema });
const context = { agent: 'implement', sessionID: 'controller', directory: '/isolated/work' };
async function harness(variant = {}) {
  let now = Date.parse('2026-01-01T00:00:00Z'), next = 0, active = 0, maxActive = 0;
  const timers = new Map(), calls = [], statusCalls = [];
  const controls = { status: async () => ({ data: { controller: { type: 'busy' } } }), release: true, renew: true };
  const sandbox = createIsolatedSandbox({
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    childProcess: { execFileSync: (file, args, opts) => {
      calls.push({ file, args: Array.from(args), input: opts?.input, timeout: opts?.timeout });
      if (args[0] === 'version') return '';
      if (args[1] === 'claim') return JSON.stringify({ claim_token: 'private-claim', reserved_files: [
        { path: 'a.go', lease_token: 'private-a' }, { path: 'b.go', lease_token: 'private-b' }] });
      if (args[1] === 'status') return JSON.stringify({ status: 'in_progress', claim: {
        owner: 'opencode-session:controller', expires_at: new Date(now + 900000).toISOString() } });
      if (args[1] === 'controller-renew') {
        if (!controls.renew) throw new Error('expired claim');
        return JSON.stringify({ task_id: 'task', owner: 'opencode-session:controller', lease_count: 2,
          expires_at: new Date(now + 900000).toISOString() });
      }
      if (args[1] === 'release-all') {
        if (controls.release === 'throw') throw new Error('release failed');
        return JSON.stringify(controls.release ? { task_id: 'task', released_all: true } : { task_id: 'other', released_all: true });
      }
      if (args[1] === 'transition') return JSON.stringify({ task_id: 'task', status: 'in_review', leases: [] });
      return '{}';
    } }
  });
  sandbox.Date = class extends Date { constructor(...args) { super(...(args.length ? args : [now])); } static now() { return now; } };
  sandbox.setTimeout = (callback, delay) => { const id = ++next; timers.set(id, { at: now + delay, callback }); return id; };
  sandbox.clearTimeout = id => timers.delete(id);
  vm.runInContext(source, sandbox);
  // variant.probe === false reproduces an OpenCode v2 plugin context: no ctx.client
  // and therefore no session status RPC for the claim heartbeat.
  const definition = sandbox.exports.CortexDelegationBridge;
  const ctx = variant.probe === false ? {} : { client: { session: { status: async options => {
    statusCalls.push(options); active++; maxActive = Math.max(maxActive, active);
    try { return await controls.status(); } finally { active--; }
  } } } };
  const plugin = await (typeof definition === 'function' ? definition(ctx) : definition.setup(ctx));
  const flush = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); };
  const advance = async ms => {
    const target = now + ms;
    for (;;) {
      const entry = [...timers].filter(([, t]) => t.at <= target).sort((a, b) => a[1].at - b[1].at)[0];
      if (!entry) break;
      now = entry[1].at; timers.delete(entry[0]); void entry[1].callback(); await flush();
    }
    now = target; await flush();
  };
  const event = pending => plugin.event({ event: pending });
  for (const pending of variant.preClaimEvents || []) await event(pending);
  const claim = await plugin.tool.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go', 'b.go'] }, context);
  assert.equal(claim.includes('private-'), false);
  return { plugin, calls, controls, advance, flush, timers, statusCalls, claim: JSON.parse(claim), maxActive: () => maxActive,
    renewals: () => calls.filter(c => c.args[1] === 'controller-renew'),
    event };
}

test('fresh host status renews complete authority once and keeps tokens only in stdin', async () => {
  const h = await harness();
  await h.advance(30000);
  assert.equal(h.renewals().length, 1);
  assert.equal(h.statusCalls[0].query.directory, context.directory);
  assert.ok(h.statusCalls[0].signal);
  assert.equal(h.renewals()[0].args.includes('private-claim'), false);
  assert.deepEqual(JSON.parse(h.renewals()[0].input), { claim_token: 'private-claim', leases: { 'a.go': 'private-a', 'b.go': 'private-b' } });
  assert.equal(h.renewals()[0].timeout, 10000);
  assert.equal(h.claim.maintenance.active, true);
  await h.plugin.dispose();
  assert.equal(h.timers.size, 0);
});

test('busy/retry and duplicate or unrelated events cannot renew an orphan indefinitely', async () => {
  const h = await harness();
  h.controls.status = async () => ({ data: { controller: { type: 'retry', attempt: 1, next: 9999999999999 } } });
  const part = { id: 'part', sessionID: 'controller', type: 'text', text: 'same' };
  await h.event({ type: 'message.part.updated', properties: { part } });
  for (let i = 0; i < 20; i++) {
    await h.advance(60000);
    await h.event({ type: 'message.part.updated', properties: { part } });
    await h.event({ type: 'message.updated', properties: { info: { id: `foreign-${i}`, sessionID: 'other' } } });
  }
  const count = h.renewals().length;
  assert.equal(count, 29);
  await h.advance(900000);
  assert.equal(h.renewals().length, count);
  assert.equal(h.maxActive(), 1);
  await h.plugin.dispose();
});

test('changed host activity supports long active runs beyond the stale window', async () => {
  const h = await harness();
  for (let i = 0; i < 40; i++) {
    await h.advance(60000);
    await h.event({ type: 'message.part.updated', properties: { part: { id: 'part', sessionID: 'controller', text: `progress-${i}` } } });
  }
  assert.equal(h.renewals().length, 80);
  await h.plugin.dispose();
});

test('idle, unknown, error and status timeout stop with no overlapping or resurrected renewal', async () => {
  for (const response of [{ data: { controller: { type: 'idle' } } }, { data: {} }, { error: 'offline' }, null]) {
    const h = await harness();
    let resolve;
    h.controls.status = response ? async () => response : () => new Promise(r => { resolve = r; });
    await h.advance(35000);
    if (resolve) { resolve({ data: { controller: { type: 'busy' } } }); await h.flush(); }
    await h.advance(120000);
    assert.equal(h.renewals().length, 0);
    assert.equal(h.statusCalls.length, 1);
    await h.plugin.dispose();
  }
});

test('late status response after terminal event, dispose, or delivery never renews', async () => {
  for (const stop of ['session.idle', 'session.deleted', 'session.error', 'server.instance.disposed', 'dispose', 'delivery']) {
    const h = await harness();
    let resolve;
    h.controls.status = () => new Promise(r => { resolve = r; });
    await h.advance(30000);
    if (stop === 'dispose') await h.plugin.dispose();
    else if (stop === 'delivery') await h.plugin.tool.cortex_ia_work_transition.execute({ task_id: 'task', to: 'in_review' }, context);
    else await h.event({ type: stop, properties: { sessionID: 'controller', info: { id: 'controller' } } });
    resolve({ data: { controller: { type: 'busy' } } }); await h.flush();
    await h.advance(120000);
    assert.equal(h.renewals().length, 0, stop);
    await h.plugin.dispose();
  }
});

test('release-all sends one transaction and retains reservations after failure or wrong acknowledgement', async () => {
  for (const failure of [false, 'throw']) {
    const h = await harness(); h.controls.release = failure;
    await assert.rejects(h.plugin.tool.cortex_ia_work_release_all.execute({ task_id: 'task' }, context));
    h.controls.release = true;
    const receipt = JSON.parse(await h.plugin.tool.cortex_ia_work_release_all.execute({ task_id: 'task' }, context));
    assert.deepEqual(receipt.released, ['a.go', 'b.go']);
    assert.equal(h.calls.filter(c => c.args[1] === 'release-all').length, 2);
    assert.equal(h.calls.filter(c => c.args[1] === 'release').length, 0);
    assert.equal(receipt.released_all, true);
    await h.plugin.dispose();
  }
});

test('failed renewal stops, executable discovery is cached and invalidated on error', async () => {
  const h = await harness();
  await h.advance(30000);
  assert.equal(h.calls.filter(c => c.args[0] === 'version').length, 1);
  h.controls.renew = false;
  await h.advance(30000);
  const count = h.renewals().length;
  h.controls.renew = true;
  await h.advance(120000);
  assert.equal(h.renewals().length, count);
  await h.plugin.tool.cortex_ia_work_release_all.execute({ task_id: 'task' }, context);
  assert.ok(h.calls.filter(c => c.args[0] === 'version').length >= 2);
  await h.plugin.dispose();
});

test('OpenCode v2 context without a status RPC keeps the heartbeat renewing', async () => {
  const h = await harness({ probe: false });
  assert.equal(h.claim.maintenance.active, true);
  assert.equal(h.claim.maintenance.reason, undefined);
  await h.event({ type: 'session.status', properties: { sessionID: 'controller', status: { type: 'busy' } } });
  await h.advance(30000);
  assert.equal(h.renewals().length, 1);
  assert.equal(h.statusCalls.length, 0);
  assert.equal(h.renewals()[0].args.includes('private-claim'), false);
  assert.deepEqual(JSON.parse(h.renewals()[0].input), { claim_token: 'private-claim', leases: { 'a.go': 'private-a', 'b.go': 'private-b' } });
  await h.plugin.dispose();
  assert.equal(h.timers.size, 0);
});

test('event-tracked idle and terminal events stop the fallback heartbeat', async () => {
  const stale = await harness({ probe: false, preClaimEvents: [
    { type: 'session.status', properties: { sessionID: 'controller', status: { type: 'idle' } } }] });
  assert.equal(stale.claim.maintenance.active, true);
  await stale.advance(120000);
  assert.equal(stale.renewals().length, 0);
  await stale.plugin.dispose();

  const h = await harness({ probe: false });
  await h.advance(30000);
  assert.equal(h.renewals().length, 1);
  await h.event({ type: 'session.idle', properties: { sessionID: 'controller' } });
  await h.advance(120000);
  assert.equal(h.renewals().length, 1);
  await h.plugin.dispose();
});

test('fallback heartbeat still dies at the stale-progress window without host activity', async () => {
  const h = await harness({ probe: false });
  await h.advance(900000);
  assert.equal(h.renewals().length, 29);
  await h.advance(600000);
  assert.equal(h.renewals().length, 29);
  await h.plugin.dispose();
});

function durationMs(value) {
  const match = /^(\d+)(ms|s|m|h)$/.exec(String(value));
  if (!match) return NaN;
  return Number(match[1]) * { ms: 1, s: 1000, m: 60000, h: 3600000 }[match[2]];
}

test('TestREQ_WAUTH_001 claim TTL is decoupled from the stale-progress window', async () => {
  const h = await harness();
  const policy = h.claim.maintenance;
  assert.notEqual(policy.ttl, '15m');
  assert.equal(policy.ttl, '30m');
  assert.equal(policy.stale_progress_ms, 900000);
  assert.equal(durationMs(policy.ttl), 1800000);
  assert.notEqual(durationMs(policy.ttl), policy.stale_progress_ms);
  const policyLine = source.split('\n').find(line => line.includes('stale_progress_ms')) || '';
  assert.equal(/ttl\s*:[^,}\n]*stale_progress_ms/.test(policyLine), false);
  await h.plugin.dispose();
});

test('TestREQ_WAUTH_002 owning implement session recovers its own task through the scoped CLI', async () => {
  const h = await harness();
  const receipt = await h.plugin.tool.cortex_ia_work_recover_own.execute({ task_id: 'task' }, context);
  const recoverCalls = h.calls.filter(c => c.args[0] === 'work' && c.args[1] === 'recover');
  assert.equal(recoverCalls.length, 1);
  assert.deepEqual(recoverCalls[0].args, ['work', 'recover', '--task', 'task', '--owner', 'opencode-session:controller']);
  assert.equal(recoverCalls[0].args.includes('private-claim'), false);
  assert.equal(typeof receipt, 'string');
  await h.plugin.dispose();
});

test('TestREQ_WAUTH_002 a session without the authority fails closed with no CLI recovery call', async () => {
  const h = await harness();
  await assert.rejects(
    h.plugin.tool.cortex_ia_work_recover_own.execute({ task_id: 'task' }, { ...context, sessionID: 'foreign' }),
    /scoped recover denied/
  );
  await assert.rejects(
    h.plugin.tool.cortex_ia_work_recover_own.execute({ task_id: 'unclaimed' }, context),
    /scoped recover denied/
  );
  assert.equal(h.calls.filter(c => c.args[0] === 'work' && c.args[1] === 'recover').length, 0);
  await h.plugin.dispose();
});

test('TestREQ_WAUTH_002 non-implement roles are rejected by the mutation gate', async () => {
  const h = await harness();
  for (const agent of ['orchestrator', 'reviewer', 'planner', 'investigate', 'discovery']) {
    await assert.rejects(
      h.plugin.tool.cortex_ia_work_recover_own.execute({ task_id: 'task' }, { ...context, agent }),
      /BRIDGE_ROLE_DENIED/
    );
  }
  assert.equal(h.calls.filter(c => c.args[0] === 'work' && c.args[1] === 'recover').length, 0);
  await h.plugin.dispose();
});

test('TestREQ_WAUTH_002 successful recovery stops maintenance and drops the stale handle', async () => {
  const h = await harness();
  assert.equal(h.claim.maintenance.active, true);
  await h.plugin.tool.cortex_ia_work_recover_own.execute({ task_id: 'task' }, context);
  const status = JSON.parse(await h.plugin.tool.cortex_ia_work_status.execute({ task_id: 'task' }, context));
  assert.equal(status.bridge_authority.handle_present, false);
  assert.equal(status.bridge_authority.maintenance, undefined);
  assert.equal(h.timers.size, 0);
  await h.plugin.dispose();
});
