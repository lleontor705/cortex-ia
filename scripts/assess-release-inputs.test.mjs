import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {
  detectRealSecrets,
  classifyLinkerInput,
  assessReleaseInputs,
  runCLI,
} from './assess-release-inputs.mjs';

test('detectRealSecrets identifies populated sensitive environment variables', () => {
  const dirtyEnv = { GITHUB_TOKEN: 'ghp_fakeTokenString1234567890' };
  const violations = detectRealSecrets(dirtyEnv);
  assert.ok(violations.length > 0);
  assert.equal(violations[0].source, 'env.GITHUB_TOKEN');

  const cleanEnv = { PATH: '/usr/bin', NODE_ENV: 'test' };
  const cleanViolations = detectRealSecrets(cleanEnv);
  assert.equal(cleanViolations.length, 0);
});

test('detectRealSecrets catches real secret patterns in extra input arguments', () => {
  const dirtyInputs = ['-X main.token=ghp_0123456789abcdef0123456789abcdef'];
  const violations = detectRealSecrets({}, dirtyInputs);
  assert.ok(violations.length > 0);
  assert.equal(violations[0].source, 'input');
});

test('classifyLinkerInput separates public identifiers from authority-bearing symbols', () => {
  const versionInput = classifyLinkerInput('-X main.version=v1.2.3');
  assert.equal(versionInput.category, 'public_identifier');
  assert.equal(versionInput.authority, false);
  assert.equal(versionInput.value, 'v1.2.3');

  const commitInput = classifyLinkerInput('-X main.commit=abcdef0123456789');
  assert.equal(commitInput.category, 'public_identifier');

  const secretInput = classifyLinkerInput('-X main.signingSecret=sensitiveValue123');
  assert.equal(secretInput.category, 'authority_material');
  assert.equal(secretInput.authority, true);
  assert.equal(secretInput.value, '[REDACTED-POTENTIAL-SECRET]');
});

test('assessReleaseInputs rejects real secrets under syntheticOnly mode', () => {
  const badEnv = { GITHUB_TOKEN: 'token_present' };
  assert.throws(() => {
    assessReleaseInputs({ env: badEnv, syntheticOnly: true });
  }, /SECRET_LEAK_PREVENTION/);
});

test('assessReleaseInputs produces structured report with unverified privilege status and isolated output', () => {
  const cleanEnv = {};
  const report = assessReleaseInputs({
    env: cleanEnv,
    syntheticOnly: true,
    isolatedOutput: true,
  });

  assert.equal(report.mode, 'synthetic_assessment');
  assert.equal(report.privilege_status, 'unverified');
  assert.equal(report.network_called, false);
  assert.equal(report.environment_scrubbed, true);
  assert.ok(report.outputPath);
  assert.ok(fs.existsSync(report.outputPath));

  // Ensure isolated output is in a temp directory, not production paths
  const resolvedOut = path.resolve(report.outputPath);
  assert.ok(resolvedOut.startsWith(path.resolve(os.tmpdir())));

  // Clean up the temporary file
  fs.rmSync(path.dirname(report.outputPath), { recursive: true, force: true });
});

test('runCLI executes cleanly with synthetic-only and isolated-output flags', async () => {
  const origEnv = process.env;
  try {
    const scrubbed = { ...origEnv };
    delete scrubbed.GITHUB_TOKEN;
    delete scrubbed.GH_TOKEN;
    delete scrubbed.RELEASE_KEY;
    delete scrubbed.SIGNING_KEY;
    process.env = scrubbed;

    const result = await runCLI(['--synthetic-only', '--isolated-output']);
    assert.equal(result.privilege_status, 'unverified');
    assert.equal(result.network_called, false);
    if (result.outputPath && fs.existsSync(result.outputPath)) {
      fs.rmSync(path.dirname(result.outputPath), { recursive: true, force: true });
    }
  } finally {
    process.env = origEnv;
  }
});
