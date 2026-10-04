import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

// Contract under test (internal/assets/plugins/cortex-task-latch.ts): the latch
// is a graded circuit breaker, not an immediate latch. Terminal failures for the
// same durable objective trip the circuit on attempt 4 (MAX_CIRCUIT_ATTEMPTS);
// before that, re-dispatch stays allowed so the orchestrator can self-heal. Read-only
// roles (investigate, reviewer, discovery, planner) never latch at all. Envelope
// identity rejection (ambiguous / duplicate-key / malformed dispatch envelopes)
// is owned by cortex-subagent-transport's dispatchInfo -> SUBAGENT_TRANSPORT_ERROR
// and is asserted there (scripts/harness-budget.test.mjs); here we only pin that
// the latch never manufactures durable identity or telemetry from bad input.

function dispatch(task = 'work-real', role = 'implement', resume = 'host-resume-1') {
  return { subagent_type: role, task_id: resume, prompt: '<minion-dispatch>' + JSON.stringify({
    contract_version: '1.0', task_id: task, role, workflow: 'sdd-full', phase: 'apply',
    objective: 'Bounded work', spec_plane: 'openspec', allowed_files: role === 'implement' ? ['one.go'] : [],
    acceptance_checks: ['check'], artifact_refs: [], max_steps: 10,
  }) + '</minion-dispatch>' };
}

async function harness() {
  const reports = [];
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', {
    virtualFiles: { '/usr/bin/cortex-ia': '' },
    childProcess: { execFileSync: (_file, args) => { reports.push([...args]); return ''; } },
  });
  const plugin = await mod.exports.CortexTaskLatchPlugin({
    directory: '/isolated/harness',
    client: { session: { get: async ({ path }) => ({ data: {
      id: path.id, ...(path.id === 'leaf' ? { parentID: 'root' } : {}),
    } }) } },
  });
  return {
    plugin, reports,
    before: args => plugin['tool.execute.before']({ tool: 'task', sessionID: 'root' }, { args }),
    after: (args, output) => plugin['tool.execute.after']({ tool: 'task', sessionID: 'root', args }, { output }),
    retry: async (task, result, sessionID = 'root') => {
      const input = { tool: 'cortex_ia_work_retry', sessionID, callID: 'retry' };
      await plugin['tool.execute.before'](input, { args: { task_id: task } });
      await plugin['tool.execute.after'](input, { output: JSON.stringify(result) });
    },
  };
}

const attempt = n => new RegExp('SUBAGENT_ATTEMPT_FAILED: Attempt ' + n);

test('circuit opens on the fourth terminal failure and reports under durable identity', async () => {
  const h = await harness(), args = dispatch();
  for (let n = 1; n <= 3; n++) await assert.rejects(h.after(args, ''), attempt(n));
  await assert.rejects(h.after(args, ''), /CORTEX_CIRCUIT_OPEN.*work-real/);

  assert.equal(h.reports.length, 1, 'only the tripped circuit reports');
  const report = h.reports[0];
  assert.equal(report[report.indexOf('--code') + 1], 'ERR_SUBAGENT_CIRCUIT_OPEN');
  assert.equal(report[report.indexOf('--task') + 1], 'work-real');
  assert.equal(h.reports.some(a => a.includes('host-resume-1')), false,
    'the host resume id must never become a durable identity');

  await assert.rejects(h.before(args), /CORTEX_CIRCUIT_OPEN.*work-real/);
  await h.retry('work-real', { task_id: 'work-real', status: 'ready', revision: 2 });
  await h.before(args);
  await h.retry('work-real', { error: 'failed' });
  await h.before(args);
  await assert.rejects(h.after(args, ''), attempt(1), 'the next failure restarts the graded cycle');
  await h.plugin.dispose();
});

test('recovery alone cannot close a circuit and successes clear it', async () => {
  const h = await harness(), args = dispatch();
  for (let n = 1; n <= 4; n++) await assert.rejects(h.after(args, ''));
  await h.plugin['tool.execute.before']({ tool: 'cortex_ia_work_recover', sessionID: 'root' }, { args: { task_id: 'work-real' } });
  await h.plugin['tool.execute.after']({ tool: 'cortex_ia_work_recover', sessionID: 'root' }, { output: '{"recovered":1}' });
  await assert.rejects(h.before(args), /CORTEX_CIRCUIT_OPEN/,
    'recovery counts alone never prove the objective is ready');

  for (const result of ['ok', '{"status":"done"}', '{"phase_status":"success"}']) {
    await h.retry('work-real', { task_id: 'work-real', status: 'ready', revision: 7 });
    await h.after(args, result);
    await h.before(args);
  }
  await h.plugin.dispose();
});

test('read-only roles never latch and never report', async () => {
  const h = await harness();
  for (const role of ['investigate', 'reviewer', 'discovery', 'planner']) {
    const args = dispatch(null, role);
    for (let n = 0; n < 6; n++) await h.after(args, '');
    await h.before(args);
  }
  assert.equal(h.reports.length, 0, 'read-only aborts raise no circuit signal here');
  await h.before(dispatch('other-work'));
  await h.plugin.dispose();
});

test('independent work items keep separate circuits and retry clears only its own', async () => {
  const h = await harness(), a = dispatch('work-a'), b = dispatch('work-b');
  for (let n = 1; n <= 4; n++) {
    await assert.rejects(h.after(a, ''), n < 4 ? attempt(n) : /CORTEX_CIRCUIT_OPEN.*work-a/);
    await assert.rejects(h.after(b, '{"status":"failed"}'), n < 4 ? attempt(n) : /CORTEX_CIRCUIT_OPEN.*work-b/);
  }
  await assert.rejects(h.before(a), /CORTEX_CIRCUIT_OPEN.*work-a/);
  await assert.rejects(h.before(b), /CORTEX_CIRCUIT_OPEN.*work-b/);
  await h.retry('work-b', { task_id: 'work-b', status: 'ready', revision: 3 });
  await h.before(b);
  await assert.rejects(h.before(a), /CORTEX_CIRCUIT_OPEN.*work-a/);
  await h.plugin.dispose();
});

test('unknown objectives latch distinctly and capacity exhaustion never evicts failures', async () => {
  const h = await harness();
  const a = { subagent_type: 'investigate', prompt: 'Read a' };
  await h.before({ subagent_type: 'implement', prompt: 'Diagnose missing identity' });
  await h.after(a, '');
  await h.before(a);
  for (let i = 0; i < 256; i++) await assert.rejects(h.after(dispatch('capacity-' + i), ''), attempt(1));
  await assert.rejects(h.after(dispatch('new-work'), ''), /CORTEX_CIRCUIT_OPEN/);
  await assert.rejects(h.before(dispatch('new-work')), /CORTEX_LATCH_CAPACITY/);
  await h.before({ subagent_type: 'investigate', prompt: 'Diagnose saturation read-only' });
  await h.retry('capacity-0', { task_id: 'capacity-0', status: 'ready', revision: 2 });
  await assert.rejects(h.before(dispatch('capacity-1')), /CORTEX_LATCH_CAPACITY/,
    'saturation requires explicit reconciliation; no failure is released');
  await h.plugin.dispose();
});

test('identity rules: role mismatch and oversized retry ids fail closed, bad envelopes earn no identity', async () => {
  const h = await harness(), args = dispatch();
  const mismatched = { ...args, prompt: args.prompt.replace('"role":"implement"', '"role":"reviewer"') };
  await assert.rejects(h.before(mismatched), /CORTEX_DISPATCH_IDENTITY_INVALID/);
  await assert.rejects(h.plugin['tool.execute.before'](
    { tool: 'cortex_ia_work_retry', sessionID: 'root', callID: 'retry' },
    { args: { task_id: 'x'.repeat(129) } }), /CORTEX_DISPATCH_IDENTITY_INVALID/);

  // Duplicate decoded keys collapse to "no durable identity": no task attribution,
  // no telemetry, but the open circuit still blocks the same malformed dispatch.
  const duplicated = { ...args, prompt: '<minion-dispatch>{"task_id":"a","task\\u005fid":"b"}</minion-dispatch>' };
  for (let n = 1; n <= 4; n++) await assert.rejects(h.after(duplicated, ''), n < 4 ? attempt(n) : /CORTEX_CIRCUIT_OPEN/);
  assert.equal(h.reports.length, 0, 'ambiguous identity never manufactures durable telemetry');
  await assert.rejects(h.before(duplicated), /CORTEX_CIRCUIT_OPEN/);
  await h.before(args);
  await h.plugin.dispose();
});

test('background acceptance stays pending and terminal failure latches the durable work', async () => {
  const h = await harness(), args = { ...dispatch(), background: true };
  await h.after(args, '{"status":"accepted","session_id":"child"}');
  await h.plugin.event({ event: { type: 'session.idle', properties: { sessionID: 'child' } } });
  await h.before(dispatch());
  await h.plugin.event({ event: { type: 'session.error', properties: { sessionID: 'child', error: 'failed' } } });
  await h.before(dispatch());
  for (let n = 2; n <= 4; n++) await assert.rejects(h.after(dispatch(), ''), n < 4 ? attempt(n) : /CORTEX_CIRCUIT_OPEN.*work-real/);
  await assert.rejects(h.before(dispatch()), /CORTEX_CIRCUIT_OPEN.*work-real/);
  await h.retry('work-real', { task_id: 'work-real', status: 'ready', revision: 9 });

  await h.after(args, '{"status":"accepted","session_id":"child-2"}');
  await assert.rejects(h.plugin['tool.execute.after']({
    tool: 'background_output', sessionID: 'root', args: { task_id: 'child-2' },
  }, { output: '{"status":"failed"}' }), /FAILED_TERMINAL_RESULT/);
  await h.plugin.event({ event: { type: 'session.deleted', properties: { info: { id: 'root' } } } });
  await h.before(dispatch());
  await h.plugin.dispose();
});

test('synthetic unlatch and host-proven leaf recovery cannot bypass authority', async () => {
  const h = await harness();
  for (const tool of ['unlatch', 'cortex_unlatch', 'cortex_ia_unlatch', 'cortex_work_unlatch']) {
    await assert.rejects(h.plugin['tool.execute.before']({ tool, sessionID: 'root' }, { args: {} }), /CORTEX_UNLATCH_UNAVAILABLE/);
  }
  for (const tool of ['cortex_recover', 'cortex_ia_recover', 'cortex_ia_work_recover', 'cortex_ia_work_retry']) {
    await assert.rejects(h.plugin['tool.execute.before']({ tool, sessionID: 'leaf' },
      { args: { role: 'orchestrator' } }), /CORTEX_RECOVERY_UNAUTHORIZED/);
  }
  await h.plugin.dispose();
});
