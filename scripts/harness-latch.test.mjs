import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

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
test('canonical work identity governs reports, repeated dispatch and successful retry', async () => {
  const h = await harness(), args = dispatch();
  await assert.rejects(h.after(args, ''), /CORTEX_SUBAGENT_EMPTY_RESULT.*work-real/);
  assert.ok(h.reports.some(a => a[a.indexOf('--task') + 1] === 'work-real'));
  assert.equal(h.reports.some(a => a.includes('host-resume-1')), false);
  await assert.rejects(h.before(dispatch('work-real', 'implement', 'different-session')), /CORTEX_DISPATCH_LATCHED/);
  await h.retry('host-resume-1', { task_id: 'host-resume-1', status: 'ready', revision: 2 });
  await assert.rejects(h.before(args), /CORTEX_DISPATCH_LATCHED/);
  await h.retry('work-real', { error: 'failed' });
  await assert.rejects(h.before(args), /CORTEX_DISPATCH_LATCHED/);
  await h.plugin['tool.execute.after']({ tool: 'cortex_ia_work_recover', sessionID: 'root' }, { output: '{"recovered":1}' });
  await assert.rejects(h.before(args), /CORTEX_DISPATCH_LATCHED/);
  await h.retry('work-real', { task_id: 'work-real', status: 'ready', revision: 7 });
  await h.before(args);
  await h.plugin.dispose();
});
test('bounded diagnostics and unrelated work do not erase the failed objective', async () => {
  const h = await harness();
  await assert.rejects(h.after(dispatch(), '{"phase_status":"failed","summary":"detailed failure"}'), /FAILED_TERMINAL_RESULT/);
  await h.before(dispatch(null, 'investigate'));
  await h.before(dispatch(null, 'reviewer'));
  await h.before(dispatch('other-work'));
  await assert.rejects(h.before(dispatch()), /CORTEX_DISPATCH_LATCHED/);
  await h.plugin.dispose();
});
test('independent failed work items retain separate latches through matching retries', async () => {
  const h = await harness(), a = dispatch('work-a'), b = dispatch('work-b');
  await assert.rejects(h.after(a, ''), /work-a/);
  await h.before(b);
  await assert.rejects(h.after(b, '{"status":"failed"}'), /work-b/);
  await assert.rejects(h.before(a), /CORTEX_DISPATCH_LATCHED.*work-a/);
  await assert.rejects(h.before(b), /CORTEX_DISPATCH_LATCHED.*work-b/);
  await h.retry('work-b', { task_id: 'work-b', status: 'ready', revision: 3 });
  await h.before(b);
  await assert.rejects(h.before(a), /CORTEX_DISPATCH_LATCHED.*work-a/);
  await h.retry('work-a', { task_id: 'work-a', status: 'ready', revision: 3 });
  await h.before(a);
  await h.before(b);
  await h.plugin.dispose();
});
test('unknown objectives are distinct and capacity exhaustion never evicts failures', async () => {
  const h = await harness();
  const a = { subagent_type: 'investigate', prompt: 'Read a' };
  const b = { subagent_type: 'investigate', prompt: 'Read b' };
  await assert.rejects(h.after(a, ''), /Task identity is unknown/);
  await h.before(b);
  await assert.rejects(h.after(b, ''), /Task identity is unknown/);
  await assert.rejects(h.before(a), /CORTEX_DISPATCH_LATCHED/);
  await assert.rejects(h.before(b), /CORTEX_DISPATCH_LATCHED/);
  for (let i = 0; i < 255; i++) await assert.rejects(h.after(dispatch('capacity-' + i), ''), /CORTEX_SUBAGENT_EMPTY_RESULT/);
  await assert.rejects(h.before(dispatch('new-work')), /CORTEX_LATCH_CAPACITY/);
  await h.before({ subagent_type: 'investigate', prompt: 'Diagnose saturation read-only' });
  await h.retry('capacity-0', { task_id: 'capacity-0', status: 'ready', revision: 2 });
  await assert.rejects(h.before(dispatch('capacity-1')), /CORTEX_LATCH_CAPACITY/);
  await h.plugin.dispose();
});
test('host resume IDs never manufacture durable identity for historical dispatches', async () => {
  const h = await harness();
  const args = { subagent_type: 'investigate', task_id: 'host-only', prompt: 'Read target' };
  await assert.rejects(h.after(args, ''), /Task identity is unknown/);
  await assert.rejects(h.before({ ...args, task_id: 'other-host' }), /CORTEX_DISPATCH_LATCHED/);
  await h.before({ ...args, prompt: 'Diagnose the failed attempt read-only' });
  assert.equal(h.reports.some(a => a.includes('host-only')), false);
  await h.plugin.dispose();
});
test('short results remain compatible and explicit terminal errors latch', async () => {
  const h = await harness(), args = dispatch();
  for (const result of ['ok', '{"status":"done"}', '{"phase_status":"success"}']) await h.after(args, result);
  await h.before(args);
  await assert.rejects(h.after(args, '{"status":"failed"}'), /FAILED_TERMINAL_RESULT/);
  await assert.rejects(h.before(args), /CORTEX_DISPATCH_LATCHED/);
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
test('ambiguous, duplicate decoded, malformed and oversized identity envelopes fail closed', async () => {
  const h = await harness(), args = dispatch();
  const invalid = [
    args.prompt + args.prompt,
    '<minion-dispatch>{"task_id":"a","task\\u005fid":"b"}</minion-dispatch>',
    '<minion-dispatch>{"task_id":"a","nested":{"x":1,"x":2}}</minion-dispatch>',
    '<minion-dispatch>{"task_id":{"nested":"a"}}</minion-dispatch>',
    '<minion-dispatch>[]</minion-dispatch>',
    '<minion-dispatch>{</minion-dispatch>',
    '<minion-dispatch>{"task_id":"a"}</minion-contract>',
    args.prompt.replace('"role":"implement"', '"role":"reviewer"'),
    'x'.repeat(256 * 1024 + 1),
  ];
  for (const prompt of invalid) await assert.rejects(h.before({ ...args, prompt }), /CORTEX_DISPATCH_IDENTITY_INVALID/);
  await assert.rejects(h.after(dispatch(), ''), /CORTEX_SUBAGENT_EMPTY_RESULT/);
  for (const prompt of invalid) await assert.rejects(h.before({ ...args, prompt }), /CORTEX_DISPATCH_IDENTITY_INVALID/);
  await assert.rejects(h.before(args), /CORTEX_DISPATCH_LATCHED/);
  await h.plugin.dispose();
});
test('background acceptance remains pending and terminal failure latches the durable work', async () => {
  const h = await harness(), args = { ...dispatch(), background: true };
  await h.after(args, '{"status":"accepted","session_id":"child"}');
  await h.plugin.event({ event: { type: 'session.idle', properties: { sessionID: 'child' } } });
  await h.before(dispatch());
  await h.plugin.event({ event: { type: 'session.error', properties: { sessionID: 'child', error: 'failed' } } });
  await assert.rejects(h.before(dispatch()), /CORTEX_DISPATCH_LATCHED.*work-real/);
  await h.retry('work-real', { task_id: 'work-real', status: 'ready', revision: 9 });
  await h.after(args, '{"status":"accepted","session_id":"child-2"}');
  await assert.rejects(h.plugin['tool.execute.after']({
    tool: 'background_output', sessionID: 'root', args: { task_id: 'child-2' },
  }, { output: '{"status":"failed"}' }), /work-real/);
  await h.plugin.event({ event: { type: 'session.deleted', properties: { info: { id: 'root' } } } });
  await h.before(dispatch());
  await h.plugin.dispose();
});
