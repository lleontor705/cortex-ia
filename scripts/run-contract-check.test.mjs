import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { parseGoEvents, runContractCheck } from './run-contract-check.mjs';

describe('run-contract-check', () => {
  it('parses Go JSON events and confirms all required cases pass', () => {
    const events = [
      JSON.stringify({ Time: '2026-09-12T00:00:00Z', Action: 'run', Package: 'pkg', Test: 'TestFoo' }),
      JSON.stringify({ Time: '2026-09-12T00:00:01Z', Action: 'pass', Package: 'pkg', Test: 'TestFoo' }),
      JSON.stringify({ Time: '2026-09-12T00:00:02Z', Action: 'run', Package: 'pkg', Test: 'TestBar' }),
      JSON.stringify({ Time: '2026-09-12T00:00:03Z', Action: 'pass', Package: 'pkg', Test: 'TestBar' }),
    ];

    const res = parseGoEvents(events, ['TestFoo', 'TestBar']);
    assert.equal(res.valid, true);
    assert.deepEqual(res.executedCases, ['TestFoo', 'TestBar']);
  });

  it('rejects when a required test case is missing/absent', () => {
    const events = [
      JSON.stringify({ Time: '2026-09-12T00:00:00Z', Action: 'run', Package: 'pkg', Test: 'TestFoo' }),
      JSON.stringify({ Time: '2026-09-12T00:00:01Z', Action: 'pass', Package: 'pkg', Test: 'TestFoo' }),
    ];

    assert.throws(
      () => parseGoEvents(events, ['TestFoo', 'TestMissing']),
      /Required test cases were not executed: TestMissing/
    );
  });

  it('rejects when a required test case is skipped', () => {
    const events = [
      JSON.stringify({ Time: '2026-09-12T00:00:00Z', Action: 'run', Package: 'pkg', Test: 'TestFoo' }),
      JSON.stringify({ Time: '2026-09-12T00:00:01Z', Action: 'skip', Package: 'pkg', Test: 'TestFoo' }),
    ];

    assert.throws(
      () => parseGoEvents(events, ['TestFoo']),
      /Required test cases were skipped: TestFoo/
    );
  });

  it('rejects when an event is malformed JSON or empty', () => {
    assert.throws(
      () => parseGoEvents([], ['TestFoo']),
      /Missing test events: empty event stream/
    );
    assert.throws(
      () => parseGoEvents(['not valid json'], ['TestFoo']),
      /Malformed JSON event/
    );
  });

  it('runs synthetic child process and confirms contract check', async () => {
    const script = `
      console.log(JSON.stringify({ Action: 'run', Test: 'TestSynthetic' }));
      console.log(JSON.stringify({ Action: 'pass', Test: 'TestSynthetic' }));
    `;

    const res = await runContractCheck({
      cases: ['TestSynthetic'],
      command: process.execPath,
      cmdArgs: ['-e', script],
    });

    assert.equal(res.valid, true);
    assert.deepEqual(res.executedCases, ['TestSynthetic']);
  });

  it('propagates nonzero child exit code', async () => {
    await assert.rejects(
      () =>
        runContractCheck({
          cases: ['TestFail'],
          command: process.execPath,
          cmdArgs: ['-e', 'process.exit(2)'],
        }),
      /Child command exited with code 2/
    );
  });

  it('rejects full-suite execution without I20', async () => {
    await assert.rejects(
      () =>
        runContractCheck({
          cases: ['TestA'],
          command: 'go',
          cmdArgs: ['test', './...'],
        }),
      /Full-suite execution rejected without I20/
    );
  });
});
