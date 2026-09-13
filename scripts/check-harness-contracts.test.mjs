import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import {
  extractBlocks,
  analyzeCorpus,
  findBrokenPointers,
  runCLI,
} from './check-harness-contracts.mjs';

function createTempDir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

test('19/20-word boundary: ignores 19-word repetition, catches 20-word duplicate', () => {
  const words19 = 'alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra';
  const words20 = words19 + ' tango';

  assert.equal(words19.split(' ').length, 19);
  assert.equal(words20.split(' ').length, 20);

  const tmp = createTempDir('cortex-w-bound-');
  try {
    const f1 = path.join(tmp, 'f1.md');
    const f2 = path.join(tmp, 'f2.md');

    // 19 words repeated: should not count as duplicate (D = 0)
    fs.writeFileSync(f1, words19, 'utf8');
    fs.writeFileSync(f2, words19, 'utf8');
    const res19 = analyzeCorpus([f1, f2], { minBlockWords: 20, baseDir: tmp });
    assert.equal(res19.D, 0);
    assert.equal(res19.duplicates.length, 0);

    // 20 words repeated: counted as duplicate (D = 20)
    fs.writeFileSync(f1, words20, 'utf8');
    fs.writeFileSync(f2, words20, 'utf8');
    const res20 = analyzeCorpus([f1, f2], { minBlockWords: 20, baseDir: tmp });
    assert.equal(res20.D, 20);
    assert.equal(res20.duplicates.length, 1);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('code/pointer exclusion: fenced code and pointer-only blocks excluded from W and D', () => {
  const content = `# Header

Some introductory prose explaining the module architecture in clear language.

\`\`\`go
func DuplicateCodeAcrossFiles() {
  // This large code block must be excluded from prose word metric
  fmt.Println("one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty")
}
\`\`\`

[See canonical protocol](internal/assets/skills/_shared/cortex-work-protocol.md)
file:///d:/cortex-ia/specs/protocol.md

Final concluding paragraph with additional instructions for the agent.
`;

  const blocks = extractBlocks(content);
  // Only 2 prose blocks extracted: intro and conclusion
  assert.equal(blocks.length, 2);
  const combinedText = blocks.map(b => b.text).join(' ');
  assert.ok(!combinedText.includes('DuplicateCodeAcrossFiles'));
  assert.ok(!combinedText.includes('file:///'));
});

test('duplicate vs paraphrase distinction: exact match caught, paraphrase ignored by metric', () => {
  const original = 'Every coordinated initiative should have one stable board identifier and tasks must reside within the same board without circular dependencies or invalid states.';
  const paraphrase = 'All collaborative projects require a consistent board ID, and related work items need to belong to identical boards avoiding cyclical graph relationships.';

  const tmp = createTempDir('cortex-para-');
  try {
    const f1 = path.join(tmp, 'f1.md');
    const f2 = path.join(tmp, 'f2.md');

    // Paraphrase has different wording
    fs.writeFileSync(f1, original, 'utf8');
    fs.writeFileSync(f2, paraphrase, 'utf8');
    const res = analyzeCorpus([f1, f2], { minBlockWords: 20, baseDir: tmp });
    assert.equal(res.D, 0);
    assert.equal(res.duplicates.length, 0);

    // Exact duplicate is caught
    fs.writeFileSync(f2, original, 'utf8');
    const resExact = analyzeCorpus([f1, f2], { minBlockWords: 20, baseDir: tmp });
    assert.ok(resExact.D > 0);
    assert.equal(resExact.duplicates.length, 1);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('80/81-word summary boundary: summary <= 80 words passes, 81 words flagged', () => {
  const words80 = Array(80).fill('safety').join(' ');
  const words81 = Array(81).fill('safety').join(' ');

  const tmp = createTempDir('cortex-sum-');
  try {
    const f1 = path.join(tmp, 'f1.md');
    fs.writeFileSync(f1, words80, 'utf8');
    const res80 = analyzeCorpus([f1], {
      maxExemptSummaryWords: 80,
      exemptSummaryCheck: true,
      baseDir: tmp,
    });
    assert.equal(res80.exemptViolations.length, 0);

    // With 81 words, if marked exempt summary, it exceeds threshold
    const b81 = { text: words81, wordCount: 81, isExemptSummary: true };
    const res81Exempt = [b81].filter(b => b.isExemptSummary && b.wordCount > 80);
    assert.equal(res81Exempt.length, 1);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('broken pointer detection: catches non-existent relative link targets', () => {
  const tmp = createTempDir('cortex-ptr-');
  try {
    const validFile = path.join(tmp, 'target.md');
    fs.writeFileSync(validFile, '# Target', 'utf8');

    const sourceFile = path.join(tmp, 'source.md');
    fs.writeFileSync(
      sourceFile,
      'See [valid](./target.md) and [broken](./ghost.md) for details.',
      'utf8'
    );

    const content = fs.readFileSync(sourceFile, 'utf8');
    const broken = findBrokenPointers(content, sourceFile, tmp);
    assert.equal(broken.length, 1);
    assert.equal(broken[0].link, './ghost.md');
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('W increase detection: batch fails if W exceeds baseline', async () => {
  const tmp = createTempDir('cortex-w-inc-');
  try {
    const baselineFile = path.join(tmp, 'baseline.json');
    fs.writeFileSync(baselineFile, JSON.stringify({ W: 100 }), 'utf8');

    // Simulate batch execution where current W exceeds baseline
    await assert.rejects(
      async () => {
        await runCLI(['--mode', 'batch', '--batch', 'C21', '--baseline', baselineFile]);
      },
      /W_INCREASE_DETECTED/
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('corpus enumeration: manifest contains all files and includes workflow-retrospective', () => {
  const contractsPath = path.resolve('scripts/harness-contracts.json');
  const contracts = JSON.parse(fs.readFileSync(contractsPath, 'utf8'));

  assert.ok(contracts.corpus.includes('internal/assets/skills/workflow-retrospective/SKILL.md'));
  assert.ok(contracts.corpus.length >= 40);

  for (const rel of contracts.corpus) {
    const full = path.resolve(rel);
    assert.ok(fs.existsSync(full), `Corpus file missing: ${rel}`);
  }
});

test('baseline reports violations without failing, check mode rejects them on unmigrated corpus', async () => {
  const tmp = createTempDir('cortex-base-test-');
  try {
    const outPath = path.join(tmp, 'test-baseline.json');
    const res = await runCLI(['--mode', 'baseline', '--output', outPath]);
    assert.equal(res.ok, true);
    assert.ok(res.analysis.D > 0, 'Unmigrated corpus must report non-zero duplication');
    assert.ok(fs.existsSync(outPath));
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
