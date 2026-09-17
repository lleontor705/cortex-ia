import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });

async function harness(jobState) {
  let job = { ...jobState };
  const calls = [];
  const mod = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    fs: { existsSync: () => true, readFileSync: () => '', mkdirSync: () => {}, writeFileSync: () => {} },
    childProcess: {
      execFileSync: (_file, args) => {
        calls.push(args);
        if (args[0] === 'delegate') {
          if (args[1] === 'query') {
            return JSON.stringify({
              ...job,
              receipt_available: !!job.receipt,
              receipt: job.receipt || undefined,
              receipt_missing: !job.receipt && ['succeeded', 'failed', 'cancelled', 'timed_out', 'lost'].includes(job.status)
            });
          }
          if (args[1] === 'status') {
            return JSON.stringify(job);
          }
          if (args[1] === 'result') {
            if (job.receipt) return JSON.stringify(job.receipt);
            throw new Error('receipt not found');
          }
        }
        return '{}';
      }
    }
  });
  const plugin = await mod.instantiate({ client: {} });
  return {
    tools: plugin.tool,
    calls,
    setJob: next => { job = { ...next }; }
  };
}

test('coherent delegation_result retrieves receipt in a single process call', async () => {
  const receipt = { job_id: 'job-1', status: 'succeeded', exit_code: 0, output: { verdict: 'PASS' } };
  const h = await harness({ job_id: 'job-1', status: 'succeeded', role: 'implement', transport: 'direct', receipt });

  const resStr = await h.tools.cortex_ia_delegation_result.execute({ job_id: 'job-1' });
  const res = JSON.parse(resStr);

  assert.equal(res.status, 'succeeded');
  assert.equal(res.exit_code, 0);
  assert.deepEqual(res.output, { verdict: 'PASS' });

  // Assert single process call for terminal retrieval (query instead of status + result)
  const delegateCalls = h.calls.filter(args => args[0] === 'delegate');
  assert.equal(delegateCalls.length, 1);
  assert.equal(delegateCalls[0][0], 'delegate');
  assert.equal(delegateCalls[0][1], 'query');
  assert.equal(delegateCalls[0][2], 'job-1');
});

test('coherent delegation_wait completes with receipt without extra result process call', async () => {
  const receipt = { job_id: 'job-2', status: 'succeeded', exit_code: 0, output: { summary: 'all tests pass' } };
  const h = await harness({ job_id: 'job-2', status: 'succeeded', role: 'implement', transport: 'direct', receipt });

  const waitResStr = await h.tools.cortex_ia_delegation_wait.execute({ job_id: 'job-2', timeout_seconds: 5 });
  const waitRes = JSON.parse(waitResStr);

  assert.equal(waitRes.status, 'succeeded');
  assert.deepEqual(waitRes.result, receipt);

  // Assert single process call in wait
  const delegateCalls = h.calls.filter(args => args[0] === 'delegate');
  assert.equal(delegateCalls.length, 1);
  assert.equal(delegateCalls[0][0], 'delegate');
  assert.equal(delegateCalls[0][1], 'query');
  assert.equal(delegateCalls[0][2], 'job-2');
});

test('explicit receipt_missing when terminal job has no receipt recorded', async () => {
  const h = await harness({ job_id: 'job-3', status: 'failed', role: 'implement', transport: 'direct' });

  const resStr = await h.tools.cortex_ia_delegation_result.execute({ job_id: 'job-3' });
  const res = JSON.parse(resStr);

  assert.equal(res.status, 'failed');
  assert.match(res.error, /receipt_missing/);
});

test('pending cancellation reports in progress without claiming completion', async () => {
  const h = await harness({ job_id: 'job-4', status: 'running', cancellation_requested: true, role: 'implement' });

  const resStr = await h.tools.cortex_ia_delegation_result.execute({ job_id: 'job-4' });
  const res = JSON.parse(resStr);

  assert.equal(res.completed, false);
  assert.equal(res.cancellation_requested, true);
  assert.equal(res.status, 'running');
});
