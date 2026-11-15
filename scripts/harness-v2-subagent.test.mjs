import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

test('OpenCode v2 subagent dispatch and cold restore resolve sessionID without parentSessionId', async () => {
  const sessionDb = new Map([
    ['root-ses', { id: 'root-ses' }],
    ['child-v2', { id: 'child-v2', parentID: 'root-ses' }]
  ]);

  const messagesDb = new Map([
    ['root-ses', [
      {
        info: { id: 'msg-parent-1', sessionID: 'root-ses', role: 'assistant' },
        parts: [
          {
            id: 'call-subagent-1',
            type: 'tool',
            tool: 'subagent',
            name: 'subagent',
            callID: 'call-subagent-1',
            sessionID: 'root-ses',
            messageID: 'msg-parent-1',
            state: {
              status: 'running',
              input: {
                agent: 'investigate',
                description: 'Diagnosticar arquitectura',
                prompt: '<minion-dispatch>\n{"contract_version":"2.0","role":"investigate","max_steps":10}\n</minion-dispatch>'
              },
              metadata: { sessionID: 'child-v2', status: 'running' }
            }
          }
        ]
      }
    ]],
    ['child-v2', [
      {
        info: { id: 'msg-child-1', sessionID: 'child-v2', role: 'user' },
        parts: [{ id: 'part-user-1', sessionID: 'child-v2', messageID: 'msg-child-1', type: 'text' }]
      }
    ]]
  ]);

  const mockClient = {
    session: {
      get: async ({ path: { id } }) => ({ data: sessionDb.get(id) }),
      messages: async ({ path: { id } }) => ({ data: messagesDb.get(id) || [] })
    }
  };

  const mod = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts');
  const { CortexSubagentTransportPlugin } = mod.exports;
  const plugin = await CortexSubagentTransportPlugin({ client: mockClient, directory: process.cwd() });

  // Test 1: executeBefore with tool: 'subagent'
  await plugin['tool.execute.before'](
    { tool: 'subagent', sessionID: 'root-ses', callID: 'call-subagent-1' },
    { args: { agent: 'investigate', prompt: '<minion-dispatch>\n{"contract_version":"2.0","role":"investigate","max_steps":10}\n</minion-dispatch>' } }
  );

  // Test 2: metadata hook receives OpenCode v2 part with 'subagent' and 'metadata.sessionID'
  await plugin.event({
    event: { type: 'message.part.updated', properties: { part: messagesDb.get('root-ses')[0].parts[0] } }
  });

  // Test 3: child session can execute tools without ambiguity error
  await plugin['tool.execute.before'](
    { tool: 'read', sessionID: 'child-v2', callID: 'call-child-read-1' },
    { args: { path: 'package.json' } }
  );

  await plugin.dispose();

  // Test 4: Cold restore from fresh plugin instance where in-memory Maps are empty
  const pluginCold = await CortexSubagentTransportPlugin({ client: mockClient, directory: process.cwd() });
  await pluginCold['tool.execute.before'](
    { tool: 'read', sessionID: 'child-v2', callID: 'call-child-read-cold' },
    { args: { path: 'package.json' } }
  );

  await pluginCold.dispose();
});
