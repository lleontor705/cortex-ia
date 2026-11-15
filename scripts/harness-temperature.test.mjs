import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const SOURCE = 'internal/assets/plugins/cortex-subagent-transport.ts';

// Mirror of the six frontmatter request.body.temperature values that Wave B
// removes; the plugin map is now their single live home.
const EXPECTED_TEMPERATURE = {
  orchestrator: 0.2,
  planner: 0.2,
  implement: 0.2,
  discovery: 0.2,
  investigate: 0.3,
  reviewer: 0.1,
};

function mockContext() {
  const hooks = [];
  const session = {
    hook: (name, handler) => { hooks.push({ name, handler }); },
    get: async () => ({ data: { id: 'root-1' } }),
    messages: async () => ({ data: [] }),
  };
  const client = {
    session: {
      get: async () => ({ data: { id: 'root-1' } }),
      messages: async () => ({ data: [] }),
    },
  };
  return { ctx: { client, session }, hooks };
}

async function loadTransport() {
  const mod = await loadPluginFile(SOURCE);
  const { ctx, hooks } = mockContext();
  const plugin = await mod.exports.CortexSubagentTransportPlugin(ctx);
  return { plugin, hooks, contextHook: hooks.find((h) => h.name === 'context')?.handler };
}

test('TestREQ_W2T_001 maps each agent to its temperature and leaves unknown agents untouched', async () => {
  const { plugin, hooks, contextHook } = await loadTransport();
  try {
    assert.equal(hooks.length, 1, 'the transport plugin must register exactly one context hook');
    assert.equal(typeof contextHook, 'function', 'the context hook must be registered through ctx.session.hook');

    for (const [agent, expected] of Object.entries(EXPECTED_TEMPERATURE)) {
      const event = { agent, sessionID: 'child-temp', options: {} };
      await contextHook(event);
      assert.equal(event.options.temperature, expected, `${agent} must receive temperature ${expected}`);
    }

    for (const agent of ['general', 'synthetic-agent', '']) {
      const event = { agent, sessionID: 'child-temp', options: {} };
      await contextHook(event);
      assert.ok(!('temperature' in event.options), `unmapped agent '${agent}' must receive no override`);
    }

    const anonymous = { sessionID: 'child-temp', options: {} };
    await contextHook(anonymous);
    assert.ok(!('temperature' in anonymous.options), 'an event without an agent must receive no override');
  } finally {
    await plugin.dispose();
  }
});

test('TestREQ_W2T_002 prunes live v2 write tools and drops the dead v1 prune entries', async () => {
  const { plugin, contextHook } = await loadTransport();
  try {
    await plugin.event({
      event: { type: 'session.created', properties: { info: { id: 'child-prune', parentID: 'root-1' } } },
    });
    const event = {
      sessionID: 'child-prune',
      agent: 'investigate',
      tools: { edit: {}, write: {}, write_to_file: {}, apply_patch: {}, read: {} },
    };
    await contextHook(event);
    assert.equal(event.tools.edit, undefined, 'edit must still be pruned for read-only roles');
    assert.equal(event.tools.write, undefined, 'write must still be pruned for read-only roles');
    assert.ok(event.tools.write_to_file, 'write_to_file is absent in v2; its dead prune delete must be removed');
    assert.ok(event.tools.apply_patch, 'apply_patch is absent in v2; its dead prune delete must be removed');
    assert.ok(event.tools.read, 'unrelated tools must be preserved');
  } finally {
    await plugin.dispose();
  }
});

test('TestREQ_W2T_003 keeps the registered hook effective in plugin order with fresh per-call options', async () => {
  const { plugin, contextHook } = await loadTransport();
  try {
    const order = [];
    const siblingBefore = async () => { order.push('sibling'); };
    const transport = async (event) => { order.push('transport'); await contextHook(event); };

    const first = { agent: 'reviewer', sessionID: 'child-order', options: {} };
    await siblingBefore(first);
    await transport(first);
    assert.deepEqual(order, ['sibling', 'transport'], 'hooks must run in registration order');
    assert.equal(first.options.temperature, 0.1, 'temperature must survive sibling hook registration order');

    const second = { agent: 'general', sessionID: 'child-order', options: {} };
    await transport(second);
    assert.ok(!('temperature' in second.options), 'a fresh per-call options object must start empty');
    assert.equal(first.options.temperature, 0.1, 'a prior call options object must not be mutated');
  } finally {
    await plugin.dispose();
  }
});
