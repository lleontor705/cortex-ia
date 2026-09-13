import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';

export function normalizeBlock(text) {
  return text.replace(/\r\n/g, '\n').replace(/\s+/g, ' ').trim();
}

export function extractBlocks(content) {
  const withoutCode = content.replace(/```[\s\S]*?```/g, '');
  const rawBlocks = withoutCode.split(/\n\s*\n/);
  const blocks = [];

  for (const raw of rawBlocks) {
    const lines = raw.split('\n');
    const currentProse = [];
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      if (/^(?:[-*]\s+)?(?:\[[^\]]+\]\([^)]+\)|file:\/\/\S+)$/.test(trimmed)) continue;
      if (/^#{1,6}\s+[^#]+$/.test(trimmed) && trimmed.split(/\s+/).length < 6) continue;
      currentProse.push(trimmed);
    }
    if (currentProse.length > 0) {
      const text = normalizeBlock(currentProse.join(' '));
      const words = text.split(/\s+/).filter(Boolean);
      if (words.length > 0) {
        blocks.push({ text, words, wordCount: words.length });
      }
    }
  }
  return blocks;
}

export function findBrokenPointers(content, filePath, baseDir = process.cwd()) {
  const fileDir = path.dirname(path.resolve(baseDir, filePath));
  const linkRegex = /\[[^\]]+\]\(([^)]+)\)/g;
  const broken = [];
  let match;
  while ((match = linkRegex.exec(content)) !== null) {
    let target = match[1].split('#')[0].trim();
    if (!target || target.startsWith('http://') || target.startsWith('https://') || target.startsWith('mailto:')) continue;
    if (target.startsWith('file:///')) target = target.replace('file:///', '');
    const resolved = path.isAbsolute(target) ? target : path.resolve(fileDir, target);
    if (!fs.existsSync(resolved)) {
      broken.push({ link: match[1], file: filePath, resolved });
    }
  }
  return broken;
}

export function isExemptInvariant(text, wordCount, maxExempt = 80) {
  if (wordCount > maxExempt) return false;
  const lower = text.toLowerCase();
  const patterns = [
    /host\s+precedence|host\s+policy/i,
    /role\s+(?:authority|assignment)|allowed_files/i,
    /claim|lease|token/i,
    /reconciliation|delegation\s+admission/i,
    /secret|live\s+memory/i
  ];
  return patterns.some(p => p.test(lower));
}

export function analyzeCorpus(corpusFiles, options = {}) {
  const minWords = options.minBlockWords ?? 20;
  const maxExempt = options.maxExemptSummaryWords ?? 80;
  const baseDir = options.baseDir || process.cwd();
  let totalW = 0;
  const blockMap = new Map();
  const brokenPointers = [];
  const exemptViolations = [];

  for (const rel of corpusFiles) {
    const full = path.resolve(baseDir, rel);
    if (!fs.existsSync(full)) {
      brokenPointers.push({ link: rel, file: rel, error: 'corpus file missing' });
      continue;
    }
    const content = fs.readFileSync(full, 'utf8');
    const broken = findBrokenPointers(content, rel, baseDir);
    brokenPointers.push(...broken);

    const blocks = extractBlocks(content);
    for (const b of blocks) {
      totalW += b.wordCount;
      if (options.exemptSummaryCheck && b.isExemptSummary && b.wordCount > maxExempt) {
        exemptViolations.push({ file: rel, words: b.wordCount, text: b.text });
      }
      if (b.wordCount >= minWords) {
        const key = b.text.toLowerCase();
        const entry = blockMap.get(key) || { count: 0, length: b.wordCount, text: b.text, files: [] };
        entry.count++;
        entry.files.push(rel);
        blockMap.set(key, entry);
      }
    }
  }

  let totalD = 0;
  let exemptD = 0;
  const duplicates = [];
  for (const [, entry] of blockMap) {
    if (entry.count > 1) {
      const excess = (entry.count - 1) * entry.length;
      totalD += excess;
      if (isExemptInvariant(entry.text, entry.length, maxExempt)) {
        exemptD += excess;
      } else {
        duplicates.push(entry);
      }
    }
  }

  return { W: totalW, D: totalD, exemptD, duplicates, brokenPointers, exemptViolations };
}

export function loadBaseline(baselinePath) {
  if (!baselinePath || !fs.existsSync(baselinePath)) {
    throw new Error(`BASELINE_NOT_FOUND: baseline file ${baselinePath} does not exist`);
  }
  return JSON.parse(fs.readFileSync(baselinePath, 'utf8'));
}

export function resolveBaselinePath(cliPath, baseDir = process.cwd()) {
  if (cliPath && fs.existsSync(cliPath)) return path.resolve(cliPath);
  if (process.env.CORTEX_TEST_BASELINE && fs.existsSync(process.env.CORTEX_TEST_BASELINE)) {
    return path.resolve(process.env.CORTEX_TEST_BASELINE);
  }
  const defaultEvidence = path.resolve(baseDir, '.cortex-ia', 'harness-baseline.json');
  if (fs.existsSync(defaultEvidence)) return defaultEvidence;
  return null;
}

export async function runCLI(argv = process.argv.slice(2)) {
  const args = { mode: 'check', baseDir: process.cwd() };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--mode') args.mode = argv[++i];
    else if (argv[i] === '--create-isolated-baseline') args.createIsolatedBaseline = true;
    else if (argv[i] === '--baseline-from-evidence') args.baselineFromEvidence = true;
    else if (argv[i] === '--baseline') args.baseline = argv[++i];
    else if (argv[i] === '--batch') args.batch = argv[++i];
    else if (argv[i] === '--output') args.output = argv[++i];
  }

  const contractsPath = path.resolve(args.baseDir, 'scripts/harness-contracts.json');
  const contracts = JSON.parse(fs.readFileSync(contractsPath, 'utf8'));
  const analysis = analyzeCorpus(contracts.corpus, {
    minBlockWords: contracts.metric?.min_block_words ?? 20,
    maxExemptSummaryWords: contracts.metric?.max_exempt_summary_words ?? 80,
    baseDir: args.baseDir,
  });

  if (args.mode === 'baseline') {
    const outPath = args.output
      ? path.resolve(args.output)
      : path.resolve(args.baseDir, '.cortex-ia', 'harness-baseline.json');
    fs.mkdirSync(path.dirname(outPath), { recursive: true });
    const baselineData = {
      version: 1,
      timestamp: new Date().toISOString(),
      W: analysis.W,
      D: analysis.D,
      exemptD: analysis.exemptD,
      duplicates_count: analysis.duplicates.length,
      broken_pointers_count: analysis.brokenPointers.length,
    };
    fs.writeFileSync(outPath, JSON.stringify(baselineData, null, 2) + '\n', 'utf8');
    console.log(`[HARNESS-BASELINE] Baseline captured: W=${analysis.W}, D=${analysis.D}, ExemptD=${analysis.exemptD}, Violations=${analysis.duplicates.length}`);
    console.log(`BASELINE_PATH: ${outPath}`);
    return { ok: true, baselinePath: outPath, analysis };
  }

  const baselineFile = resolveBaselinePath(args.baseline, args.baseDir);
  const baseline = baselineFile ? loadBaseline(baselineFile) : null;

  if (args.mode === 'batch') {
    if (!args.batch) throw new Error('BATCH_REQUIRED: --batch <batchId> must be specified in batch mode');
    const batchDef = contracts.batches[args.batch];
    if (!batchDef) throw new Error(`UNKNOWN_BATCH: batch '${args.batch}' is not declared in manifest`);

    if (baseline && analysis.W > baseline.W) {
      throw new Error(`W_INCREASE_DETECTED: current corpus W (${analysis.W}) exceeds baseline W (${baseline.W})`);
    }

    const otherBatches = Object.keys(contracts.batches).filter(b => b !== args.batch);
    console.log(`[HARNESS-BATCH] Batch ${args.batch} (${batchDef.name}) passed checks. W=${analysis.W}`);
    console.log(`[HARNESS-BATCH] Note: unfinished other batches: ${otherBatches.join(', ')}`);
    return { ok: true, batch: args.batch, otherBatches };
  }

  // mode === 'check'
  if (baseline && analysis.W > baseline.W) {
    console.error(`FAIL: W increased from ${baseline.W} to ${analysis.W}`);
    process.exit(1);
  }
  if (analysis.brokenPointers.length > 0 || analysis.duplicates.length > 0 || analysis.exemptViolations.length > 0) {
    console.error(`FAIL: Violations detected in corpus: ${analysis.duplicates.length} duplicate blocks, ${analysis.brokenPointers.length} broken pointers`);
    process.exit(1);
  }

  console.log(`[HARNESS-CHECK] All corpus checks passed. W=${analysis.W}, D=${analysis.D}, ExemptD=${analysis.exemptD}`);
  return { ok: true };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve('scripts/check-harness-contracts.mjs')) {
  runCLI().catch((err) => {
    console.error('ERROR:', err.message);
    process.exit(1);
  });
}
