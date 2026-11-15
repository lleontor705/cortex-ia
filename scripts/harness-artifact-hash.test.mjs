import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

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
