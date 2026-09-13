import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  captureLocalBaseline,
  compareBaselines,
  runSelfTest,
} from './capture-local-baseline.mjs';

describe('captureLocalBaseline', () => {
  it('captures comparable baseline with revision, scoped hashes, OS and exit', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'test-base-'));
    try {
      const src = path.join(tmp, 'file.js');
      fs.writeFileSync(src, 'console.log("hello");', 'utf8');
      const out = path.join(tmp, 'out.json');

      const res = captureLocalBaseline({
        command: process.execPath,
        args: ['-e', 'process.exit(0)'],
        scopedFiles: ['file.js'],
        cwd: tmp,
        outputPath: out,
      });

      assert.equal(res.oracle.actual_exit, 0);
      assert.equal(res.oracle.verdict, 'MATCH');
      assert.ok(res.environment.platform);
      assert.ok(res.scoped_files['file.js']);
      assert.ok(fs.existsSync(out));
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  it('rejects changed inputs from claiming comparability', () => {
    const base1 = {
      oracle: { full_command: 'cmd A', actual_exit: 0 },
      scoped_files: { 'f1.js': 'h1' },
    };
    const base2 = {
      oracle: { full_command: 'cmd B', actual_exit: 0 },
      scoped_files: { 'f1.js': 'h1' },
    };
    const base3 = {
      oracle: { full_command: 'cmd A', actual_exit: 0 },
      scoped_files: { 'f2.js': 'h2' },
    };

    assert.equal(compareBaselines(base1, base2).comparable, false);
    assert.equal(compareBaselines(base1, base3).comparable, false);
    assert.equal(compareBaselines(base1, base1).comparable, true);
  });

  it('distinguishes non-reproduction and failure from PASS', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'test-nonrep-'));
    try {
      const res = captureLocalBaseline({
        command: process.execPath,
        args: ['-e', 'process.exit(2)'],
        expectedExit: 0,
        cwd: tmp,
      });

      assert.equal(res.oracle.actual_exit, 2);
      assert.equal(res.oracle.verdict, 'NON_REPRODUCTION');
      assert.notEqual(res.oracle.verdict, 'PASS');
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  it('rejects empty output paths', () => {
    assert.throws(
      () => captureLocalBaseline({ command: process.execPath, outputPath: '' }),
      /Empty output path rejected/
    );
    assert.throws(
      () => captureLocalBaseline({ command: process.execPath, outputPath: '   ' }),
      /Empty output path rejected/
    );
  });

  it('rejects fabricated savings and telemetry', () => {
    assert.throws(
      () => captureLocalBaseline({ command: process.execPath, fabricatedSavings: '50% faster' }),
      /Fabricated speed\/token savings rejected/
    );
    assert.throws(
      () => captureLocalBaseline({ command: process.execPath, enableTelemetry: true }),
      /Remote telemetry and collection rejected/
    );
  });

  it('rejects full suite execution before I20', () => {
    assert.throws(
      () => captureLocalBaseline({ command: 'go', args: ['test', './...'] }),
      /Full-suite execution rejected before I20/
    );
    assert.throws(
      () => captureLocalBaseline({ command: 'npm', args: ['test'] }),
      /Full-suite execution rejected before I20/
    );
  });

  it('executes self-test cleanly', () => {
    assert.equal(runSelfTest(), true);
  });
});
