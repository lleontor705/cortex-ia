import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { POLICY_DOCUMENTS } from './check-test-policy.mjs';

const cli = fileURLToPath(new URL('./check-test-policy.mjs', import.meta.url));
const compliantDoc = `Permitted critical regressions: authority, transport, recovery, updater.
Preserved coverage: tui install-copy. Modular bounds: 250 and 300.
Temporary home directories, synthetic inputs, never weaken contract or skip invalid, ephemeral.`;

describe('check-test-policy CLI subprocess', () => {
  it('reports truthful structural-only output on synthetic root', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'cli-pol-'));
    try {
      for (const p of POLICY_DOCUMENTS) {
        const f = path.join(tmp, p);
        fs.mkdirSync(path.dirname(f), { recursive: true });
        fs.writeFileSync(f, compliantDoc, 'utf8');
      }
      const out = execFileSync(process.execPath, [cli, tmp], { encoding: 'utf8' });
      assert.match(out, /structural-only/i);
      assert.match(out, /PASS/);
      assert.match(out, /runtime isolation.*not-verified/i);
      assert.match(out, /execution.*not-verified/i);
      assert.match(out, /semantic integrity.*not-verified/i);
      assert.doesNotMatch(out, /enforced/i);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  it('fails subprocess on empty synthetic root', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'cli-fail-'));
    try {
      assert.throws(
        () => execFileSync(process.execPath, [cli, tmp], { encoding: 'utf8', stdio: 'pipe' }),
        (err) => err.status !== 0 && /structural-only/i.test(err.stderr || err.stdout)
      );
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });
});
