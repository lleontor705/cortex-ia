import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

// REQ-W3-002 (E3 escalation): repeated read-only-role aborts must surface a
// structured recovery-request receipt without granting those roles any mutation
// authority. The existing once-per-streak ERR_SUBAGENT_READONLY_REPEATED_ABORT
// telemetry is preserved; ERR_SUBAGENT_READONLY_RECOVERY_REQUEST is additive and
// reuses the same report seam, so it must never carry a bridge mutation verb.
// The recovery request targets durable work, so a task-less diagnostic streak
// keeps its existing repeated-abort-only behavior.

const READONLY_ROLES = ['investigate', 'reviewer', 'discovery'];
const MAX_CIRCUIT_ATTEMPTS = 4;
const MUTATION_VERBS = ['transition', 'claim', 'lease', 'approve', 'recover', 'retry', 'reserve', 'degrade'];

function dispatch(task, role) {
  return { subagent_type: role, task_id: 'host-resume-' + role, prompt: '<minion-dispatch>' + JSON.stringify({
    contract_version: '2.0', task_id: task, role, workflow: 'fast-tdd', phase: 'execute',
    objective: 'Recover a read-only objective', spec_plane: 'hybrid', allowed_files: [],
    acceptance_checks: ['check'], artifact_refs: [], max_steps: 10,
  }) + '</minion-dispatch>' };
}

async function harness() {
  const calls = [];
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', {
    virtualFiles: { '/usr/bin/cortex-ia': '' },
    childProcess: { execFileSync: (_file, args) => { calls.push([...args]); return ''; } },
  });
  const plugin = await mod.exports.CortexTaskLatchPlugin({
    directory: '/isolated/harness',
    client: { session: { get: async () => ({ data: { id: 'root' } }) } },
  });
  return {
    plugin,
    calls,
    after: (args, output) => plugin['tool.execute.after']({ tool: 'task', sessionID: 'root', args }, { output }),
    before: args => plugin['tool.execute.before']({ tool: 'task', sessionID: 'root' }, { args }),
  };
}

function reports(calls) {
  return calls.map(args => ({
    code: args[args.indexOf('--code') + 1],
    source: args[args.indexOf('--source') + 1],
    task: args.includes('--task') ? args[args.indexOf('--task') + 1] : undefined,
    details: JSON.parse(args[args.indexOf('--details') + 1]),
  }));
}

const codeCount = (calls, role, code) =>
  reports(calls).filter(r => r.details.role === role && r.code === code).length;

test('TestREQ_LATCH_001 threshold escalation fires once with structured recovery details', async () => {
  const h = await harness();
  for (const role of READONLY_ROLES) {
    const args = dispatch('task-readonly', role);
    for (let n = 0; n < MAX_CIRCUIT_ATTEMPTS + 2; n++) await h.after(args, '');

    assert.equal(codeCount(h.calls, role, 'ERR_SUBAGENT_READONLY_REPEATED_ABORT'), 1,
      `${role}: existing once-per-streak telemetry is preserved`);

    const recovery = reports(h.calls).filter(r => r.details.role === role && r.code === 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST');
    assert.equal(recovery.length, 1, `${role}: recovery request fires exactly once per streak`);
    const { details } = recovery[0];
    assert.equal(details.task_id, 'task-readonly');
    assert.equal(details.attempts, MAX_CIRCUIT_ATTEMPTS);
    assert.equal(details.dispatch_blocked, false);
    assert.equal(typeof details.reason, 'string');
    assert.match(details.objective, /^[0-9a-f]{64}$/, 'objective digest is carried');
    assert.ok(typeof details.recovery_hint === 'string' && details.recovery_hint.length > 0,
      'a suggested orchestrator action is carried');
  }
  await h.plugin.dispose();
});

test('TestREQ_LATCH_002 success clears the streak so the recovery request re-arms', async () => {
  const h = await harness();
  const args = dispatch('task-readonly', 'investigate');
  for (let n = 0; n < MAX_CIRCUIT_ATTEMPTS; n++) await h.after(args, '');
  assert.equal(codeCount(h.calls, 'investigate', 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST'), 1, 'fires at the threshold');

  await h.after(args, 'ok');
  assert.equal(codeCount(h.calls, 'investigate', 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST'), 1, 'a success emits nothing');

  for (let n = 0; n < MAX_CIRCUIT_ATTEMPTS - 1; n++) await h.after(args, '');
  assert.equal(codeCount(h.calls, 'investigate', 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST'), 1, 'below threshold stays quiet');
  await h.after(args, '');
  assert.equal(codeCount(h.calls, 'investigate', 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST'), 2, 're-armed from zero');
  await h.plugin.dispose();
});

test('TestREQ_LATCH_003 escalation grants no mutation authority and never latches', async () => {
  const h = await harness();
  const args = dispatch('task-readonly', 'reviewer');
  for (let n = 0; n < MAX_CIRCUIT_ATTEMPTS + 3; n++) {
    await h.after(args, '');
    await h.before(args);
  }

  assert.ok(h.calls.length >= 2, 'telemetry was emitted');
  for (const call of h.calls) {
    assert.equal(call[0], 'report', 'only the report seam is used');
    assert.equal(call[1], 'error');
    assert.equal(call.some(a => MUTATION_VERBS.includes(a)), false, 'no bridge mutation verb is invoked');
  }
  await h.plugin.dispose();
});

test('TestREQ_LATCH_004 task-less read-only streaks keep repeated-abort-only behavior', async () => {
  const h = await harness();
  const args = dispatch(null, 'discovery');
  for (let n = 0; n < MAX_CIRCUIT_ATTEMPTS + 2; n++) await h.after(args, '');

  assert.equal(codeCount(h.calls, 'discovery', 'ERR_SUBAGENT_READONLY_REPEATED_ABORT'), 1,
    'the existing telemetry still fires once per streak');
  assert.equal(codeCount(h.calls, 'discovery', 'ERR_SUBAGENT_READONLY_RECOVERY_REQUEST'), 0,
    'no durable task means no recovery request');
  await h.before(args);
  await h.plugin.dispose();
});
