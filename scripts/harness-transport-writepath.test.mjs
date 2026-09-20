import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const source = 'internal/assets/plugins/cortex-subagent-transport.ts';

function mockContext(sessionDb, messagesDb, overrides = {}) {
  const client = {
    session: {
      get: async ({ path: { id } }) => ({ data: sessionDb.get(id) }),
      messages: async ({ path: { id } }) => ({ data: messagesDb.get(id) || [] }),
    },
  };
  return { client, directory: process.cwd(), ...overrides };
}

async function loadPlugin(context) {
  const mod = await loadPluginFile(source);
  const plugin = await mod.exports.CortexSubagentTransportPlugin(context);
  return plugin;
}

async function registerChild(plugin, childId, parentId) {
  await plugin.event({
    event: { type: 'session.created', properties: { info: { id: childId, parentID: parentId } } },
  });
}

test('duplicate callID is an idempotent no-op on the memory plane and fails closed on the authority plane', async () => {
  const sessionDb = new Map([['child-1', { id: 'child-1', parentID: 'root-1' }]]);
  const messagesDb = new Map([['root-1', []], ['child-1', []]]);
  const plugin = await loadPlugin(mockContext(sessionDb, messagesDb));
  try {
    await registerChild(plugin, 'child-1', 'root-1');

    await plugin['tool.execute.before'](
      { tool: 'cortex_save', sessionID: 'child-1', callID: 'call-mem-dup' },
      { args: { title: 'synthetic' } }
    );
    await assert.doesNotReject(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex.cortex_save', sessionID: 'child-1', callID: 'call-mem-dup' },
        { args: { title: 'synthetic' } }
      );
    }, 'duplicate memory-plane before-hook must be idempotent');

    await plugin['tool.execute.before'](
      { tool: 'cortex_ia_work_claim', sessionID: 'child-1', callID: 'call-auth-dup' },
      { args: { task_id: 'synthetic-task' } }
    );
    await assert.rejects(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_ia.cortex_ia_work_claim', sessionID: 'child-1', callID: 'call-auth-dup' },
        { args: { task_id: 'synthetic-task' } }
      );
    }, { message: /execute_active_call_duplicate/ }, 'duplicate authority-plane before-hook must still fail closed');
  } finally {
    await plugin.dispose();
  }
});

test('missing sessionID correlates through SDK child history and fails closed when unproven', async () => {
  const sessionDb = new Map([['child-2', { id: 'child-2', parentID: 'root-2' }]]);
  const messagesDb = new Map([
    ['root-2', []],
    ['child-2', [{
      info: { id: 'msg-child-2', sessionID: 'child-2', role: 'assistant' },
      parts: [{
        id: 'part-orphan',
        callID: 'call-orphan',
        type: 'tool',
        tool: 'cortex_save',
        sessionID: 'child-2',
        messageID: 'msg-child-2',
        state: { status: 'running' },
      }],
    }]],
  ]);
  const plugin = await loadPlugin(mockContext(sessionDb, messagesDb));
  try {
    await registerChild(plugin, 'child-2', 'root-2');

    await assert.rejects(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_save', callID: 'call-unproven' },
        { args: { title: 'synthetic' } }
      );
    }, { message: /execute_no_session_id/ }, 'an orphan call absent from child history must fail closed');

    await assert.doesNotReject(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_save', callID: 'call-orphan' },
        { args: { title: 'synthetic' } }
      );
    }, 'an orphan call proven by SDK child history must proceed');
  } finally {
    await plugin.dispose();
  }
});

test('identify failure attempts bounded cold restore before the fail-closed rethrow', async () => {
  const sessionDb = new Map([['child-3', { id: 'child-3', parentID: 'root-3' }]]);
  const messagesDb = new Map([['root-3', []], ['child-3', []]]);
  let getCalls = 0;
  const context = mockContext(sessionDb, messagesDb);
  const originalGet = context.client.session.get;
  context.client.session.get = async (input) => {
    getCalls += 1;
    if (getCalls === 1) throw new Error('synthetic transient SDK failure');
    return originalGet(input);
  };
  const plugin = await loadPlugin(context);
  try {
    await assert.doesNotReject(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_ia_work_claim', sessionID: 'child-3', callID: 'call-recovered' },
        { args: { task_id: 'synthetic-task' } }
      );
    }, 'a transient SDK failure must be recovered through cold restore and one retry');
    assert.ok(getCalls >= 2, 'cold restore must consult the SDK again before rethrowing');
  } finally {
    await plugin.dispose();
  }
});

test('unproven identity still fails closed for a mutation and permits read-only tools', async () => {
  const sessionDb = new Map();
  const messagesDb = new Map();
  const plugin = await loadPlugin(mockContext(sessionDb, messagesDb));
  try {
    await assert.rejects(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_ia_work_claim', sessionID: 'child-ghost', callID: 'call-ghost-mutation' },
        { args: { task_id: 'synthetic-task' } }
      );
    }, { message: /SUBAGENT_TRANSPORT_AMBIGUOUS/ }, 'an unproven identity must not mutate');

    await assert.doesNotReject(async () => {
      await plugin['tool.execute.before'](
        { tool: 'cortex_search', sessionID: 'child-ghost', callID: 'call-ghost-read' },
        { args: { query: 'synthetic' } }
      );
    }, 'read-only tools must never be hard-blocked by transport ambiguity');
  } finally {
    await plugin.dispose();
  }
});
