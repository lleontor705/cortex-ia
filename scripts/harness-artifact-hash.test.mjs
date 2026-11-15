import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('reviewer prompt and shared conventions make cortex_relate conditional', () => {
  const reviewerText = fs.readFileSync('internal/assets/agents/reviewer.md', 'utf8');
  const conventionText = fs.readFileSync('internal/assets/skills/_shared/cortex-convention.md', 'utf8');

  // Relate should be conditional, not mandatory on every outcome
  assert.doesNotMatch(conventionText, /always call `?cortex_relate`? to connect/i);
  assert.match(conventionText, /call `?cortex_relate`? to connect it when a meaningful relationship exists/i);
  assert.match(reviewerText, /Link via `?cortex_relate`? when a meaningful relationship exists/i);

});
