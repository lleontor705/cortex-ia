import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

test('CortexSubagentTransportPlugin transforms system prompt for child subagents', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const PluginFn = mod.exports.CortexSubagentTransportPlugin;

  const mockCtx = {
    client: {
      session: {
        get: async () => ({ data: { id: 'child-1', parentID: 'root-1' } }),
        messages: async () => ({ data: [] })
      }
    }
  };

  const hooks = await PluginFn(mockCtx);

  // Register child session
  await hooks.event({
    event: {
      type: 'session.created',
      properties: {
        info: { id: 'child-1', parentID: 'root-1' }
      }
    }
  });

  const output = { system: ['Host system instructions: adhere strictly to security rules.'] };
  await hooks['experimental.chat.system.transform']({ sessionID: 'child-1' }, output);

  // 1. Preserves host instructions
  assert.equal(output.system[0], 'Host system instructions: adhere strictly to security rules.');
  assert.equal(output.system.length, 2);

  const transportText = output.system[1];
  const words = transportText.split(/\s+/).filter(Boolean);

  // 2. Word count <= 80
  assert.ok(words.length <= 80, `Transport summary exceeds 80 words: actual ${words.length}`);
  assert.ok(words.length >= 20, `Transport summary should be substantive: actual ${words.length}`);

  // 3. Retains role boundaries and host precedence
  assert.match(transportText, /host\s+precedence|host\s+policy/i);
  assert.match(transportText, /role\s+authority/i);
  assert.match(transportText, /orchestrator/i);

  // 4. Retains live claims and leases
  assert.match(transportText, /claim/i);
  assert.match(transportText, /lease/i);
  assert.match(transportText, /token|secret/i);

  // 5. Retains direct routing and reconciliation
  assert.match(transportText, /direct\s+routing|unassigned\s+duties/i);
  assert.match(transportText, /reconcile/i);

  // 6. No telemetry or savings claims
  assert.doesNotMatch(transportText, /telemetry|token\s+savings|benchmark/i);

  // 7. Idempotent: does not duplicate on repeat transform
  await hooks['experimental.chat.system.transform']({ sessionID: 'child-1' }, output);
  assert.equal(output.system.length, 2);

  // 8. Root session is untouched
  const rootOutput = { system: ['Root system instruction.'] };
  await hooks['experimental.chat.system.transform']({ sessionID: 'root-1' }, rootOutput);
  assert.equal(rootOutput.system.length, 1);
  assert.equal(rootOutput.system[0], 'Root system instruction.');

  await hooks.dispose();
});

test('dispatchInfo enforces role alignment and boundaries', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const validPrompt = `<minion-dispatch>
{
  "task_id": "task-test-01",
  "role": "implement",
  "workflow": "direct-change",
  "allowed_files": ["foo.go"]
}
</minion-dispatch>`;

  const res = dispatchInfo({ subagent_type: 'implement' }, validPrompt);
  assert.equal(res.limit, 70);
  assert.equal(res.operational, false);

  const mismatchPrompt = `<minion-dispatch>
{
  "task_id": "task-test-02",
  "role": "reviewer",
  "workflow": "review",
  "allowed_files": []
}
</minion-dispatch>`;

  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, mismatchPrompt), {
    message: /dispatch role does not match host task target/
  });
});
