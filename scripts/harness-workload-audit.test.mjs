import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { loadPluginFile } from './harness-plugin-loader.mjs';

for (const key of Object.keys(process.env)) {
  if (key.startsWith('GIT_')) delete process.env[key];
}

const PLUGIN = 'internal/assets/plugins/cortex-permission-fence.ts';

function makeRepo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cortex-audit-'));
  const git = (...args) => execFileSync('git', args, { cwd: dir, stdio: 'ignore' });
  git('init', '-q');
  git('config', 'user.email', 'harness@example.invalid');
  git('config', 'user.name', 'harness');
  fs.mkdirSync(path.join(dir, 'src'));
  fs.writeFileSync(path.join(dir, 'src', 'main.go'), 'package main\n');
  git('add', '.');
  git('commit', '-q', '-m', 'base');
  fs.writeFileSync(path.join(dir, 'src', 'main.go'), 'package main\n\nfunc main() {}\n');
  return dir;
}

test('workload audit rejects an option-shaped baseRef without touching the named file', async () => {
  const dir = makeRepo();
  const target = path.join(dir, 'src', 'main.go');
  const before = fs.readFileSync(target, 'utf8');
  const mod = await loadPluginFile(PLUGIN, { allowChildProcess: true });
  assert.throws(
    () => mod.exports.auditWorkload(dir, 'flexible', '--output=src/main.go'),
    /baseRef must be a git revision, not an option/,
  );
  assert.equal(fs.readFileSync(target, 'utf8'), before);
});

test('workload audit diffs a normal ref in a temp repo', async () => {
  const dir = makeRepo();
  const mod = await loadPluginFile(PLUGIN, { allowChildProcess: true });
  const result = mod.exports.auditWorkload(dir, 'flexible', 'HEAD');
  assert.equal(result.workload_status, 'PASS');
  assert.equal(result.files_changed, 1);
  assert.equal(result.files[0].path, 'src/main.go');
  assert.equal(result.logic_lines, 2);
});

test('workload audit fails closed on a failed git call and an unknown policy', async () => {
  const notRepo = fs.mkdtempSync(path.join(os.tmpdir(), 'cortex-audit-norepo-'));
  const mod = await loadPluginFile(PLUGIN, { allowChildProcess: true });
  assert.throws(() => mod.exports.auditWorkload(notRepo, 'flexible', 'HEAD'), /git diff failed/);
  assert.throws(() => mod.exports.auditWorkload(makeRepo(), 'lenient', 'HEAD'), /policy must be strict, flexible or unbounded/);
});
