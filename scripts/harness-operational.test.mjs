import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { loadPluginFile } from './harness-plugin-loader.mjs';

function hashFile(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

test('dispatchInfo admits valid native operational envelope with explicit target/effects', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const prompt = `<minion-dispatch>
{
  "task_id": "task-ops-01",
  "role": "implement",
  "workflow": "ops-task",
  "operational_target": "db:users",
  "operational_effects": ["create index idx_email on users(email)"],
  "allowed_files": []
}
</minion-dispatch>`;

  const res = dispatchInfo({ subagent_type: 'implement' }, prompt);
  assert.equal(res.operational, true);
  assert.equal(res.limit, 70);
});

test('dispatchInfo rejects missing operational target or effects', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const promptNoEffects = `<minion-dispatch>
{
  "task_id": "task-ops-02",
  "role": "implement",
  "workflow": "ops-task",
  "operational_target": "db:users",
  "allowed_files": []
}
</minion-dispatch>`;
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, promptNoEffects), {
    message: /implementation requires an explicit file scope or authorized operational target and effects/
  });

  const promptNoTarget = `<minion-dispatch>
{
  "task_id": "task-ops-03",
  "role": "implement",
  "workflow": "ops-task",
  "operational_effects": ["migration"],
  "allowed_files": []
}
</minion-dispatch>`;
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, promptNoTarget), {
    message: /implementation requires an explicit file scope or authorized operational target and effects/
  });
});

test('dispatchInfo rejects ordinary empty file scope and spoofed roles', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const ordinaryEmpty = `<minion-dispatch>
{
  "task_id": "task-impl-01",
  "role": "implement",
  "workflow": "sdd-lite",
  "allowed_files": []
}
</minion-dispatch>`;
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, ordinaryEmpty), {
    message: /implementation requires an explicit file scope or authorized operational target and effects/
  });

  const spoofedRole = `<minion-dispatch>
{
  "task_id": "task-spoof-01",
  "role": "planner",
  "workflow": "sdd-full",
  "allowed_files": []
}
</minion-dispatch>`;
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, spoofedRole), {
    message: /dispatch role does not match host task target/
  });
});

test('dispatchInfo rejects external operational execution and unavailable authority', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { dispatchInfo } = mod.exports;

  const opsPrompt = `<minion-dispatch>
{
  "task_id": "task-ops-ext",
  "role": "implement",
  "workflow": "ops-task",
  "operational_target": "sp_sync",
  "operational_effects": ["exec sp_sync"],
  "allowed_files": []
}
</minion-dispatch>`;

  assert.throws(() => dispatchInfo({ subagent_type: 'implement', delegated: true }, opsPrompt), {
    message: /external operational execution is unavailable; requires fresh orchestrator-native dispatch/
  });

  const extPrompt = opsPrompt.replace('"workflow": "ops-task",', '"workflow": "ops-task", "execution_mode": "external",');
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, extPrompt), {
    message: /external operational execution is unavailable; requires fresh orchestrator-native dispatch/
  });

  const noTaskPrompt = opsPrompt.replace('"task_id": "task-ops-ext",', '"task_id": null,');
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, noTaskPrompt), {
    message: /operational implementation requires valid task authority/
  });

  const noAuthPrompt = opsPrompt.replace('"task_id": "task-ops-ext",', '"task_id": "task-ops-ext", "authority": false,');
  assert.throws(() => dispatchInfo({ subagent_type: 'implement' }, noAuthPrompt), {
    message: /operational implementation requires valid task authority/
  });
});

test('plugin fixture blocks repository writes for operational child sessions and preserves baseline', async () => {
  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { CortexSubagentTransportPlugin } = mod.exports;

  const sessionDb = new Map([
    ['root-ses', { id: 'root-ses', parentID: null }],
    ['child-ops-ses', { id: 'child-ops-ses', parentID: 'root-ses' }],
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
            callID: 'call-task-ops-1',
            sessionID: 'root-ses',
            messageID: 'msg-1',
            state: {
              status: 'running',
              input: {
                subagent_type: 'implement',
                prompt: `<minion-dispatch>
{
  "task_id": "task-ops-live",
  "role": "implement",
  "workflow": "ops-task",
  "operational_target": "postgres://prod",
  "operational_effects": ["apply schema"],
  "allowed_files": []
}
</minion-dispatch>`
              },
              metadata: {
                sessionId: 'child-ops-ses',
                parentSessionId: 'root-ses'
              }
            }
          }
        ]
      }
    ]],
    ['child-ops-ses', []]
  ]);

  const mockClient = {
    session: {
      get: async ({ path: { id } }) => {
        const item = sessionDb.get(id);
        if (!item) throw new Error('not found');
        return { data: item };
      },
      messages: async ({ path: { id } }) => {
        return { data: messagesDb.get(id) || [] };
      }
    }
  };

  const plugin = await CortexSubagentTransportPlugin({ client: mockClient, directory: process.cwd() });

  await plugin['tool.execute.before'](
    { tool: 'task', sessionID: 'root-ses', callID: 'call-task-ops-1' },
    {
      args: {
        subagent_type: 'implement',
        prompt: `<minion-dispatch>
{
  "task_id": "task-ops-live",
  "role": "implement",
  "workflow": "ops-task",
  "operational_target": "postgres://prod",
  "operational_effects": ["apply schema"],
  "allowed_files": []
}
</minion-dispatch>`
      }
    }
  );

  await plugin.event({
    event: {
      type: 'message.part.updated',
      properties: { part: messagesDb.get('root-ses')[0].parts[0] }
    }
  });

  const baselineFixture = path.join(process.cwd(), 'package.json');
  const baselineHashBefore = hashFile(baselineFixture);

  await plugin['tool.execute.before'](
    { tool: 'read', sessionID: 'child-ops-ses', callID: 'call-child-read' },
    { args: { path: 'package.json' } }
  );

  await assert.rejects(
    async () => {
      await plugin['tool.execute.before'](
        { tool: 'write_to_file', sessionID: 'child-ops-ses', callID: 'call-child-write' },
        { args: { target_file: 'package.json', content: 'mutated' } }
      );
    },
    { message: /repository write tools are forbidden for operational tasks/ }
  );

  await assert.rejects(
    async () => {
      await plugin['tool.execute.before'](
        { tool: 'edit', sessionID: 'child-ops-ses', callID: 'call-child-edit' },
        { args: { target_file: 'package.json' } }
      );
    },
    { message: /repository write tools are forbidden for operational tasks/ }
  );

  const baselineHashAfter = hashFile(baselineFixture);
  assert.equal(baselineHashBefore, baselineHashAfter, 'unrelated baseline must remain preserved');

  await plugin.dispose();
});
