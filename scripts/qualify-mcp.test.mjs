import { test, describe } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { parseArgs, checkNodeEngine, qualifyLock, qualifyMCP } from './qualify-mcp.mjs';

describe('MCP Qualification Suite', () => {
  test('parseArgs parses all expected flags', () => {
    const args = [
      '--locked',
      '--isolate',
      '--timeout-ms', '8000',
      '--max-output-bytes', '524288',
      '--max-requests', '2',
      '--fixture-root', 'custom/root',
      '--require-live-schema-evidence',
      '--offline'
    ];
    const flags = parseArgs(args);
    assert.equal(flags.locked, true);
    assert.equal(flags.isolate, true);
    assert.equal(flags.timeoutMs, 8000);
    assert.equal(flags.maxOutputBytes, 524288);
    assert.equal(flags.maxRequests, 2);
    assert.equal(flags.fixtureRoot, 'custom/root');
    assert.equal(flags.requireLiveSchemaEvidence, true);
    assert.equal(flags.offline, true);
  });

  test('checkNodeEngine enforces >=20.18.1', () => {
    assert.equal(checkNodeEngine('v20.18.1', '>=20.18.1'), true);
    assert.equal(checkNodeEngine('v24.20.0', '>=20.18.1'), true);
    assert.equal(checkNodeEngine('v20.18.0', '>=20.18.1'), false);
    assert.equal(checkNodeEngine('v20.17.9', '>=20.18.1'), false);
    assert.equal(checkNodeEngine('v18.20.0', '>=20.18.1'), false);
  });

  test('qualifyLock validates official context7 4.1.0 and lock integrity', () => {
    const res = qualifyLock('scripts/fixtures/mcp');
    assert.equal(res.pkg.dependencies['@upstash/context7-mcp'], '4.1.0');
    assert.equal(res.lockEntry.version, '4.1.0');
    assert.ok(res.lockEntry.integrity.startsWith('sha512-'));
    assert.ok(res.lockEntry.resolved.includes('registry.npmjs.org'));
  });

  test('qualifyLock rejects corrupted lockfile integrity', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mcp-test-'));
    try {
      fs.writeFileSync(path.join(tmp, 'package.json'), JSON.stringify({
        dependencies: { '@upstash/context7-mcp': '4.1.0' },
        engines: { node: '>=20.18.1' }
      }));
      fs.writeFileSync(path.join(tmp, 'package-lock.json'), JSON.stringify({
        packages: {
          'node_modules/@upstash/context7-mcp': {
            version: '4.1.0',
            integrity: 'sha512-corrupted-hash=='
          }
        }
      }));
      assert.throws(() => qualifyLock(tmp), /Lockfile integrity mismatch/);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('qualifyLock rejects wrong package version', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mcp-test-'));
    try {
      fs.writeFileSync(path.join(tmp, 'package.json'), JSON.stringify({
        dependencies: { '@upstash/context7-mcp': '4.0.7' },
        engines: { node: '>=20.18.1' }
      }));
      fs.writeFileSync(path.join(tmp, 'package-lock.json'), JSON.stringify({
        packages: {
          'node_modules/@upstash/context7-mcp': { version: '4.0.7', integrity: 'sha512-xxx' }
        }
      }));
      assert.throws(() => qualifyLock(tmp), /Expected @upstash\/context7-mcp@4\.1\.0/);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('qualifyMCP executes offline validation successfully', () => {
    const res = qualifyMCP(['--offline', '--fixture-root', 'scripts/fixtures/mcp']);
    assert.ok(res.capabilities.cortex);
    assert.ok(res.capabilities.context7);
    assert.equal(res.lockInfo.lockEntry.version, '4.1.0');
  });

  test('qualifyMCP live mode verifies actual Cortex MCP executable and schema', () => {
    const res = qualifyMCP([
      '--locked',
      '--isolate',
      '--timeout-ms', '10000',
      '--max-output-bytes', '1048576',
      '--max-requests', '2',
      '--fixture-root', 'scripts/fixtures/mcp',
      '--require-live-schema-evidence'
    ]);
    assert.ok(res.liveResult);
    assert.ok(res.liveResult.executableSha256);
    assert.ok(res.liveResult.toolsCount >= 30);
    assert.ok(Array.isArray(res.liveResult.optionalAbsent));
  });
});
