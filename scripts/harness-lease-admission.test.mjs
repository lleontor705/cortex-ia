import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const guardPath = 'internal/assets/plugins/cortex-lease-guard.ts';
const fencePath = 'internal/assets/plugins/cortex-permission-fence.ts';
const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');

function timedOut(message) {
  const error = new Error(message);
  error.code = 'ETIMEDOUT';
  return error;
}

function guardOptions({ found, execFileSync }) {
  return {
    cwd: root,
    env: { USERPROFILE: home, HOME: home },
    fs: {
      existsSync: p => found && p === executable,
      realpathSync: p => p,
      lstatSync: () => ({ isSymbolicLink: () => false }),
      readFileSync: () => '',
      writeFileSync: () => {},
      mkdirSync: () => {},
    },
    childProcess: { execFileSync },
  };
}

async function guardHook(opts) {
  const mod = await loadPluginFile(guardPath, opts);
  return (await mod.instantiate({ directory: root }))['tool.execute.before'];
}

test('lease guard reports CLI timeout with attempt count and latency telemetry', async () => {
  const calls = [];
  const hook = await guardHook(guardOptions({
    found: true,
    execFileSync: (_file, args) => {
      calls.push(args);
      throw timedOut('spawnSync ETIMEDOUT');
    },
  }));
  await assert.rejects(
    hook({ tool: 'write', sessionID: 'test' }, { args: { filePath: 'src/main.ts' } }),
    err => {
      assert.match(err.message, /^LEASE_REQUIRED/);
      assert.match(err.message, /lease verification CLI exceeded 15000ms \(attempt 2\/2\)/);
      assert.match(err.message, /admission_elapsed_ms=\d+/);
      return true;
    }
  );
  assert.equal(calls.filter(args => args[1] === 'verify-lease').length, 2);
});

test('lease guard retries one timed-out verification and admits a verified lease', async () => {
  let attempts = 0;
  const hook = await guardHook(guardOptions({
    found: true,
    execFileSync: (_file, args) => {
      if (args[1] !== 'verify-lease') return '';
      attempts += 1;
      if (attempts === 1) throw timedOut('spawnSync ETIMEDOUT');
      return JSON.stringify({
        valid: true,
        owner: 'opencode-session:test',
        task_id: 'task-1',
        expires_at: new Date(Date.now() + 60000).toISOString(),
      });
    },
  }));
  await hook({ tool: 'write', sessionID: 'test' }, { args: { filePath: 'src/main.ts' } });
  assert.equal(attempts, 2);
});

test('lease guard bounds executable lookup and surfaces a stalled PATH scan', async () => {
  const lookups = [];
  const hook = await guardHook(guardOptions({
    found: false,
    execFileSync: (file, _args, opts) => {
      lookups.push({ file, opts });
      throw timedOut('spawnSync ETIMEDOUT');
    },
  }));
  await assert.rejects(
    hook({ tool: 'write', sessionID: 'test' }, { args: { filePath: 'src/main.ts' } }),
    err => {
      assert.match(err.message, /^LEASE_REQUIRED/);
      assert.match(err.message, /executable lookup for 'cortex-ia' timed out after 3000ms/);
      assert.match(err.message, /admission_elapsed_ms=\d+/);
      return true;
    }
  );
  assert.equal(lookups.length, 1);
  assert.match(lookups[0].file, /^(where\.exe|which)$/);
  assert.equal(lookups[0].opts.timeout, 3000);
});

test('permission fence falls through without auto-allow when executable lookup stalls', async () => {
  const lookups = [];
  const opts = {
    cwd: root,
    env: { USERPROFILE: home, HOME: home },
    fs: {
      existsSync: () => false,
      readFileSync: () => '',
      statSync: () => ({ size: 0 }),
      writeFileSync: () => {},
      mkdirSync: () => {},
    },
    childProcess: {
      execFileSync: (file, _args, callOpts) => {
        lookups.push({ file, opts: callOpts });
        throw timedOut('spawnSync ETIMEDOUT');
      },
    },
  };
  let evaluate;
  const mod = await loadPluginFile(fencePath, opts);
  await mod.instantiate({
    directory: root,
    permission: { hook: (_name, callback) => { evaluate = callback; } },
  });
  assert.equal(typeof evaluate, 'function');
  const decision = await evaluate({ tool: 'write', path: 'src/main.ts', sessionID: 'test' });
  assert.equal(decision, undefined);
  assert.equal(lookups.length, 1);
  assert.match(lookups[0].file, /^(where\.exe|which)$/);
  assert.equal(lookups[0].opts.timeout, 3000);
});
