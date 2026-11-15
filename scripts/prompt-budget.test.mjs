import test from 'node:test';
import assert from 'node:assert/strict';
import { stripFrontmatter, countImperatives, estimateTokens, evaluate } from './prompt-budget.mjs';

test('stripFrontmatter removes the leading YAML block and keeps the body', () => {
  const doc = '---\nname: planner\ndescription: "x --- y"\n---\n# Body\nText\n';
  assert.equal(stripFrontmatter(doc), '# Body\nText\n');
});

test('stripFrontmatter leaves text without frontmatter untouched', () => {
  const doc = '# Heading\n---\nnot frontmatter\n';
  assert.equal(stripFrontmatter(doc), doc);
  assert.equal(stripFrontmatter('---\nunclosed: true\n'), '---\nunclosed: true\n');
});

test('countImperatives counts only capitalised whole-word imperatives', () => {
  assert.equal(countImperatives('You MUST stop. NEVER edit. ALWAYS verify.'), 3);
  assert.equal(countImperatives('you must never, always or critical'), 0);
  assert.equal(countImperatives('MUSTARD MUSTN\'T CRITICALITY'), 0);
  assert.equal(countImperatives('STRICTLY, MANDATORY, IMMEDIATELY, CRITICAL.'), 4);
});

test('estimateTokens uses characters divided by four, rounded up', () => {
  assert.equal(estimateTokens('abcd'), 1);
  assert.equal(estimateTokens('abcde'), 2);
  assert.equal(estimateTokens(''), 0);
});

test('evaluate flags a skill over the hard max unless allowlisted', () => {
  const big = { body: 1200, imperatives: 0 };
  const metrics = {
    agentsMd: { tokens: 1, imperatives: 0 },
    roles: {},
    shared: {},
    skills: { planner: big, 'opencode2-knowledge': big },
  };
  const findings = evaluate(metrics, null);
  assert.equal(findings.length, 1);
  assert.match(findings[0], /skill planner/);
});
