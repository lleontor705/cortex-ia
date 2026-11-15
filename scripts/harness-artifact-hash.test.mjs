import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const workDir = path.resolve('/isolated/work');

async function harness() {
  const files = new Map();
  const mod = await loadPluginFile('internal/assets/plugins/cortex-work.ts', {
    env: { HOME: home, USERPROFILE: home },
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    fs: {
      existsSync: p => p === executable || files.has(p),
      readFileSync: p => files.get(p) || '',
      mkdirSync: () => {},
      writeFileSync: (p, c) => files.set(p, c),
      renameSync: (from, to) => {
        files.set(to, files.get(from));
        files.delete(from);
      },
      unlinkSync: p => files.delete(p),
    },
    childProcess: {
      execFileSync: () => '{}'
    },
  });
  const plugin = await mod.instantiate({ client: {} });
  return { tools: plugin.tool, files };
}

test('cortex_ia_openspec_write returns exact committed byte length and sha256', async () => {
  const h = await harness();
  const context = { agent: 'planner', sessionID: 'session-planner', directory: workDir };
  const content = '# Proposal\n\nExact UTF-8 test with emojis 🚀 and accented characters áéíóú.\n';
  const expectedBytes = Buffer.byteLength(content, 'utf-8');
  const expectedHash = createHash('sha256').update(Buffer.from(content, 'utf-8')).digest('hex');

  const resultStr = await h.tools.cortex_ia_openspec_write.execute({
    relative_path: 'openspec/changes/test-change/proposal.md',
    content
  }, context);

  const result = JSON.parse(resultStr);
  assert.equal(result.written, 'openspec/changes/test-change/proposal.md');
  assert.equal(result.bytes, expectedBytes);
  assert.equal(result.sha256, expectedHash);

  // Compare with cortex_ia_content_hash to prove exact equivalence
  const hashResultStr = await h.tools.cortex_ia_content_hash.execute({ content });
  const hashResult = JSON.parse(hashResultStr);
  assert.equal(hashResult.byte_length, expectedBytes);
  assert.equal(hashResult.sha256, expectedHash);
});

test('cortex_ia_openspec_write rejects invalid Unicode or path escaping', async () => {
  const h = await harness();
  const context = { agent: 'planner', sessionID: 'session-planner', directory: workDir };

  // Invalid Unicode surrogate pair
  const badUnicode = '\uD800';
  await assert.rejects(
    h.tools.cortex_ia_openspec_write.execute({
      relative_path: 'openspec/changes/test-change/proposal.md',
      content: badUnicode
    }, context),
    /well-formed Unicode/
  );

  // Escaping path
  await assert.rejects(
    h.tools.cortex_ia_openspec_write.execute({
      relative_path: 'openspec/changes/../../escape.md',
      content: 'test'
    }, context),
    /OpenSpec writes must target a Markdown file under openspec\/changes\//
  );
});

test('reviewer prompt and shared conventions omit unconditional relate and mandatory duplicate hashes', () => {
  const reviewerText = fs.readFileSync('internal/assets/agents/reviewer.md', 'utf8');
  const conventionText = fs.readFileSync('internal/assets/skills/_shared/cortex-convention.md', 'utf8');
  const protocolText = fs.readFileSync('internal/assets/skills/_shared/cortex-work-protocol.md', 'utf8');

  // Relate should be conditional, not mandatory on every outcome
  assert.doesNotMatch(conventionText, /always call `?cortex_relate`? to connect/i);
  assert.match(conventionText, /call `?cortex_relate`? to connect it when a meaningful relationship exists/i);
  assert.match(reviewerText, /Link via `?cortex_relate`? when a meaningful relationship exists/i);

  // Reviewer Phase 1 allows direct verification without mandatory manual hash call
  assert.match(reviewerText, /verify the SHA-256 digest directly from the verified snapshot retrieval/i);

  // Protocol acknowledges byte-owning operations expose digests
  assert.match(protocolText, /byte-owning artifact operations and verified snapshot reads expose exact UTF-8 SHA-256 digests/i);
});
