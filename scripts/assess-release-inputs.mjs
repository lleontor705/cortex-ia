import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';

export function detectRealSecrets(env = process.env, extraInputs = []) {
  const secretPatterns = [
    /gh[pousr]_[A-Za-z0-9_]{16,}|github_pat_[A-Za-z0-9_]{22,}/,
    /AKIA[0-9A-Z]{16}/,
    /eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}/,
    /sk_live_[0-9a-zA-Z]{24,}/,
  ];
  const sensitiveVarNames = [
    'GITHUB_TOKEN', 'GH_TOKEN', 'RELEASE_KEY', 'SIGNING_KEY',
    'AWS_SECRET_ACCESS_KEY', 'PROD_SECRET', 'DEPLOY_KEY'
  ];

  const violations = [];
  for (const name of sensitiveVarNames) {
    if (env[name] && typeof env[name] === 'string' && env[name].trim() !== '') {
      violations.push({ source: `env.${name}`, reason: 'Sensitive environment variable populated' });
    }
  }

  for (const input of extraInputs) {
    if (typeof input === 'string') {
      for (const pat of secretPatterns) {
        if (pat.test(input.trim())) {
          violations.push({ source: 'input', reason: 'Matches real secret pattern' });
        }
      }
    }
  }

  return violations;
}

export function classifyLinkerInput(flag) {
  const match = /^-X\s+([^=]+)=(.*)$/.exec(flag);
  if (!match) {
    return { flag, category: 'other', authority: false };
  }
  const [, symbol, value] = match;
  const isPublicId = /^(?:main\.)?(?:version|gitcommit|commit|builddate|date|tag|build)$/i.test(symbol.trim());
  const isSensitive = /key|secret|token|credential|cert/i.test(symbol.trim());

  return {
    symbol,
    value: isSensitive ? '[REDACTED-POTENTIAL-SECRET]' : value,
    category: isPublicId ? 'public_identifier' : isSensitive ? 'authority_material' : 'metadata',
    authority: isSensitive,
  };
}

export function assessReleaseInputs(options = {}) {
  const {
    env = process.env,
    syntheticOnly = true,
    isolatedOutput = true,
    outputPath = null,
    linkerFlags = [
      '-X main.version=v1.0.0-synthetic',
      '-X main.commit=0000000000000000000000000000000000000000',
      '-X main.buildDate=1970-01-01T00:00:00Z',
    ],
  } = options;

  if (syntheticOnly) {
    const violations = detectRealSecrets(env, linkerFlags);
    if (violations.length > 0) {
      throw new Error(`SECRET_LEAK_PREVENTION: real secrets detected in environment or inputs: ${JSON.stringify(violations)}`);
    }
  }

  const classified = linkerFlags.map(classifyLinkerInput);
  const publicIds = classified.filter(c => c.category === 'public_identifier');
  const authorityCandidates = classified.filter(c => c.authority);

  const report = {
    assessment_version: 1,
    timestamp: new Date().toISOString(),
    mode: 'synthetic_assessment',
    network_called: false,
    environment_scrubbed: true,
    privilege_status: 'unverified',
    summary: 'Distinguishes visible public identifiers from authority-bearing material without asserting compromise or harmlessness.',
    classified_inputs: classified,
    counts: {
      public_identifiers: publicIds.length,
      authority_material: authorityCandidates.length,
      total_flags: linkerFlags.length,
    },
  };

  let targetPath = outputPath;
  if (!targetPath && isolatedOutput) {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cortex-assess-'));
    targetPath = path.join(tempDir, 'release-inputs-assessment.json');
  }

  if (targetPath) {
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    fs.writeFileSync(targetPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
    report.outputPath = targetPath;
  }

  return report;
}

export async function runCLI(argv = process.argv.slice(2)) {
  const args = { syntheticOnly: false, isolatedOutput: false, outputPath: null };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--synthetic-only') args.syntheticOnly = true;
    else if (argv[i] === '--isolated-output') args.isolatedOutput = true;
    else if (argv[i] === '--output') args.outputPath = argv[++i];
  }

  const result = assessReleaseInputs({
    syntheticOnly: args.syntheticOnly,
    isolatedOutput: args.isolatedOutput,
    outputPath: args.outputPath,
  });

  console.log(`[ASSESS-RELEASE-INPUTS] Successfully assessed ${result.counts.total_flags} inputs. Privilege status: ${result.privilege_status}`);
  if (result.outputPath) {
    console.log(`[ASSESS-RELEASE-INPUTS] Report saved to isolated path: ${result.outputPath}`);
  }
  return result;
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve('scripts/assess-release-inputs.mjs')) {
  runCLI().catch((err) => {
    console.error('ERROR:', err.message);
    process.exit(1);
  });
}
