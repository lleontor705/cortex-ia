import { spawn } from 'node:child_process';
import readline from 'node:readline';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export function parseArgs(argv) {
  let requireOs = null;
  let command = null;
  let cmdArgs = [];

  const doubleDashIdx = argv.indexOf('--');
  const flags = doubleDashIdx >= 0 ? argv.slice(0, doubleDashIdx) : argv;
  const targetCmd = doubleDashIdx >= 0 ? argv.slice(doubleDashIdx + 1) : [];

  for (let i = 0; i < flags.length; i++) {
    if (flags[i] === '--require-os') {
      requireOs = flags[++i];
    }
  }

  if (targetCmd.length > 0) {
    command = targetCmd[0];
    cmdArgs = targetCmd.slice(1);
  }

  return { requireOs, command, cmdArgs };
}

export async function runWindowsBoundary(options = {}) {
  const { requireOs, command, cmdArgs = [], env = { ...process.env } } = options;

  if (requireOs === 'windows' && process.platform !== 'win32') {
    throw new Error(`Runner requires OS windows, current platform is ${process.platform}`);
  }

  if (!command) {
    throw new Error('Child command is required after --');
  }

  // Refuse pre-existing fixture paths and track owned ephemeral creation
  const fixtureDir = path.join(os.tmpdir(), `cortex-w1-smoke-${Date.now()}`);
  if (fs.existsSync(fixtureDir)) {
    throw new Error(`Pre-existing fixture path refused: ${fixtureDir}`);
  }
  fs.mkdirSync(fixtureDir, { recursive: true });
  env.CORTEX_WINDOWS_BOUNDARY_FIXTURE = fixtureDir;

  const expectedPackages = ['internal/homelock', 'internal/delegation'];
  const packageResults = new Map();
  for (const pkg of expectedPackages) {
    packageResults.set(pkg, { executed: false, passed: false, failed: false });
  }

  try {
    await new Promise((resolve, reject) => {
      const child = spawn(command, cmdArgs, {
        env,
        stdio: ['ignore', 'pipe', 'pipe'],
      });

      let stderr = '';
      const rl = readline.createInterface({ input: child.stdout });

      rl.on('line', (line) => {
        const trimmed = line.trim();
        if (!trimmed) return;
        try {
          const ev = JSON.parse(trimmed);
          if (ev.Test === 'TestWindowsBoundarySmoke' && ev.Package) {
            const pkg = ev.Package;
            for (const expected of expectedPackages) {
              if (pkg.endsWith(expected)) {
                const res = packageResults.get(expected);
                if (ev.Action === 'run') res.executed = true;
                if (ev.Action === 'pass') res.passed = true;
                if (ev.Action === 'fail') res.failed = true;
              }
            }
          }
        } catch {
          // ignore non-JSON line
        }
      });

      child.stderr.on('data', (c) => { stderr += c.toString(); });
      child.on('error', (err) => reject(new Error(`Failed to spawn child: ${err.message}`)));

      child.on('close', (code) => {
        if (code !== 0) {
          return reject(new Error(`Child command exited with code ${code}: ${stderr.trim()}`));
        }
        for (const [pkg, res] of packageResults.entries()) {
          if (!res.executed || !res.passed) {
            return reject(new Error(`TestWindowsBoundarySmoke did not pass in required package: ${pkg}`));
          }
        }
        resolve();
      });
    });
  } finally {
    // Remove only newly created ephemeral files
    if (fs.existsSync(fixtureDir)) {
      fs.rmSync(fixtureDir, { recursive: true, force: true });
    }
  }

  return { success: true, packages: Array.from(packageResults.keys()) };
}

if (process.argv[1] && process.argv[1].endsWith('windows-boundary.mjs')) {
  const parsed = parseArgs(process.argv.slice(2));
  runWindowsBoundary(parsed)
    .then((res) => {
      console.log(`[windows-boundary] PASS: boundary smoke executed in packages: ${res.packages.join(', ')}`);
      process.exit(0);
    })
    .catch((err) => {
      console.error(`[windows-boundary] FAILED: ${err.message}`);
      process.exit(1);
    });
}
