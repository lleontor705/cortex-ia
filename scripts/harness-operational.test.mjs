import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const source = 'internal/assets/plugins/cortex-subagent-transport.ts';
const envelope = (extra = {}) => ({
  contract_version: '1.0', role: 'implement', workflow: 'direct-change', phase: 'apply',
  spec_plane: null, task_id: 'work-ops', objective: 'Inspect the assigned synthetic operation',
  allowed_files: [], acceptance_checks: [], artifact_refs: [], ...extra,
});
const prompt = (value) => `<minion-dispatch>${JSON.stringify(value)}</minion-dispatch>`;
const load = () => loadPluginFile(source, {
  childProcess: { execFileSync: () => assert.fail('External processes are forbidden') },
});

test('fileless implementation rejects target/effects and claimed authority in all execution modes', async () => {
  const { exports: { dispatchInfo } } = await load();
  const variants = [
    {},
    { operational_target: 'synthetic-db' },
    { operational_effects: ['write'] },
    { workflow: 'ops-task', operational_target: 'synthetic-db', operational_effects: ['write'] },
    { workflow: 'ops-task', target: 'synthetic-db', target_effects: ['write'], authority: true, authority_available: true },
    { operation_type: 'database', allowed_effects: ['write'], execution_mode: 'native' },
    { is_operational: true, execution_mode: 'external' },
    { task_id: null },
    { authority: false },
  ];
  for (const extra of variants) {
    for (const delegated of [false, true]) {
      assert.throws(() => dispatchInfo({ subagent_type: 'implement', delegated }, prompt(envelope(extra))),
        /implementation requires a non-empty file scope; external-effect authority is unavailable/);
    }
  }
  // Historical unversioned operational envelopes cannot restore the retired bypass.
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, prompt({
    role: 'implement', workflow: 'ops-task', operational_target: 'synthetic-db',
    operational_effects: ['write'], allowed_files: [], task_id: 'legacy-work',
  })), /external-effect authority is unavailable/);
});

test('read-only investigation and review retain canonical identity and budget validation', async () => {
  const { exports: { dispatchInfo } } = await load();
  for (const role of ['investigate', 'reviewer']) {
    const value = envelope({ role, workflow: role, phase: 'inspect', task_id: null, max_steps: 5 });
    const result = dispatchInfo({ subagent_type: role }, prompt(value));
    assert.equal(result.limit, 5);
    assert.equal(result.operational, false);
    assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, prompt(value)), /role does not match/);
    assert.throws(() => dispatchInfo({ subagent_type: role, max_steps: 5 }, prompt(value)), /ambiguous step budgets/);
  }
  assert.throws(() => dispatchInfo({ subagent_type: 'unknown' }, prompt(envelope({ role: 'unknown' }))), /invalid common dispatch contract/);
});

test('actual task hook rejects unsupported mutations and admits scoped or read-only dispatches', async () => {
  const { exports: { CortexSubagentTransportPlugin } } = await load();
  const plugin = await CortexSubagentTransportPlugin({ client: { session: {
    get: async ({ path }) => ({ data: { id: path.id } }), messages: async () => ({ data: [] }),
  } } });
  try {
    const rejected = { args: { subagent_type: 'implement', prompt: prompt(envelope({
      workflow: 'ops-task', operational_target: 'synthetic-db', operational_effects: ['write'],
    })) } };
    const before = JSON.stringify(rejected);
    await assert.rejects(plugin['tool.execute.before']({ tool: 'task', sessionID: 'root', callID: 'rejected' }, rejected),
      /external-effect authority is unavailable/);
    assert.equal(JSON.stringify(rejected), before, 'rejected admission must preserve caller input');
    for (const role of ['implement', 'investigate', 'reviewer']) {
      const allowed_files = role === 'implement' ? ['synthetic.go'] : [];
      const output = { args: { subagent_type: role, prompt: prompt(envelope({ role, allowed_files, max_steps: 8 })) } };
      await plugin['tool.execute.before']({ tool: 'task', sessionID: 'root', callID: role }, output);
      const normalized = JSON.parse(output.args.prompt.match(/<minion-dispatch>(.*)<\/minion-dispatch>/)[1]);
      assert.equal(normalized.role, role);
      assert.equal(normalized.task_id, 'work-ops');
      assert.equal(normalized.max_steps, 8);
      assert.deepEqual(normalized.allowed_files, allowed_files);
    }
  } finally { await plugin.dispose(); }
});
