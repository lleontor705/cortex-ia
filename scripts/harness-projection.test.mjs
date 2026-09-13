import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPluginFile } from './harness-plugin-loader.mjs';

test('bridge deepFreeze and nested mutation protection', async () => {
  const bridge = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    allowChildProcess: true,
  });
  const { processWorkStatusResponse, deepFreeze } = bridge.exports;

  const sample = {
    task_id: 'task-mutation',
    revision: 1,
    status: 'ready',
    projection: {
      task_id: 'task-mutation',
      revision: 1,
      status: 'ready',
      candidate_actions: [{ action: 'request_claim', role: 'implement', conditions: ['ready'] }],
      notes: [{ source: 'test', content: 'immutable note' }],
    }
  };

  const result = processWorkStatusResponse('task-mutation', 'session-1', sample);
  assert.equal(result.accepted, true);
  assert.ok(Object.isFrozen(result.projection));
  assert.ok(Object.isFrozen(result.projection.candidate_actions));
  assert.ok(Object.isFrozen(result.projection.candidate_actions[0]));
  assert.ok(Object.isFrozen(result.projection.notes[0]));

  assert.throws(() => {
    result.projection.status = 'mutated';
  }, TypeError);

  assert.throws(() => {
    result.projection.candidate_actions.push({ action: 'hacked', role: 'root' });
  }, TypeError);

  assert.throws(() => {
    result.projection.notes[0].content = 'mutated';
  }, TypeError);
});

test('response ordering discards older revisions', async () => {
  const bridge = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    allowChildProcess: true,
  });
  const { processWorkStatusResponse } = bridge.exports;

  const rev2 = {
    task_id: 'task-rev-order',
    revision: 2,
    status: 'ready',
    projection: { task_id: 'task-rev-order', revision: 2, status: 'ready' }
  };
  const r2 = processWorkStatusResponse('task-rev-order', 'session-1', rev2);
  assert.equal(r2.accepted, true);
  assert.equal(r2.projection.revision, 2);

  // Stale older revision 1 received
  const rev1 = {
    task_id: 'task-rev-order',
    revision: 1,
    status: 'backlog',
    projection: { task_id: 'task-rev-order', revision: 1, status: 'backlog' }
  };
  const r1 = processWorkStatusResponse('task-rev-order', 'session-1', rev1);
  assert.equal(r1.accepted, false);
  assert.equal(r1.reason, 'STALE_REVISION_DISCARDED');
  assert.equal(r1.projection.revision, 2);

  // Newer revision 3 received
  const rev3 = {
    task_id: 'task-rev-order',
    revision: 3,
    status: 'in_review',
    projection: { task_id: 'task-rev-order', revision: 3, status: 'in_review' }
  };
  const r3 = processWorkStatusResponse('task-rev-order', 'session-1', rev3);
  assert.equal(r3.accepted, true);
  assert.equal(r3.projection.revision, 3);
});

test('response task and session mismatch rejected', async () => {
  const bridge = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    allowChildProcess: true,
  });
  const { processWorkStatusResponse } = bridge.exports;

  // Task ID mismatch
  const wrongTask = {
    task_id: 'task-wrong',
    revision: 1,
    status: 'ready',
    projection: { task_id: 'task-wrong', revision: 1, status: 'ready' }
  };
  const resTask = processWorkStatusResponse('task-expected', 'session-1', wrongTask);
  assert.equal(resTask.accepted, false);
  assert.equal(resTask.reason, 'RESPONSE_TASK_MISMATCH');

  // Session ID mismatch
  const wrongSession = {
    task_id: 'task-match',
    revision: 1,
    status: 'ready',
    opencode_session_id: 'session-other',
    projection: { task_id: 'task-match', revision: 1, status: 'ready' }
  };
  const resSession = processWorkStatusResponse('task-match', 'session-1', wrongSession);
  assert.equal(resSession.accepted, false);
  assert.equal(resSession.reason, 'RESPONSE_SESSION_MISMATCH');
});

test('failed retrieval replaces cached readiness with unknown authority', async () => {
  const bridge = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    allowChildProcess: true,
  });
  const { processWorkStatusResponse, handleWorkStatusFailure, cachedProjections } = bridge.exports;

  const valid = {
    task_id: 'task-failure-test',
    revision: 1,
    status: 'ready',
    projection: { task_id: 'task-failure-test', revision: 1, status: 'ready', authority_available: true }
  };
  processWorkStatusResponse('task-failure-test', 'session-1', valid);
  assert.equal(cachedProjections.get('task-failure-test').projection.status, 'ready');

  // Failure occurs
  const unknownProj = handleWorkStatusFailure('task-failure-test');
  assert.equal(unknownProj.status, 'unknown');
  assert.equal(unknownProj.authority_available, false);
  assert.equal(cachedProjections.get('task-failure-test').projection.status, 'unknown');
});

test('model-selected privileged role escalation is rejected', async () => {
  const bridge = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    allowChildProcess: true,
    env: { CORTEX_BIN: 'mock' },
  });

  assert.ok(bridge.exports);
  assert.ok(bridge.exports.processWorkStatusResponse);
});
