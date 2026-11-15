import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const BRIDGE = 'internal/assets/plugins/cortex-work.ts';
const ROOT = path.resolve('/isolated/harness');
const HOME = path.resolve('/isolated/home');
const EXECUTABLE = path.join(HOME, 'go', 'bin', 'cortex-ia.exe');
const MARKER = '[CORTEX-IA STATE SNAPSHOT]';

function workListOutput() {
  return JSON.stringify([
    { task_id: 'w2-04', status: 'in_progress', title: 'Compaction possession' },
    { task_id: 'w2-06', status: 'blocked', title: 'Authority frontmatter' },
    { task_id: 'w2-07', status: 'ready', title: 'Minion surface' },
  ]);
}

function options(control = {}) {
  const calls = [];
  return {
    calls, cwd: ROOT, env: { USERPROFILE: HOME, HOME },
    fs: {
      existsSync: p => p === EXECUTABLE,
      readFileSync: () => '',
      realpathSync: p => p,
      lstatSync: () => ({ isSymbolicLink: () => false }),
      mkdirSync: () => {},
      writeFileSync: () => {},
    },
    childProcess: {
      execFileSync: (file, args) => {
        calls.push({ file, args: Array.from(args) });
        if (args[0] === 'work' && args[1] === 'list') {
          if (control.listFails) throw new Error('durable work list unavailable');
          return control.listOutput ?? workListOutput();
        }
        if (args[0] === 'version') return '';
        return '{}';
      },
    },
  };
}

async function bridgeFor(control = {}) {
  const opts = options(control);
  const hooks = [];
  const ctx = { client: {}, session: { hook: (name, handler) => hooks.push({ name, handler }) } };
  const bridge = await (await loadPluginFile(BRIDGE, opts)).instantiate(ctx);
  return {
    opts, bridge, hooks,
    compaction: hooks.find(h => h.name === 'compaction')?.handler,
    retry: hooks.find(h => h.name === 'retry')?.handler,
  };
}

const transient = (attempt, error) => ({ sessionID: 'sess-retry', attempt, error, decision: {} });

test('TestREQ_COMP_001 possesses the compaction summary deterministically via event.result', async () => {
  const { opts, bridge, compaction } = await bridgeFor();
  try {
    assert.equal(typeof compaction, 'function', 'the plugin must register the compaction hook');

    const event = { sessionID: 'sess-1', result: {} };
    await compaction(event);
    assert.equal(typeof event.result.summary, 'string');
    assert.ok(event.result.summary.includes(MARKER), 'the possessed summary must carry the DAG snapshot marker');
    assert.ok(event.result.summary.includes('w2-04'), 'the summary must embed the live work DAG state');
    assert.ok(event.result.summary.includes('Continuation'), 'the summary must include a continuation pointer');
    assert.ok(event.result.summary.includes('w2-06'), 'blocked tasks must be represented in the snapshot');

    // Determinism: an identical work list yields a byte-identical summary.
    const second = { sessionID: 'sess-1', result: {} };
    await compaction(second);
    assert.equal(second.result.summary, event.result.summary);

    // Possession owns summarization; the legacy injection path stays unused.
    assert.deepEqual(event.context, undefined);
    assert.deepEqual(event.messages, undefined);
    assert.equal(opts.calls.filter(c => c.args[0] === 'work' && c.args[1] === 'list').length, 2);
  } finally {
    await bridge.dispose();
  }
});

test('TestREQ_COMP_002 falls back to injection when event.result is unsupported or the read fails', async () => {
  const supported = await bridgeFor();
  try {
    const contextEvent = { sessionID: 'sess-2', context: [] };
    await supported.compaction(contextEvent);
    assert.equal(contextEvent.context.length, 1);
    assert.ok(contextEvent.context[0].includes(MARKER));
    assert.ok(contextEvent.context[0].includes('w2-04'));
    assert.ok(!('result' in contextEvent), 'an unsupported runtime must not receive a fabricated result');

    const messageEvent = { sessionID: 'sess-2', messages: [] };
    await supported.compaction(messageEvent);
    assert.equal(messageEvent.messages.length, 1);
    assert.equal(messageEvent.messages[0].role, 'system');
    assert.ok(messageEvent.messages[0].content.includes(MARKER));
  } finally {
    await supported.bridge.dispose();
  }

  const failing = await bridgeFor({ listFails: true });
  try {
    const event = { sessionID: 'sess-3', context: [] };
    await assert.doesNotReject(() => failing.compaction(event), 'a read failure must never escape the hook');
    assert.equal(event.context.length, 1);
    assert.ok(event.context[0].includes(MARKER));
  } finally {
    await failing.bridge.dispose();
  }
});

test('TestREQ_COMP_003 grants bounded transient retries and passes other failures through', async () => {
  const { bridge, retry } = await bridgeFor();
  try {
    assert.equal(typeof retry, 'function', 'the plugin must register the retry hook');

    const first = transient(1, { status: 429, message: 'Too Many Requests' });
    await retry(first);
    assert.equal(first.decision.retry, true);
    assert.ok(first.decision.delay > 0);

    const second = transient(2, { message: 'rate limit exceeded' });
    await retry(second);
    assert.equal(second.decision.retry, true);
    assert.ok(second.decision.delay > first.decision.delay, 'the delay must back off exponentially');

    const third = transient(3, { message: 'rate limit exceeded' });
    await retry(third);
    assert.equal(third.decision.retry, false, 'the retry cap must veto at attempt 3');

    const quota = transient(4, { code: 'QUOTA_EXCEEDED', message: 'usage limit has been reached' });
    await retry(quota);
    assert.equal(quota.decision.retry, false, 'quota exhaustion must surface rather than loop');

    const serverError = transient(1, { statusCode: 503 });
    await retry(serverError);
    assert.equal(serverError.decision.retry, true);
    assert.ok(serverError.decision.delay > 0);

    for (const error of [{ status: 400, message: 'bad request' }, { status: 401, message: 'unauthorized' }, { name: 'AbortError' }]) {
      const event = transient(1, error);
      await retry(event);
      assert.ok(!('retry' in event.decision), `non-transient '${error.message ?? error.name}' must pass through unchanged`);
    }
  } finally {
    await bridge.dispose();
  }
});
