import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import {
  CANONICAL_TARGETS,
  parseArgs,
  checkGoVersion,
  validateRecord,
  qualifyTargets,
  runTargetQualification
} from './run-target-qualification.mjs';

describe('Target Runner Qualification Suite', () => {
  test('canonical targets contains all 6 required architectures', () => {
    assert.equal(CANONICAL_TARGETS.length, 6);
    const names = CANONICAL_TARGETS.map(t => `${t.os}/${t.arch}`);
    assert.deepEqual(names.sort(), [
      'darwin/amd64',
      'darwin/arm64',
      'linux/amd64',
      'linux/arm64',
      'windows/amd64',
      'windows/arm64'
    ]);
  });

  test('parseArgs parses all qualification flags', () => {
    const flags = parseArgs([
      '--isolated',
      '--require-go', '1.26.1',
      '--require-all-targets',
      '--record', 'scripts/qualification-inputs.json',
      '--mock-builds'
    ]);
    assert.equal(flags.isolated, true);
    assert.equal(flags.requireGo, '1.26.1');
    assert.equal(flags.requireAllTargets, true);
    assert.equal(flags.record, 'scripts/qualification-inputs.json');
    assert.equal(flags.mockBuilds, true);
  });

  test('checkGoVersion validates compatible 1.26 toolchain and rejects mismatched major/minor', () => {
    const res = checkGoVersion('1.26.1');
    assert.equal(res.compatible, true);
    assert.throws(() => checkGoVersion('1.25.0'), /Incompatible Go version/);
  });

  test('validateRecord validates authentic record file', () => {
    const data = validateRecord('scripts/qualification-inputs.json');
    assert.equal(data.tools.go.version, '1.26.1');
    assert.equal(data.tools.goreleaser.version, '2');
  });

  test('qualifyTargets in mock mode reports CGO=0 pass across all targets', async () => {
    const results = await qualifyTargets(CANONICAL_TARGETS, { isolated: true, mockBuilds: true });
    assert.equal(results.length, 6);
    for (const r of results) {
      assert.equal(r.build, 'pass');
      assert.equal(r.cgo_enabled, 0);
      assert.ok(['verified_local', 'deferred_ci_runner'].includes(r.runtime));
    }
  });

  test('runTargetQualification returns pass with verified local and deferred CI targets', async () => {
    const res = await runTargetQualification([
      '--isolated',
      '--require-go', '1.26.1',
      '--require-all-targets',
      '--record', 'scripts/qualification-inputs.json',
      '--mock-builds'
    ]);
    assert.equal(res.status, 'pass');
    assert.equal(res.targets.length, 6);
    assert.ok(res.verified_local >= 0);
    assert.ok(res.deferred_ci >= 0);
  });
});
