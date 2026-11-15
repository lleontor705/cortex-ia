import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { parseArgs, validateCommitSHA, validateInputsRecord, runCheckQualification } from './check-qualification.mjs';

describe('CI Inputs Qualification Suite', () => {
  test('parseArgs parses flags correctly', () => {
    const flags = parseArgs([
      '--mode', 'inputs',
      '--record', 'custom/inputs.json',
      '--scope', 'ci',
      '--require-authentic-provenance'
    ]);
    assert.equal(flags.mode, 'inputs');
    assert.equal(flags.record, 'custom/inputs.json');
    assert.equal(flags.scope, 'ci');
    assert.equal(flags.requireAuthenticProvenance, true);
  });

  test('validateCommitSHA enforces exact 40-hex lowercase format', () => {
    // Valid 40-hex
    assert.doesNotThrow(() => validateCommitSHA('11d5960a326750d5838078e36cf38b85af677262'));
    assert.doesNotThrow(() => validateCommitSHA('fe9f00320a3a9618d6f8bb841eae891861ec5774'));

    // 39 chars
    assert.throws(() => validateCommitSHA('11d5960a326750d5838078e36cf38b85af67726'), /Commit SHA must be exactly 40 hex characters/);
    // 41 chars
    assert.throws(() => validateCommitSHA('11d5960a326750d5838078e36cf38b85af6772621'), /Commit SHA must be exactly 40 hex characters/);
    // Non-hex
    assert.throws(() => validateCommitSHA('11d5960a326750d5838078e36cf38b85af67726z'), /Commit SHA must be exactly 40 hex characters/);
    // Uppercase
    assert.throws(() => validateCommitSHA('11D5960A326750D5838078E36CF38B85AF677262'), /Commit SHA must be exactly 40 hex characters/);
  });

  test('validateInputsRecord validates authentic qualification record', () => {
    const data = validateInputsRecord('scripts/qualification-inputs.json', true);
    assert.equal(data.tools.go.version, '1.26.1');
    assert.equal(data.tools.goreleaser.version, '2');
    assert.ok(data.actions['actions/checkout@v4']);
    assert.ok(data.reusable_workflows['lleontor705/ats-deploy-public/.github/workflows/quality-go.yml@main']);
  });

  test('validateInputsRecord rejects missing required action', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'q-test-'));
    try {
      const bad = {
        tools: { go: { version: '1.26.1' }, goreleaser: { version: '2' } },
        actions: {},
        reusable_workflows: {}
      };
      const p = path.join(tmp, 'rec.json');
      fs.writeFileSync(p, JSON.stringify(bad));
      assert.throws(() => validateInputsRecord(p, true), /Missing required action/);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('validateInputsRecord rejects wrong Go version', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'q-test-'));
    try {
      const bad = {
        tools: { go: { version: '1.25.0' }, goreleaser: { version: '2' } },
        actions: {},
        reusable_workflows: {}
      };
      const p = path.join(tmp, 'rec.json');
      fs.writeFileSync(p, JSON.stringify(bad));
      assert.throws(() => validateInputsRecord(p, true), /Expected Go version 1.26.1/);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('runCheckQualification succeeds in inputs and verify-runners modes', () => {
    const resInputs = runCheckQualification(['--mode', 'inputs', '--record', 'scripts/qualification-inputs.json', '--require-authentic-provenance']);
    assert.equal(resInputs.status, 'pass');

    const resRunners = runCheckQualification(['--mode', 'verify-runners', '--record', 'scripts/qualification-inputs.json']);
    assert.equal(resRunners.status, 'pass');
  });
});
