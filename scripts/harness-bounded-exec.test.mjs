import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

// Every synchronous child process a plugin spawns on the host event loop
// carries an explicit budget: a stalled PATH scan or hung git must degrade
// instead of blocking admission, circuit reporting, or plugin setup.

function timedOut(message) {
  const error = new Error(message);
  error.code = 'ETIMEDOUT';
  return error;
}

function dispatch(task = 'work-real') {
  return { subagent_type: 'implement', prompt: '<minion-dispatch>' + JSON.stringify({
    contract_version: '1.0', task_id: task, role: 'implement', workflow: 'sdd-full', phase: 'apply',
    objective: 'Bounded work', spec_plane: 'openspec', allowed_files: ['one.go'],
    acceptance_checks: ['check'], artifact_refs: [], max_steps: 10,
  }) + '</minion-dispatch>' };
}

test('task latch bounds the PATH scan behind the circuit report', async () => {
  const calls = [];
  const mod = await loadPluginFile('internal/assets/plugins/cortex-task-latch.ts', {
    childProcess: {
      execFileSync: (file, args, opts) => {
        calls.push({ file, args, opts });
        if (args[0] !== 'report') throw timedOut('spawnSync ETIMEDOUT');
        return '';
      },
    },
  });
  const plugin = await mod.exports.CortexTaskLatchPlugin({ directory: '/isolated/harness' });
  const after = args => plugin['tool.execute.after'](
    { tool: 'task', sessionID: 'root', args }, { output: '' });
  const args = dispatch();
  for (let attempt = 1; attempt <= 4; attempt++) {
    await assert.rejects(after(args), attempt < 4 ? /SUBAGENT_ATTEMPT_FAILED/ : /CORTEX_CIRCUIT_OPEN/);
  }
  assert.equal(calls.length, 1, 'a stalled lookup must not spawn the report CLI');
  assert.match(calls[0].file, /^(where\.exe|which)$/);
  assert.equal(calls[0].opts.timeout, 3000);
  await plugin.dispose();
});

test('workload audit bounds the git diff and degrades to empty stats', async () => {
  const calls = [];
  const mod = await loadPluginFile('internal/assets/plugins/cortex-permission-fence.ts', {
    childProcess: {
      execFileSync: (file, args, opts) => {
        calls.push({ file, args, opts });
        throw timedOut('spawnSync ETIMEDOUT');
      },
    },
  });
  const result = mod.exports.auditWorkload('/isolated/harness', 'flexible', 'HEAD');
  assert.equal(calls.length, 1);
  assert.equal(calls[0].file, 'git');
  assert.equal(calls[0].opts.timeout, 10000);
  assert.equal(result.workload_status, 'PASS');
  assert.equal(result.files_changed, 0);
});

test('cortex project lookup bounds its git spawns during plugin setup', async () => {
  const calls = [];
  const load = async spawnSync => loadPluginFile('internal/assets/plugins/cortex.ts', {
    env: { CORTEX_HTTP_TOKEN: '' },
    globals: { Bun: { spawnSync } },
    childProcess: { execFileSync: () => '' },
  });

  const fallback = await load((cmd, opts) => { calls.push(opts); return { exitCode: 1 }; });
  assert.equal(fallback.exports.extractProjectName('/isolated/harness'), 'harness');
  assert.equal(calls.length, 2);

  const success = await load((cmd, opts) => {
    calls.push(opts);
    return { exitCode: 0, stdout: { toString: () => 'git@github.com:lleontor705/cortex-ia.git' } };
  });
  assert.equal(success.exports.extractProjectName('/isolated/work'), 'cortex-ia');
  assert.equal(calls.length, 3);
  for (const opts of calls) assert.equal(opts.timeout, 3000);
});
