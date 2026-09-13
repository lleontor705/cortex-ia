import test, { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  POLICY_DOCUMENTS,
  PERMITTED_PERSISTENT_CATEGORIES,
  validateTestCandidate,
  verifyPolicyDocuments,
} from './check-test-policy.mjs';

describe('validateTestCandidate', () => {
  it('permits authorized critical regression test with temporary home and modular size', () => {
    const candidate = {
      category: 'authority',
      isNewFile: true,
      lineCount: 150,
      usesTempHome: true,
      touchesRealState: false,
      testCases: ['TestWorkClaim', 'TestWorkLease'],
      executedCases: ['TestWorkClaim', 'TestWorkLease'],
      isEphemeral: false,
    };
    const result = validateTestCandidate(candidate);
    assert.equal(result.valid, true);
    assert.equal(result.errors.length, 0);
  });

  it('permits other critical regression categories and existing coverage', () => {
    for (const cat of ['transport', 'recovery', 'updater', 'tui', 'install-copy']) {
      const result = validateTestCandidate({
        category: cat,
        isNewFile: true,
        lineCount: 180,
        usesTempHome: true,
        touchesRealState: false,
        testCases: ['TestCase1'],
        executedCases: ['TestCase1'],
        isEphemeral: false,
      });
      assert.equal(result.valid, true, `Category ${cat} should be permitted`);
    }
  });

  it('permits ephemeral smoke for deeper unrelated transactional exploration', () => {
    const candidate = {
      category: 'unrelated-transactional',
      isNewFile: true,
      lineCount: 100,
      usesTempHome: true,
      touchesRealState: false,
      testCases: ['TestSmoke'],
      executedCases: ['TestSmoke'],
      isEphemeral: true,
    };
    const result = validateTestCandidate(candidate);
    assert.equal(result.valid, true);
  });

  it('rejects real-home access and unsafe developer state', () => {
    const resFlag = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      lineCount: 100,
      usesTempHome: false,
      touchesRealState: true,
      testCases: ['TestCase'],
      executedCases: ['TestCase'],
    });
    assert.equal(resFlag.valid, false);
    assert.match(resFlag.errors.join('; '), /real-home access/i);

    const resContent = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      usesTempHome: true,
      hasTempHomeIsolation: true,
      content: 'const cfg = path.join("~/.cortex-ia", "config.json");',
      testCases: ['TestCase'],
      executedCases: ['TestCase'],
    });
    assert.equal(resContent.valid, false);
    assert.match(resContent.errors.join('; '), /real-home access/i);
  });

  it('rejects missing cases and incomplete case execution', () => {
    const resEmpty = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      lineCount: 50,
      usesTempHome: true,
      testCases: [],
      executedCases: [],
    });
    assert.equal(resEmpty.valid, false);
    assert.match(resEmpty.errors.join('; '), /missing cases/i);

    const resMissing = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      lineCount: 50,
      usesTempHome: true,
      testCases: ['CaseA', 'CaseB'],
      executedCases: ['CaseA'],
    });
    assert.equal(resMissing.valid, false);
    assert.match(resMissing.errors.join('; '), /CaseB/);
  });

  it('rejects oversized new test files (>250 lines)', () => {
    const result = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      lineCount: 260,
      usesTempHome: true,
      testCases: ['Case1'],
      executedCases: ['Case1'],
    });
    assert.equal(result.valid, false);
    assert.match(result.errors.join('; '), /oversized new test file.*260/i);
  });

  it('rejects appends to oversized test suites (>300 lines)', () => {
    const result = validateTestCandidate({
      category: 'authority',
      isNewFile: false,
      existingFileLineCount: 310,
      usesTempHome: true,
      testCases: ['Case1'],
      executedCases: ['Case1'],
    });
    assert.equal(result.valid, false);
    assert.match(result.errors.join('; '), /oversized suites.*310/i);
  });

  it('rejects silent invalid-record skipping and contract weakening', () => {
    const resFlag = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      lineCount: 50,
      usesTempHome: true,
      testCases: ['Case1'],
      executedCases: ['Case1'],
      silentlySkipsInvalidRecords: true,
    });
    assert.equal(resFlag.valid, false);
    assert.match(resFlag.errors.join('; '), /silent invalid-record skipping/i);

    const resContent = validateTestCandidate({
      category: 'authority',
      isNewFile: true,
      usesTempHome: true,
      testCases: ['Case1'],
      executedCases: ['Case1'],
      content: 'for (const r of records) { if (!r.ok) { continue; // skip invalid } }',
    });
    assert.equal(resContent.valid, false);
    assert.match(resContent.errors.join('; '), /silent invalid-record skipping/i);
  });

  it('rejects unauthorized persistent test categories', () => {
    const result = validateTestCandidate({
      category: 'arbitrary-feature',
      isNewFile: true,
      lineCount: 80,
      usesTempHome: true,
      testCases: ['Case1'],
      executedCases: ['Case1'],
      isEphemeral: false,
    });
    assert.equal(result.valid, false);
    assert.match(result.errors.join('; '), /unauthorized persistent test/i);
  });
});

describe('verifyPolicyDocuments', () => {
  it('validates compliant documents across a synthetic root', () => {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'policy-test-'));
    try {
      const compliantContent = `
# Policy
- Permitted critical regressions: authority, transport, recovery, updater verification.
- Preserved coverage: TUI and install-copy.
- Modular bounds: dedicated new test files <= 250 lines; suite append limit <= 300 lines.
- Temporary home directories and synthetic inputs without real user state.
- Contract integrity: never weaken contracts or silently skip invalid records.
- Deeper unrelated transactional exploration remains ephemeral.
`;
      for (const relPath of POLICY_DOCUMENTS) {
        const full = path.join(tempDir, relPath);
        fs.mkdirSync(path.dirname(full), { recursive: true });
        fs.writeFileSync(full, compliantContent, 'utf8');
      }
      const result = verifyPolicyDocuments(tempDir);
      assert.equal(result.valid, true);
      assert.equal(result.errors.length, 0);
      assert.equal(result.documentsChecked.length, POLICY_DOCUMENTS.length);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it('rejects when a required policy document is missing', () => {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'policy-missing-'));
    try {
      const result = verifyPolicyDocuments(tempDir);
      assert.equal(result.valid, false);
      assert.equal(result.errors.length, POLICY_DOCUMENTS.length);
      assert.match(result.errors[0], /Missing required policy document/i);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it('rejects when a policy document lacks required tenets', () => {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'policy-bad-'));
    try {
      for (const relPath of POLICY_DOCUMENTS) {
        const full = path.join(tempDir, relPath);
        fs.mkdirSync(path.dirname(full), { recursive: true });
        // Lacks modular bounds (250/300) and contract integrity
        fs.writeFileSync(full, '# Stale policy\nNo tests allowed except TUI.\n', 'utf8');
      }
      const result = verifyPolicyDocuments(tempDir);
      assert.equal(result.valid, false);
      assert.ok(result.errors.length > 0);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });
});
