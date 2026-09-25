import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const bridgePath = 'internal/assets/plugins/cortex-work.ts';
const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const leasedPath = 'scripts/harness-work-reconcile.test.mjs';
const schema = new Proxy(() => schema, { get: () => schema });
const sdk = { tool: Object.assign(definition => definition, { schema }) };

function options(control = {}) {
  const calls = [];
  return {
    calls, cwd: root, env: { USERPROFILE: home, HOME: home }, mockPluginSDK: sdk,
    fs: {
      existsSync: p => p === executable, readFileSync: () => '', realpathSync: p => p,
      lstatSync: () => ({ isSymbolicLink: () => false }), mkdirSync: () => {}, writeFileSync: () => {},
    },
    childProcess: {
      execFileSync: (_file, args) => {
        calls.push(Array.from(args));
        if (args[1] === 'claim') {
          if (control.claimConflict) throw new Error('claim refused: durable live foreign owner');
          const paths = args.flatMap((value, index) => value === '--path' ? [args[index + 1]] : []);
          return JSON.stringify({ claim_token: 'synthetic-claim', reserved_files: paths.map(p => ({ path: p, lease_token: 'synthetic-lease' })) });
        }
        if (args[1] === 'status') {
          if (control.statusFails) throw new Error('durable work status unavailable');
          return JSON.stringify({ status: 'in_progress', claim: control.claim });
        }
        if (args[1] === 'reconcile') return control.receipt ?? '{"decision":"released"}';
        return '{}';
      },
    },
  };
}

async function bridgeFor(control = {}) {
  const opts = options(control);
  const bridge = await (await loadPluginFile(bridgePath, opts)).instantiate({ client: {} });
  return { opts, bridge };
}

const claimCount = opts => opts.calls.filter(args => args[1] === 'claim').length;
const inactiveFlag = args => args[args.indexOf('--owner-session-inactive') + 1];

test('a durable-void retained handle is dropped so the fresh claim proceeds', async () => {
  const control = { claim: { owner: 'opencode-session:sess-a', expires_at: new Date(Date.now() - 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'implement', sessionID: 'sess-a', directory: root };
  await bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-void', paths: [leasedPath] }, context);
  const fresh = await bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-void', paths: [leasedPath] }, context);
  assert.equal(claimCount(opts), 2);
  assert.equal(JSON.parse(fresh).maintenance.active, false);
});

test('a live durable claim owned by another session defers to durable authority', async () => {
  const control = { claim: { owner: 'opencode-session:sess-a', expires_at: new Date(Date.now() + 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'implement', sessionID: 'sess-a', directory: root };
  await bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-reassigned', paths: [leasedPath] }, context);
  // The durable claim was reassigned to a live foreign owner; the stale handle must
  // not suppress the durable refusal, so the bridge drops it and issues the fresh claim.
  control.claim = { owner: 'opencode-session:foreign', expires_at: new Date(Date.now() + 60000).toISOString() };
  control.claimConflict = true;
  await assert.rejects(
    bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-reassigned', paths: [leasedPath] }, context),
    /durable live foreign owner/
  );
  assert.equal(claimCount(opts), 2);
});

test('a genuinely held live claim still refuses a second handle', async () => {
  const control = { claim: { owner: 'opencode-session:sess-a', expires_at: new Date(Date.now() + 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'implement', sessionID: 'sess-a', directory: root };
  await bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-live', paths: [leasedPath] }, context);
  await assert.rejects(
    bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-live', paths: [leasedPath] }, context),
    /already held by this controller/
  );
  assert.equal(claimCount(opts), 1);
});

test('a failed durable probe keeps the refusal fail-closed without a new claim', async () => {
  const control = { statusFails: false };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'implement', sessionID: 'sess-a', directory: root };
  await bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-probe', paths: [leasedPath] }, context);
  control.statusFails = true;
  await assert.rejects(
    bridge.tool.cortex_ia_work_claim.execute({ task_id: 'task-probe', paths: [leasedPath] }, context),
    /WORK_STATUS_UNAVAILABLE/
  );
  assert.equal(claimCount(opts), 1);
});

test('the reconcile tool denies every non-orchestrator role before any CLI call', async () => {
  const { opts, bridge } = await bridgeFor();
  for (const agent of ['implement', 'planner', 'investigate', 'discovery', 'reviewer']) {
    await assert.rejects(
      bridge.tool.cortex_ia_work_reconcile.execute(
        { task_id: 'task-1', reason: 'orphaned claim', revision: 3 },
        { agent, sessionID: 'sess-1', directory: root }
      ),
      /BRIDGE_ROLE_DENIED/
    );
  }
  assert.equal(opts.calls.length, 0);
});

test('orchestrator reconcile binds the context session and emits the pinned argv grammar', async () => {
  const receipt = '{"decision":"released","task_id":"task-1"}';
  const control = { claim: { owner: 'opencode-session:foreign', expires_at: new Date(Date.now() + 60000).toISOString() }, receipt };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'orchestrator', sessionID: 'orch-sess', directory: root };
  const callerSupplied = { task_id: 'task-1', reason: 'orphaned claim', revision: 7, session: 'caller-supplied', sessionID: 'caller-supplied' };
  assert.equal(await bridge.tool.cortex_ia_work_reconcile.execute(callerSupplied, context), receipt);
  assert.equal(await bridge.tool.cortex_ia_work_reconcile.execute({ task_id: 'task-1', reason: 'orphaned claim', revision: 7, to: 'ready' }, context), receipt);
  const reconciles = opts.calls.filter(args => args[1] === 'reconcile');
  const expected = ['work', 'reconcile', 'task-1', '--reason', 'orphaned claim', '--session', 'orch-sess', '--revision', '7', '--owner-session-inactive', 'false'];
  assert.deepEqual(reconciles[0], expected);
  assert.deepEqual(reconciles[1], [...expected, '--to', 'ready']);
  assert.equal(JSON.stringify(opts.calls).includes('caller-supplied'), false);
  assert.equal(JSON.stringify(opts.calls).includes('synthetic-'), false);
});

test('owner-inactivity evidence is host-tracked only', async () => {
  const control = { claim: { owner: 'opencode-session:dead-owner', expires_at: new Date(Date.now() + 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'orchestrator', sessionID: 'orch-sess', directory: root };
  const args = { task_id: 'task-1', reason: 'orphaned claim', revision: 4 };
  await bridge.tool.cortex_ia_work_reconcile.execute(args, context);
  assert.equal(inactiveFlag(opts.calls.at(-1)), 'false');
  // A session the bridge only ever saw as a parentless host session is inactive evidence.
  await bridge.event({ type: 'session.created', sessionID: 'dead-owner' });
  await bridge.tool.cortex_ia_work_reconcile.execute(args, context);
  assert.equal(inactiveFlag(opts.calls.at(-1)), 'true');
  // A tracked-but-still-active subagent session must never weaken foreign-claim protection.
  await bridge.event({ type: 'session.created', sessionID: 'live-owner', properties: { info: { id: 'live-owner', parentID: 'parent-1' } } });
  control.claim = { owner: 'opencode-session:live-owner', expires_at: new Date(Date.now() + 60000).toISOString() };
  await bridge.tool.cortex_ia_work_reconcile.execute(args, context);
  assert.equal(inactiveFlag(opts.calls.at(-1)), 'false');
});
