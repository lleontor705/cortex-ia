import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

// Bridge-seam incident oracle for the reconcile initiative. It replays the
// obs-176/obs-184 chain entirely against a mocked cortex-ia CLI: the orchestrator
// force-releases an orphaned claim, retries the blocked task, and a stale in-memory
// handle no longer suppresses a fresh claim. No real CLI, no durable state, no tokens.

const bridgePath = 'internal/assets/plugins/cortex-work.ts';
const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const leasedPath = 'scripts/harness-reconcile-incidentsmoke.test.mjs';
const taskID = 'task-incident';
const reason = 'orphaned token: controller died post-quoting failure';
const schema = new Proxy(() => schema, { get: () => schema });
const sdk = { tool: Object.assign(definition => definition, { schema }) };

const reconcileReceipt = JSON.stringify({
  decision: 'released', task_id: taskID, from_status: 'in_progress', to_status: 'blocked',
  revision_before: 2, revision_after: 3, released_leases: ['src/a.go', 'src/b.go'],
});
const retryReceipt = JSON.stringify({ task_id: taskID, status: 'ready', revision: 4 });

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
          const paths = args.flatMap((value, index) => value === '--path' ? [args[index + 1]] : []);
          return JSON.stringify({ claim_token: 'synthetic-claim', revision: 5, reserved_files: paths.map(p => ({ path: p, lease_token: 'synthetic-lease' })) });
        }
        if (args[1] === 'status') return JSON.stringify({ status: control.status ?? 'in_progress', claim: control.claim });
        if (args[1] === 'reconcile') return control.reconcile ?? reconcileReceipt;
        if (args[1] === 'retry') return control.retry ?? retryReceipt;
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

const callsOf = (opts, verb) => opts.calls.filter(args => args[1] === verb);
const inactiveFlag = args => args[args.indexOf('--owner-session-inactive') + 1];

test('orchestrator reconcile releases the orphan and retry reopens the task', async () => {
  const { opts, bridge } = await bridgeFor({ claim: { owner: 'opencode-session:dead-owner', expires_at: new Date(Date.now() + 60000).toISOString() } });
  await bridge.event({ type: 'session.created', sessionID: 'dead-owner' });
  const context = { agent: 'orchestrator', sessionID: 'orch-sess', directory: root };

  const receipt = await bridge.tool.cortex_ia_work_reconcile.execute(
    { task_id: taskID, reason, revision: 2, session: 'caller-supplied', sessionID: 'caller-supplied' }, context);
  assert.equal(receipt, reconcileReceipt);
  assert.deepEqual(callsOf(opts, 'reconcile')[0], ['work', 'reconcile', taskID,
    '--reason', reason, '--session', 'orch-sess', '--revision', '2', '--owner-session-inactive', 'true']);

  assert.equal(await bridge.tool.cortex_ia_work_retry.execute({ task_id: taskID, revision: 3 }, context), retryReceipt);
  assert.deepEqual(callsOf(opts, 'retry')[0], ['work', 'retry', taskID, '--revision', '3']);

  const forwarded = JSON.stringify(opts.calls);
  assert.equal(forwarded.includes('caller-supplied'), false);
  assert.equal(forwarded.includes('synthetic-'), false);
  assert.equal(receipt.includes('synthetic-'), false);
});

test('a durable-void retained handle no longer blocks the fresh claim', async () => {
  const control = { claim: { owner: 'opencode-session:sess-a', expires_at: new Date(Date.now() + 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'implement', sessionID: 'sess-a', directory: root };
  await bridge.tool.cortex_ia_work_claim.execute({ task_id: taskID, paths: [leasedPath] }, context);

  // The owner died; durable recovery voided the claim while this bridge still
  // retained the in-memory handle. Deference must drop it before the retry.
  control.status = 'ready';
  control.claim = null;
  const fresh = await bridge.tool.cortex_ia_work_claim.execute({ task_id: taskID, paths: [leasedPath] }, context);
  assert.equal(callsOf(opts, 'claim').length, 2);
  assert.equal(JSON.parse(fresh).maintenance.active, false);
});

test('every non-orchestrator role is denied before any CLI invocation', async () => {
  const { opts, bridge } = await bridgeFor();
  for (const agent of ['implement', 'planner', 'investigate', 'discovery', 'reviewer']) {
    await assert.rejects(
      bridge.tool.cortex_ia_work_reconcile.execute({ task_id: taskID, reason, revision: 2 },
        { agent, sessionID: 'sess-1', directory: root }),
      /BRIDGE_ROLE_DENIED/
    );
  }
  assert.equal(opts.calls.length, 0);
});

test('owner-inactivity evidence stays host-attested and fail-closed', async () => {
  const control = { claim: { owner: 'opencode-session:dead-owner', expires_at: new Date(Date.now() + 60000).toISOString() } };
  const { opts, bridge } = await bridgeFor(control);
  const context = { agent: 'orchestrator', sessionID: 'orch-sess', directory: root };
  const args = { task_id: taskID, reason, revision: 2 };

  // A parentless host session the bridge saw active and that is now idle is evidence.
  await bridge.event({ type: 'session.created', sessionID: 'dead-owner' });
  await bridge.tool.cortex_ia_work_reconcile.execute(args, context);
  assert.equal(inactiveFlag(callsOf(opts, 'reconcile').at(-1)), 'true');

  // A tracked-but-still-active subagent session must never weaken protection.
  await bridge.event({ type: 'session.created', sessionID: 'live-owner', properties: { info: { id: 'live-owner', parentID: 'parent-1' } } });
  control.claim = { owner: 'opencode-session:live-owner', expires_at: new Date(Date.now() + 60000).toISOString() };
  await bridge.tool.cortex_ia_work_reconcile.execute(args, context);
  assert.equal(inactiveFlag(callsOf(opts, 'reconcile').at(-1)), 'false');
});
