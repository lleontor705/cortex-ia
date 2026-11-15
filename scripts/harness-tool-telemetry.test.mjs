import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const pluginPath = 'internal/assets/plugins/cortex-tool-telemetry.ts';
const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');

function telemetryOptions(execFileCallback) {
  return {
    cwd: root,
    env: { USERPROFILE: home, HOME: home },
    fs: {
      existsSync: p => p === executable,
      realpathSync: p => p,
      lstatSync: () => ({ isSymbolicLink: () => false }),
      readFileSync: () => '',
      writeFileSync: () => {},
      mkdirSync: () => {},
    },
    childProcess: {
      execFileSync: () => executable,
      execFile: (_file, args, _opts, cb) => {
        execFileCallback(args);
        if (typeof cb === 'function') cb(null, '', '');
      },
    },
  };
}

async function createPlugin(calls) {
  const mod = await loadPluginFile(pluginPath, telemetryOptions(args => calls.push(args)));
  const instance = await mod.instantiate({ directory: root });
  return instance;
}

test('cortex-tool-telemetry classifies LEASE_REQUIRED as ERR_TOOL_LEASE_REQUIRED', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'edit', sessionID: 'ses-1', callID: 'call-1', args: { targetFile: 'main.go' } },
    { output: 'LEASE_REQUIRED: all native mutation targets require a live session-owned claim' }
  );

  assert.equal(calls.length, 1);
  const args = calls[0];
  assert.equal(args[0], 'report');
  assert.equal(args[1], 'error');
  assert.ok(args.includes('--code'));
  assert.equal(args[args.indexOf('--code') + 1], 'ERR_TOOL_LEASE_REQUIRED');
  assert.ok(args[args.indexOf('--message') + 1].includes("Tool 'edit' failed"));
  await plugin.dispose();
});

test('cortex-tool-telemetry classifies MCP tool errors as ERR_TOOL_MCP_FAILED', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'cortex_save', sessionID: 'ses-2', callID: 'call-2', args: { type: 'decision' } },
    { output: '{"error":"storage write error: database is locked"}' }
  );

  assert.equal(calls.length, 1);
  const args = calls[0];
  assert.equal(args[args.indexOf('--code') + 1], 'ERR_TOOL_MCP_FAILED');
  await plugin.dispose();
});

test('cortex-tool-telemetry classifies argument validation as ERR_TOOL_INVALID_ARGS', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'replace_file_content', sessionID: 'ses-3', callID: 'call-3', args: {} },
    { output: 'Error: invalid argument: TargetFile is missing or empty' }
  );

  assert.equal(calls.length, 1);
  const args = calls[0];
  assert.equal(args[args.indexOf('--code') + 1], 'ERR_TOOL_INVALID_ARGS');
  await plugin.dispose();
});

test('cortex-tool-telemetry classifies cortex_* argument errors as ERR_TOOL_INVALID_ARGS', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'cortex_cortex_save', sessionID: 'ses-mcp-args', callID: 'call-mcp-args-1', args: {} },
    { output: 'Error: invalid argument: content is required' }
  );
  assert.equal(calls.length, 1);
  assert.equal(
    calls[0][calls[0].indexOf('--code') + 1],
    'ERR_TOOL_INVALID_ARGS',
    'an argument error from a cortex_* MCP tool must not be bucketed as a persistence failure'
  );

  await plugin['tool.execute.after'](
    { tool: 'cortex_cortex_relate', sessionID: 'ses-mcp-args', callID: 'call-mcp-args-2', args: {} },
    { output: '{"error":"missing required field: topic_key"}' }
  );
  assert.equal(calls.length, 2);
  assert.equal(calls[1][calls[1].indexOf('--code') + 1], 'ERR_TOOL_INVALID_ARGS');

  await plugin['tool.execute.after'](
    { tool: 'cortex_cortex_save', sessionID: 'ses-mcp-args', callID: 'call-mcp-args-3', args: {} },
    { output: '{"error":"Failed to save: write could not be persisted"}' }
  );
  assert.equal(calls.length, 3);
  assert.equal(
    calls[2][calls[2].indexOf('--code') + 1],
    'ERR_TOOL_MCP_FAILED',
    'a genuine persistence failure must stay in the MCP bucket'
  );

  await plugin.dispose();
});

test('cortex-tool-telemetry classifies generic tool failures as ERR_TOOL_EXECUTION_FAILED', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_work_transition', sessionID: 'ses-4', callID: 'call-4', args: { task_id: 'task-100', to: 'in_review' } },
    { output: '{"error":"ErrWorkConflict: task must be in_progress"}' }
  );

  assert.equal(calls.length, 1);
  const args = calls[0];
  assert.equal(args[args.indexOf('--code') + 1], 'ERR_TOOL_EXECUTION_FAILED');
  assert.equal(args[args.indexOf('--task') + 1], 'task-100');
  await plugin.dispose();
});

test('cortex-tool-telemetry classifies bridge authority failures as ERR_TOOL_AUTHORITY_UNUSABLE', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  // Shape produced by cortex-work.ts authorityFailure(): the code names the
  // refused authority, bridge_authority carries the view, action tells the
  // controller to reconcile before retrying.
  const authorityFailure = code => JSON.stringify({
    code,
    task_id: 'task-char-harness',
    durable_status: 'in_progress',
    claim_expires_at: '2026-10-05T10:00:00Z',
    bridge_authority: {
      handle_present: true,
      owned_by_current_session: false,
      durable_claim_live: false,
      usable: false,
      action: 'STOP_WRITING_AND_RECONCILE',
    },
    action: 'RECONCILE_WORK_THEN_RETRY_WITH_FRESH_AUTHORITY',
  });

  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_work_claim', sessionID: 'ses-auth', callID: 'call-auth-1', args: { task_id: 'task-char-harness' } },
    { output: { isError: true, content: [{ type: 'text', text: authorityFailure('BRIDGE_AUTHORITY_UNUSABLE') }] } }
  );
  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_work_transition', sessionID: 'ses-auth', callID: 'call-auth-2', args: { task_id: 'task-char-harness' } },
    { output: { isError: true, content: [{ type: 'text', text: authorityFailure('BRIDGE_WRITE_AUTHORITY_UNUSABLE') }] } }
  );
  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_work_status', sessionID: 'ses-auth', callID: 'call-auth-3', args: {} },
    { output: { isError: true, content: [{ type: 'text', text: authorityFailure('WORK_STATUS_UNAVAILABLE') }] } }
  );

  assert.equal(calls.length, 3);
  for (const args of calls) {
    assert.equal(
      args[args.indexOf('--code') + 1],
      'ERR_TOOL_AUTHORITY_UNUSABLE',
      'a refused authority must not fall into the execution catch-all'
    );
  }
  assert.equal(calls[0][calls[0].indexOf('--task') + 1], 'task-char-harness');
  assert.ok(
    calls[0][calls[0].indexOf('--message') + 1].includes("Tool 'cortex_ia_work_claim' failed"),
    'the report keeps the failing tool name for triage'
  );

  // The lease guard is a different failure class: no claim exists at all.
  await plugin['tool.execute.after'](
    { tool: 'edit', sessionID: 'ses-auth', callID: 'call-auth-4', args: { path: 'main.go' } },
    { output: 'LEASE_REQUIRED: all native mutation targets require a live session-owned claim' }
  );
  assert.equal(calls.length, 4);
  assert.equal(
    calls[3][calls[3].indexOf('--code') + 1],
    'ERR_TOOL_LEASE_REQUIRED',
    'the lease guard must keep its own classification'
  );

  await plugin.dispose();
});

test('cortex-tool-telemetry classifies deadline failures as ERR_TOOL_TIMEOUT', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  // Message produced by cortex-snapshot.ts when an export deadline fires: a
  // blown ceiling must reach the monitor as its own code, not the catch-all.
  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_snapshot_read', sessionID: 'ses-timeout', callID: 'call-timeout-1', args: { project: 'p', observation_id: 7 } },
    { output: '{"error":"Local Cortex snapshot read timed out at the Cortex-IA export deadline; no verified snapshot available"}' }
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0][calls[0].indexOf('--code') + 1], 'ERR_TOOL_TIMEOUT');

  // Precedence: a memory backend that times out is still a persistence failure.
  await plugin['tool.execute.after'](
    { tool: 'cortex_save', sessionID: 'ses-timeout', callID: 'call-timeout-2', args: {} },
    { output: '{"error":"storage write error: cortex_save timed out"}' }
  );
  assert.equal(calls.length, 2);
  assert.equal(calls[1][calls[1].indexOf('--code') + 1], 'ERR_TOOL_MCP_FAILED');

  await plugin.dispose();
});

test('cortex-tool-telemetry debounces duplicate identical errors', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  const input = { tool: 'edit', sessionID: 'ses-dup', callID: 'call-dup-1', args: { path: 'file.go' } };
  const output = { output: 'Error: target content not found' };

  await plugin['tool.execute.after'](input, output);
  await plugin['tool.execute.after']({ ...input, callID: 'call-dup-2' }, output);

  assert.equal(calls.length, 1);
  await plugin.dispose();
});

test('cortex-tool-telemetry keeps the failure detail behind boolean/object error flags', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'execute', sessionID: 'ses-flag', callID: 'call-flag-1', args: { code: 'await work()' } },
    { output: { error: true, content: [{ type: 'text', text: 'authority for task-f0 is already held by this controller' }] } }
  );
  assert.equal(calls.length, 1);
  assert.ok(
    calls[0][calls[0].indexOf('--message') + 1].includes('already held by this controller'),
    'content next to a boolean error flag must be reported instead of the flag itself'
  );

  await plugin['tool.execute.after'](
    { tool: 'execute', sessionID: 'ses-obj', callID: 'call-obj-1', args: {} },
    { output: { error: { code: 'WORK_CONFLICT', message: 'task must be ready' } } }
  );
  assert.equal(calls.length, 2);
  assert.ok(
    calls[1][calls[1].indexOf('--message') + 1].includes('task must be ready'),
    'a structured error object must report its message instead of [object Object]'
  );

  await plugin['tool.execute.after'](
    { tool: 'execute', sessionID: 'ses-bare', callID: 'call-bare-1', args: {} },
    { output: { error: true } }
  );
  assert.equal(calls.length, 3);
  const bareMessage = calls[2][calls[2].indexOf('--message') + 1];
  assert.ok(!bareMessage.endsWith(': true'), `bare error flag must not be reported as "true": ${bareMessage}`);
  assert.equal(calls[2][calls[2].indexOf('--code') + 1], 'ERR_TOOL_EXECUTION_FAILED');

  await plugin.dispose();
});

test('cortex-tool-telemetry ignores successful executions and self-reporting', async () => {
  const calls = [];
  const plugin = await createPlugin(calls);

  await plugin['tool.execute.after'](
    { tool: 'read', sessionID: 'ses-ok', callID: 'call-ok', args: { path: 'main.go' } },
    { output: 'package main\n\nfunc main() {}' }
  );
  await plugin['tool.execute.after'](
    { tool: 'cortex_ia_report_error', sessionID: 'ses-rep', callID: 'call-rep', args: { code: 'ERR_TASK_BLOCKED' } },
    { output: '{"status":"ok"}' }
  );

  assert.equal(calls.length, 0);
  await plugin.dispose();
});
