import test from 'node:test';
import assert from 'node:assert/strict';
import { transpileTS, createIsolatedSandbox, loadPluginSource, loadPluginFile } from './harness-plugin-loader.mjs';

test('transpileTS strips types and outputs CommonJS', () => {
  const ts = 'const x: number = 42; export const answer = (): number => x;';
  const js = transpileTS(ts);
  assert.match(js, /exports\.answer/);
  assert.doesNotMatch(js, /: number/);
});

test('isolated sandbox blocks unauthorized imports', () => {
  const sandbox = createIsolatedSandbox();
  assert.throws(
    () => sandbox.require('child_process'),
    /UNAUTHORIZED_MODULE_IMPORT: child_process/
  );
  assert.throws(
    () => sandbox.require('node:net'),
    /UNAUTHORIZED_MODULE_IMPORT: node:net/
  );
  assert.throws(
    () => sandbox.require('http'),
    /UNAUTHORIZED_MODULE_IMPORT: http/
  );
});

test('isolated sandbox permits allowed modules and bounds environment', () => {
  const sandbox = createIsolatedSandbox({
    env: { MOCK_VAR: 'isolated_val' },
  });
  const cryptoMod = sandbox.require('node:crypto');
  assert.ok(cryptoMod.createHash);
  assert.equal(sandbox.process.env.MOCK_VAR, 'isolated_val');
  assert.equal(sandbox.process.env.PATH, undefined);
});

test('loadPluginSource executes simple TS plugin with hooks', async () => {
  const source = `
    import { type Plugin } from "@opencode-ai/plugin";
    export const SamplePlugin: Plugin = async ({ client }) => {
      return {
        "session.start": async () => { return { ok: true }; },
        name: "sample-plugin",
      };
    };
    export default SamplePlugin;
  `;

  const loaded = await loadPluginSource(source);
  assert.ok(loaded.pluginFn);
  const instance = await loaded.instantiate({});
  assert.equal(instance.name, 'sample-plugin');
  assert.equal(typeof instance['session.start'], 'function');
});

test('loadPluginFile loads real cortex-subagent-transport plugin', async () => {
  const loaded = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts', {
    env: { SUBAGENT_TRANSPORT_LOG: '0' },
  });
  assert.ok(loaded.pluginFn);
  const instance = await loaded.instantiate({ directory: process.cwd() });
  assert.ok(instance);
  assert.equal(typeof instance['event'], 'function');
  assert.equal(typeof instance['dispose'], 'function');
  assert.equal(typeof instance['experimental.chat.system.transform'], 'function');
});
