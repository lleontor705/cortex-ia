import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync, execSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export function hashFile(filePath) {
  const content = fs.readFileSync(filePath);
  return crypto.createHash('sha256').update(content).digest('hex');
}

export function getGitRevision(cwd = process.cwd()) {
  try {
    return execSync('git rev-parse HEAD', { cwd, encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'] }).trim();
  } catch {
    return 'untracked';
  }
}

export function captureLocalBaseline(opts = {}) {
  const {
    command,
    args = [],
    scopedFiles = [],
    cwd = process.cwd(),
    outputPath,
    allowFullSuite = false,
    hasI20Evidence = false,
    expectedExit = 0,
    fabricatedSavings = null,
    enableTelemetry = false,
  } = opts;

  if (outputPath !== undefined && (!outputPath || outputPath.trim() === '')) {
    throw new Error('Empty output path rejected');
  }

  if (fabricatedSavings !== null && fabricatedSavings !== undefined) {
    throw new Error('Fabricated speed/token savings rejected');
  }

  if (enableTelemetry) {
    throw new Error('Remote telemetry and collection rejected');
  }

  if (!command) {
    throw new Error('Command is required');
  }

  const fullCmd = [command, ...args].filter(Boolean).join(' ');

  // Reject unmeasured full-suite execution before I20
  if (!allowFullSuite && !hasI20Evidence) {
    const isFullSuite = /^(?:go\s+test\s+\.\/\.\.\.|npm\s+test|npm\s+run\s+test:all)/i.test(fullCmd.trim());
    if (isFullSuite) {
      throw new Error('Full-suite execution rejected before I20 isolated fixture evidence');
    }
  }

  const fileHashes = {};
  for (const rel of scopedFiles) {
    const abs = path.resolve(cwd, rel);
    if (fs.existsSync(abs) && fs.statSync(abs).isFile()) {
      fileHashes[rel] = hashFile(abs);
    }
  }

  const startTime = new Date().toISOString();
  let exitCode = 0;
  let stdout = '';
  let stderr = '';
  let errorMsg = null;

  try {
    stdout = execFileSync(command, args, { cwd, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] });
  } catch (err) {
    exitCode = err.status ?? (err.code ? 1 : 0);
    stdout = err.stdout?.toString() || '';
    stderr = err.stderr?.toString() || '';
    errorMsg = err.message;
  }
  const endTime = new Date().toISOString();

  let verdict = 'UNKNOWN';
  if (exitCode === expectedExit) {
    verdict = 'MATCH';
  } else {
    verdict = 'NON_REPRODUCTION';
  }

  const record = {
    schema_version: '1.0',
    timestamp: startTime,
    duration_ms: new Date(endTime).getTime() - new Date(startTime).getTime(),
    environment: {
      platform: process.platform,
      arch: process.arch,
      node_version: process.version,
      revision: getGitRevision(cwd),
    },
    oracle: {
      command,
      args,
      full_command: fullCmd,
      expected_exit: expectedExit,
      actual_exit: exitCode,
      verdict,
      error: errorMsg,
    },
    scoped_files: fileHashes,
    output: {
      stdout_len: stdout.length,
      stderr_len: stderr.length,
    },
  };

  if (outputPath) {
    const absOut = path.resolve(cwd, outputPath);
    fs.mkdirSync(path.dirname(absOut), { recursive: true });
    fs.writeFileSync(absOut, JSON.stringify(record, null, 2), 'utf8');
  }

  return record;
}

export function compareBaselines(baselineBefore, baselineAfter) {
  if (!baselineBefore || !baselineAfter) {
    throw new Error('Both baselines are required for comparison');
  }

  const cmdBefore = baselineBefore.oracle?.full_command;
  const cmdAfter = baselineAfter.oracle?.full_command;

  if (cmdBefore !== cmdAfter) {
    return {
      comparable: false,
      reason: 'changed commands cannot claim comparability',
    };
  }

  const filesBefore = Object.keys(baselineBefore.scoped_files || {}).sort();
  const filesAfter = Object.keys(baselineAfter.scoped_files || {}).sort();
  if (JSON.stringify(filesBefore) !== JSON.stringify(filesAfter)) {
    return {
      comparable: false,
      reason: 'changed inputs cannot claim comparability',
    };
  }

  const exitChanged = baselineBefore.oracle.actual_exit !== baselineAfter.oracle.actual_exit;
  return {
    comparable: true,
    exit_changed: exitChanged,
    before_exit: baselineBefore.oracle.actual_exit,
    after_exit: baselineAfter.oracle.actual_exit,
    verdict_before: baselineBefore.oracle.verdict,
    verdict_after: baselineAfter.oracle.verdict,
  };
}

export function runSelfTest() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'baseline-self-test-'));
  try {
    const testFile = path.join(tmpDir, 'test.txt');
    fs.writeFileSync(testFile, 'initial content', 'utf8');

    const outPath = path.join(tmpDir, 'baseline.json');
    const rec = captureLocalBaseline({
      command: process.execPath,
      args: ['-e', 'process.exit(0)'],
      scopedFiles: ['test.txt'],
      cwd: tmpDir,
      outputPath: outPath,
    });

    if (rec.oracle.actual_exit !== 0 || !fs.existsSync(outPath)) {
      throw new Error('Self-test capture failed');
    }

    const comp = compareBaselines(rec, rec);
    if (!comp.comparable) {
      throw new Error('Self-test comparison failed');
    }

    console.log('[capture-local-baseline] Self-test PASS: schema, hashes, and comparison verified.');
    return true;
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  if (process.argv.includes('--self-test')) {
    runSelfTest();
    process.exit(0);
  }
  console.log('Usage: node scripts/capture-local-baseline.mjs --self-test');
}
