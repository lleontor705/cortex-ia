import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const POLICY_DOCUMENTS = [
  'AGENTS.md',
  'docs/sdd-workflow.md',
  'internal/assets/skills/_shared/codebase-design-contract.md',
  'internal/assets/skills/_shared/diagnosis-loop-contract.md',
  'internal/assets/skills/implement/SKILL.md',
];

export const PERMITTED_PERSISTENT_CATEGORIES = new Set([
  'tui',
  'install-copy',
  'authority',
  'transport',
  'recovery',
  'updater',
]);

const REQUIRED_POLICY_TENETS = [
  {
    name: 'critical-exception',
    pattern: /(?:critical|cr[ií]ticas).*?(?:authority|autoridad).*?(?:transport|transporte).*?(?:recovery|recuperaci[oó]n).*?(?:updater|actualizador|update)/is,
    description: 'Permits bounded critical regressions for authority, transport, recovery, and updater',
  },
  {
    name: 'preserved-coverage',
    pattern: /tui.*?(?:install|pipeline|copia)/is,
    description: 'Preserves existing TUI and simple install-copy coverage',
  },
  {
    name: 'modular-bounds',
    pattern: /250.*?300|300.*?250/s,
    description: 'Enforces dedicated new files <= 250 LOC and suite append limit <= 300 LOC',
  },
  {
    name: 'temp-homes',
    pattern: /temporar(?:y|ias?).*?(?:home|director)/i,
    description: 'Requires temporary homes and synthetic inputs without real user state',
  },
  {
    name: 'contract-integrity',
    pattern: /(?:weaken|debilitar).*?(?:contract|contratos)|silent.*?skip|skip.*?invalid/i,
    description: 'Forbids contract weakening or silently skipping invalid records',
  },
  {
    name: 'ephemeral-exploration',
    pattern: /ephemeral|ef[ií]meras?/i,
    description: 'Requires deeper unrelated transactional exploration to remain ephemeral',
  },
];

export function verifyPolicyDocuments(rootDir = process.cwd()) {
  const errors = [];
  const documentsChecked = [];

  for (const relPath of POLICY_DOCUMENTS) {
    const fullPath = path.resolve(rootDir, relPath);
    documentsChecked.push(relPath);

    if (!fs.existsSync(fullPath)) {
      errors.push(`Missing required policy document: ${relPath}`);
      continue;
    }

    const content = fs.readFileSync(fullPath, 'utf8');
    for (const tenet of REQUIRED_POLICY_TENETS) {
      if (!tenet.pattern.test(content)) {
        errors.push(`Document ${relPath} missing required tenet: ${tenet.name} (${tenet.description})`);
      }
    }
  }

  return {
    valid: errors.length === 0,
    errors,
    documentsChecked,
  };
}

export function validateTestCandidate(candidate) {
  const errors = [];
  if (!candidate || typeof candidate !== 'object') {
    return { valid: false, errors: ['Invalid candidate object'] };
  }

  // 1. Rejects real-home access / unsafe developer state
  if (candidate.touchesRealState || candidate.accessesRealHome || candidate.usesTempHome !== true) {
    errors.push('Rejects real-home access: tests must use temporary homes and synthetic inputs, never touching real user or developer state');
  }
  if (typeof candidate.content === 'string') {
    const unsafePatterns = [
      /~[/\\]\.cortex-ia/,
      /os\.UserHomeDir\(\)/,
      /process\.env\.HOME(?!\s*\|\|\s*temp)/,
      /process\.env\.USERPROFILE/,
      /\$env:USERPROFILE/,
      /\bHKEY_CURRENT_USER\b/i,
    ];
    for (const pat of unsafePatterns) {
      if (pat.test(candidate.content)) {
        errors.push(`Rejects real-home access: pattern ${pat} detected without isolated temp boundary`);
        break;
      }
    }
  }

  // 2. Rejects missing cases
  const testCases = Array.isArray(candidate.testCases) ? candidate.testCases : [];
  const executedCases = Array.isArray(candidate.executedCases) ? candidate.executedCases : [];
  if (testCases.length === 0) {
    errors.push('Rejects missing cases: test candidate must define and execute named test cases');
  } else {
    const executedSet = new Set(executedCases);
    const missing = testCases.filter((tc) => !executedSet.has(tc));
    if (missing.length > 0) {
      errors.push(`Rejects missing cases: required cases not executed: ${missing.join(', ')}`);
    }
  }

  // 3. Rejects oversized new test file (>250 lines)
  const lineCount = candidate.lineCount ?? (typeof candidate.content === 'string' ? candidate.content.split(/\r?\n/).length : 0);
  if (candidate.isNewFile && lineCount > 250) {
    errors.push(`Rejects oversized new test file: ${lineCount} lines exceeds modular limit of 250 lines`);
  }

  // 4. Rejects appends to oversized suites (>300 lines existing file)
  const existingFileLineCount = candidate.existingFileLineCount ?? 0;
  if (!candidate.isNewFile && existingFileLineCount > 300) {
    errors.push(`Rejects appends to oversized suites: target test file has ${existingFileLineCount} lines, exceeding 300 LOC threshold`);
  }

  // 5. Rejects silent invalid-record skipping / contract weakening
  if (candidate.silentlySkipsInvalidRecords || candidate.weakensContract) {
    errors.push('Rejects silent invalid-record skipping: tests/implementations must never weaken contracts or silently skip invalid records');
  }
  if (typeof candidate.content === 'string') {
    const silentSkipPatterns = [
      /continue;\s*\/\/\s*skip\s+invalid/i,
      /\/\/\s*silently\s+ignore\s+invalid/i,
      /if\s*\(!record\.isValid\)\s*\{\s*continue;\s*\}/i,
      /if\s*err\s*!=\s*nil\s*\{\s*\/\/\s*skip\s+invalid\s+record\s*continue\s*\}/i,
    ];
    for (const pat of silentSkipPatterns) {
      if (pat.test(candidate.content)) {
        errors.push('Rejects silent invalid-record skipping: pattern indicates contract weakening by skipping invalid records');
        break;
      }
    }
  }

  // 6. Rejects unauthorized persistent test categories
  if (!candidate.isEphemeral) {
    const category = String(candidate.category || '').toLowerCase();
    if (!PERMITTED_PERSISTENT_CATEGORIES.has(category)) {
      errors.push(`Rejects unauthorized persistent test: category '${candidate.category}' is not in permitted persistent classes (TUI, install-copy, authority, transport, recovery, updater). Deeper exploration must remain ephemeral.`);
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const targetDir = process.argv[2] ? path.resolve(process.argv[2]) : process.cwd();
  const result = verifyPolicyDocuments(targetDir);
  if (!result.valid) {
    console.error('[check-test-policy] Policy verification FAILED (structural-only):');
    for (const err of result.errors) {
      console.error(`  - ${err}`);
    }
    process.exit(1);
  }
  console.log(`[check-test-policy] Policy verification PASS (structural-only): ${result.documentsChecked.length} documents agree.`);
  console.log('[check-test-policy] Permitted persistent categories: tui, install-copy, authority, transport, recovery, updater.');
  console.log('[check-test-policy] Modular bounds: new test files <= 250 LOC, suite append threshold <= 300 LOC.');
  console.log('[check-test-policy] Invariants stated: temporary homes, synthetic inputs, contract integrity.');
  console.log('[check-test-policy] Disclaimer: runtime isolation not-verified.');
  console.log('[check-test-policy] Disclaimer: execution not-verified.');
  console.log('[check-test-policy] Disclaimer: semantic integrity not-verified.');
}
