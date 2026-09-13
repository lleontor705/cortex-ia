import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

test('empty subagent output latches session and explains supported continuation with known task_id', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', { allowChildProcess: true });
  const { CortexTaskLatchPlugin } = mod.exports;

  const plugin = await CortexTaskLatchPlugin({ directory: process.cwd() });

  // Subagent task produces empty output (< 20 chars)
  await assert.rejects(
    async () => {
      await plugin['tool.execute.after'](
        { tool: 'task', sessionID: 'ses-1', args: { subagent_type: 'implement', task_id: 'task-latched-01' } },
        { output: '' }
      );
    },
    {
      message: /CORTEX_SUBAGENT_EMPTY_RESULT.*supported continuation requires orchestrator reconciliation/
    }
  );

  // Subsequent task dispatch in same session is rejected with supported continuation guidance
  await assert.rejects(
    async () => {
      await plugin['tool.execute.before'](
        { tool: 'task', sessionID: 'ses-1' },
        { args: { subagent_type: 'implement' } }
      );
    },
    (err) => {
      assert.match(err.message, /CORTEX_DISPATCH_LATCHED/);
      assert.match(err.message, /task-latched-01/);
      assert.match(err.message, /Supported continuation: Orchestrator must reconcile/);
      assert.match(err.message, /without reusing expired tokens/);
      return true;
    }
  );

  await plugin.dispose();
});

test('latch explains unavailable continuation when task identity is absent', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', { allowChildProcess: true });
  const { CortexTaskLatchPlugin } = mod.exports;

  const plugin = await CortexTaskLatchPlugin({ directory: process.cwd() });

  // Subagent task without task_id produces empty output
  await assert.rejects(
    async () => {
      await plugin['tool.execute.after'](
        { tool: 'task', sessionID: 'ses-no-id', args: { subagent_type: 'investigate' } },
        { output: 'short' }
      );
    },
    { message: /CORTEX_SUBAGENT_EMPTY_RESULT/ }
  );

  // Subsequent dispatch reports limitation and required orchestrator action
  await assert.rejects(
    async () => {
      await plugin['tool.execute.before'](
        { tool: 'task', sessionID: 'ses-no-id' },
        { args: { subagent_type: 'investigate' } }
      );
    },
    (err) => {
      assert.match(err.message, /CORTEX_DISPATCH_LATCHED/);
      assert.match(err.message, /Task identity is unknown/);
      assert.match(err.message, /continuation capability is unavailable without an identified task/);
      return true;
    }
  );

  await plugin.dispose();
});

test('invented unlatch tools are rejected fail-closed', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts', { allowChildProcess: true });
  const latchMod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', { allowChildProcess: true });
  const { CortexTaskLatchPlugin } = latchMod.exports;

  const plugin = await CortexTaskLatchPlugin({ directory: process.cwd() });

  const unlatchTools = ['unlatch', 'cortex_unlatch', 'cortex_ia_unlatch', 'cortex_work_unlatch'];
  for (const tool of unlatchTools) {
    await assert.rejects(
      async () => {
        await plugin['tool.execute.before']({ tool, sessionID: 'ses-1' }, { args: {} });
      },
      (err) => {
        assert.match(err.message, /CORTEX_UNLATCH_UNAVAILABLE: Unlatch tools do not exist/);
        return true;
      }
    );
  }

  await plugin.dispose();
});

test('leaf recovery bypass attempts are rejected', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', { allowChildProcess: true });
  const { CortexTaskLatchPlugin } = mod.exports;

  const plugin = await CortexTaskLatchPlugin({ directory: process.cwd() });

  const recoveryTools = ['cortex_recover', 'cortex_ia_recover', 'cortex_ia_work_recover', 'cortex_ia_work_retry'];
  const leafRoles = ['implement', 'reviewer', 'planner', 'investigate'];

  for (const tool of recoveryTools) {
    for (const role of leafRoles) {
      await assert.rejects(
        async () => {
          await plugin['tool.execute.before'](
            { tool, sessionID: 'ses-leaf' },
            { args: { subagent_type: role } }
          );
        },
        (err) => {
          assert.match(err.message, /CORTEX_RECOVERY_UNAUTHORIZED: Leaf subagents cannot perform recovery or retry/);
          return true;
        }
      );
    }
  }

  // Orchestrator role is permitted past this check
  let orchError = null;
  try {
    await plugin['tool.execute.before'](
      { tool: 'cortex_ia_work_recover', sessionID: 'ses-orch' },
      { args: { role: 'orchestrator' } }
    );
  } catch (e) {
    orchError = e;
  }
  assert.equal(orchError, null, 'orchestrator must not be blocked by leaf recovery check');

  await plugin.dispose();
});

test('session.deleted event clears latch for cleaned session', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', { allowChildProcess: true });
  const { CortexTaskLatchPlugin } = mod.exports;

  const plugin = await CortexTaskLatchPlugin({ directory: process.cwd() });

  // Latch session
  await assert.rejects(
    async () => {
      await plugin['tool.execute.after'](
        { tool: 'task', sessionID: 'ses-del', args: { subagent_type: 'implement', task_id: 't-1' } },
        { output: '' }
      );
    },
    { message: /CORTEX_SUBAGENT_EMPTY_RESULT/ }
  );

  // Verify latched
  await assert.rejects(
    async () => {
      await plugin['tool.execute.before']({ tool: 'task', sessionID: 'ses-del' }, { args: { subagent_type: 'implement' } });
    },
    { message: /CORTEX_DISPATCH_LATCHED/ }
  );

  // Emit session.deleted
  await plugin.event({
    event: { type: 'session.deleted', properties: { info: { id: 'ses-del' } } }
  });

  // Verify unlatched after deletion
  let afterDeleteErr = null;
  try {
    await plugin['tool.execute.before']({ tool: 'task', sessionID: 'ses-del' }, { args: { subagent_type: 'implement' } });
  } catch (e) {
    afterDeleteErr = e;
  }
  assert.equal(afterDeleteErr, null, 'session deletion must clear latch');

  await plugin.dispose();
});
