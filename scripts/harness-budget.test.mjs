import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

test('dispatchInfo accepts canonical max_steps and deprecated budget.max_turns', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  // Canonical envelope.max_steps
  const p1 = '<minion-dispatch>\n{"task_id":"t1","max_steps":25}\n</minion-dispatch>';
  assert.equal(dispatchInfo({ subagent_type: 'implement' }, p1).limit, 25);

  // Deprecated envelope.budget.max_turns
  const p2 = '<minion-dispatch>\n{"task_id":"t2","budget":{"max_turns":15}}\n</minion-dispatch>';
  assert.equal(dispatchInfo({ subagent_type: 'implement' }, p2).limit, 15);

  // Host args.steps and args.max_steps
  const p3 = '<minion-dispatch>\n{"task_id":"t3"}\n</minion-dispatch>';
  assert.equal(dispatchInfo({ subagent_type: 'implement', steps: 30 }, p3).limit, 30);
  assert.equal(dispatchInfo({ subagent_type: 'implement', max_steps: 35 }, p3).limit, 35);
  assert.equal(dispatchInfo({ subagent_type: 'implement', budget: { max_turns: 40 } }, p3).limit, 40);
});

test('dispatchInfo rejects duplicate decoded keys including escaped spellings', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  // Exact duplicate key
  const p1 = '<minion-dispatch>\n{"task_id":"t1","max_steps":10,"max_steps":10}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p1), {
    message: /duplicate decoded key/
  });

  // Escaped spelling duplicate
  const p2 = '<minion-dispatch>\n{"task_id":"t2","max_steps":10,"\\u006dax_steps":10}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p2), {
    message: /duplicate decoded key/
  });

  // Nested budget duplicate
  const p3 = '<minion-dispatch>\n{"task_id":"t3","budget":{"max_turns":5,"\\u006dax_turns":5}}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p3), {
    message: /duplicate decoded key/
  });
});

test('dispatchInfo rejects malformed, null, array, or invalid budget integers', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const invalidForms = [
    '{"task_id":"t","budget":null}',
    '{"task_id":"t","budget":[5]}',
    '{"task_id":"t","budget":"5"}',
    '{"task_id":"t","budget":{}}',
    '{"task_id":"t","budget":{"max_turns":"ten"}}',
    '{"task_id":"t","budget":{"max_turns":0}}',
    '{"task_id":"t","budget":{"max_turns":1001}}',
    '{"task_id":"t","budget":{"max_turns":-1}}',
    '{"task_id":"t","budget":{"max_turns":2.5}}',
  ];

  for (const form of invalidForms) {
    const prompt = `<minion-dispatch>\n${form}\n</minion-dispatch>`;
    assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, prompt), {
      message: /SUBAGENT_TRANSPORT_ERROR/
    });
  }
});

test('dispatchInfo rejects multiple budget forms even when values are equal', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  // envelope max_steps AND envelope budget.max_turns
  const p1 = '<minion-dispatch>\n{"task_id":"t","max_steps":10,"budget":{"max_turns":10}}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p1), {
    message: /ambiguous step budgets/
  });

  // envelope budget having both max_turns and max_steps
  const p2 = '<minion-dispatch>\n{"task_id":"t","budget":{"max_turns":10,"max_steps":10}}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p2), {
    message: /ambiguous budget object/
  });

  // args.steps AND args.max_steps
  const p3 = '<minion-dispatch>\n{"task_id":"t"}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement', steps: 10, max_steps: 10 }, p3), {
    message: /ambiguous step budgets/
  });

  // args.max_steps AND envelope.max_steps
  const p4 = '<minion-dispatch>\n{"task_id":"t","max_steps":10}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement', max_steps: 10 }, p4), {
    message: /ambiguous step budgets/
  });
});

test('host planner is uncapped and role spoofing cannot elevate permissions', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  // Host planner target is uncapped
  const p1 = '<minion-dispatch>\n{"task_id":"t","workflow":"sdd-lite","phase":"integrated","spec_plane":"hybrid"}\n</minion-dispatch>';
  assert.equal(dispatchInfo({ subagent_type: 'planner' }, p1).limit, Infinity);

  // Implement targeting planner in envelope is rejected (role conflict)
  const p2 = '<minion-dispatch>\n{"task_id":"t","role":"planner","workflow":"sdd-lite","phase":"integrated","spec_plane":"hybrid"}\n</minion-dispatch>';
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, p2), {
    message: /dispatch role does not match host task target/
  });

  // Implement with args.agent = planner is rejected
  assert.throws(() => dispatchInfo({ subagent_type: 'implement', agent: 'planner' }, p1), {
    message: /dispatch agent does not match host task target/
  });

  // Default fallback role derive strictly from host target
  assert.equal(dispatchInfo({ subagent_type: 'implement' }, p1).limit, 70);
  assert.equal(dispatchInfo({ subagent_type: 'reviewer' }, p1).limit, 50);
  assert.equal(dispatchInfo({ subagent_type: 'investigate' }, p1).limit, 50);
  assert.equal(dispatchInfo({ subagent_type: 'discovery' }, p1).limit, 60);
});

test('actual plugin emits one advisory notice and permits up to 5 cleanup calls at emergency ceiling', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts', {
    env: { CORTEX_IA_EMERGENCY_STEPS: '3' }
  });
  const { CortexSubagentTransportPlugin } = mod.exports;

  const sessionDb = new Map([
    ['root-ses', { id: 'root-ses', parentID: null }],
    ['child-ses', { id: 'child-ses', parentID: 'root-ses' }],
  ]);

  const messagesDb = new Map([
    ['root-ses', [
      {
        info: { id: 'msg-1', sessionID: 'root-ses', role: 'assistant' },
        parts: [
          {
            id: 'part-task-1',
            type: 'tool',
            tool: 'task',
            callID: 'call-dispatch-1',
            sessionID: 'root-ses',
            messageID: 'msg-1',
            state: {
              status: 'running',
              input: {
                subagent_type: 'implement',
                prompt: '<minion-dispatch>\n{"task_id":"t-live","max_steps":2}\n</minion-dispatch>'
              },
              metadata: { sessionId: 'child-ses', parentSessionId: 'root-ses' }
            }
          }
        ]
      }
    ]],
    ['child-ses', []]
  ]);

  const mockClient = {
    session: {
      get: async ({ path: { id } }) => ({ data: sessionDb.get(id) }),
      messages: async ({ path: { id } }) => ({ data: messagesDb.get(id) || [] })
    }
  };

  const plugin = await CortexSubagentTransportPlugin({ client: mockClient, directory: process.cwd() });

  // 1. Dispatch task
  await plugin['tool.execute.before'](
    { tool: 'task', sessionID: 'root-ses', callID: 'call-dispatch-1' },
    { args: { subagent_type: 'implement', prompt: '<minion-dispatch>\n{"task_id":"t-live","max_steps":2}\n</minion-dispatch>' } }
  );
  await plugin.event({
    event: { type: 'message.part.updated', properties: { part: messagesDb.get('root-ses')[0].parts[0] } }
  });

  // Step 1: normal call
  await plugin['tool.execute.before']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c1' }, { args: {} });
  await plugin['tool.execute.after']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c1', args: {} }, { output: 'res1' });

  // Step 2: advisory threshold reached (max_steps: 2)
  await plugin['tool.execute.before']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c2' }, { args: {} });
  await plugin['tool.execute.after']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c2', args: {} }, { output: 'res2' });

  // System transform check: exactly one current advisory warning
  const chatOutput = { system: ['base prompt'] };
  await plugin['experimental.chat.system.transform']({ sessionID: 'child-ses' }, chatOutput);
  const budgetWarnings = chatOutput.system.filter(s => s.startsWith('CORTEX_BUDGET_WARNING:'));
  assert.equal(budgetWarnings.length, 1);

  // Tools still remain usable past advisory budget
  await plugin['tool.execute.before']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c3' }, { args: {} });
  await plugin['tool.execute.after']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c3', args: {} }, { output: 'res3' });

  // Step 4: hits emergency ceiling (3). Ordinary tools blocked.
  await assert.rejects(
    async () => {
      await plugin['tool.execute.before']({ tool: 'read', sessionID: 'child-ses', callID: 'call-c4' }, { args: {} });
    },
    { message: /AGENT_EMERGENCY_LIMIT/ }
  );

  // Exactly 5 cleanup calls permitted
  for (let i = 1; i <= 5; i++) {
    await plugin['tool.execute.before'](
      { tool: 'cortex_ia_file_release', sessionID: 'child-ses', callID: `call-clean-${i}` },
      { args: { path: 'foo.go' } }
    );
    await plugin['tool.execute.after'](
      { tool: 'cortex_ia_file_release', sessionID: 'child-ses', callID: `call-clean-${i}`, args: {} },
      { output: 'ok' }
    );
  }

  // 6th cleanup call rejected
  await assert.rejects(
    async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_ia_file_release', sessionID: 'child-ses', callID: 'call-clean-6' },
        { args: { path: 'foo.go' } }
      );
    },
    { message: /AGENT_EMERGENCY_LIMIT/ }
  );

  await plugin.dispose();
});

test('invalid init config rejects plugin startup', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts', {
    env: { CORTEX_IA_EMERGENCY_STEPS: 'invalid_number' }
  });
  const { CortexSubagentTransportPlugin } = mod.exports;

  await assert.rejects(
    async () => {
      await CortexSubagentTransportPlugin({ client: {}, directory: process.cwd() });
    },
    { message: /SUBAGENT_TRANSPORT_CONFIG: CORTEX_IA_EMERGENCY_STEPS must be an integer/ }
  );
});
