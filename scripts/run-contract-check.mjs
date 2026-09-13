import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import readline from 'node:readline';

export function parseArgs(argv) {
  let cases = [];
  let maxBytes = 10 * 1024 * 1024;
  let requireExactGo = true;
  let command = null;
  let cmdArgs = [];

  const doubleDashIdx = argv.indexOf('--');
  const checkArgs = doubleDashIdx >= 0 ? argv.slice(0, doubleDashIdx) : argv;
  const targetCmd = doubleDashIdx >= 0 ? argv.slice(doubleDashIdx + 1) : [];

  for (let i = 0; i < checkArgs.length; i++) {
    if (checkArgs[i] === '--cases') {
      const val = checkArgs[++i];
      if (val) {
        cases = val.split(',').map((c) => c.trim()).filter(Boolean);
      }
    } else if (checkArgs[i] === '--max-bytes') {
      maxBytes = parseInt(checkArgs[++i], 10) || maxBytes;
    } else if (checkArgs[i] === '--no-go-version-check') {
      requireExactGo = false;
    }
  }

  if (targetCmd.length > 0) {
    command = targetCmd[0];
    cmdArgs = targetCmd.slice(1);
  }

  return { cases, maxBytes, requireExactGo, command, cmdArgs };
}

export function parseGoEvents(lines, requiredCases = []) {
  if (!Array.isArray(requiredCases) || requiredCases.length === 0) {
    throw new Error('At least one required case must be specified');
  }

  const executed = new Map();
  for (const c of requiredCases) {
    executed.set(c, { run: false, pass: false, skip: false, fail: false });
  }

  let lineCount = 0;
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    lineCount++;
    let ev;
    try {
      ev = JSON.parse(trimmed);
    } catch {
      throw new Error(`Malformed JSON event on line ${lineCount}: ${trimmed.slice(0, 50)}`);
    }

    if (!ev || typeof ev !== 'object') {
      throw new Error(`Invalid event structure on line ${lineCount}`);
    }

    const testName = ev.Test;
    if (testName && executed.has(testName)) {
      const state = executed.get(testName);
      if (ev.Action === 'run') state.run = true;
      if (ev.Action === 'pass') state.pass = true;
      if (ev.Action === 'skip') state.skip = true;
      if (ev.Action === 'fail') state.fail = true;
    }
  }

  if (lineCount === 0) {
    throw new Error('Missing test events: empty event stream');
  }

  const missing = [];
  const skipped = [];
  const failed = [];

  for (const [name, state] of executed.entries()) {
    if (!state.run || !state.pass) {
      if (state.skip) skipped.push(name);
      else if (state.fail) failed.push(name);
      else missing.push(name);
    }
  }

  if (skipped.length > 0) {
    throw new Error(`Required test cases were skipped: ${skipped.join(', ')}`);
  }
  if (failed.length > 0) {
    throw new Error(`Required test cases failed: ${failed.join(', ')}`);
  }
  if (missing.length > 0) {
    throw new Error(`Required test cases were not executed: ${missing.join(', ')}`);
  }

  return {
    valid: true,
    executedCases: Array.from(executed.keys()),
    totalLines: lineCount,
  };
}

export async function runContractCheck(options = {}) {
  const {
    cases = [],
    command,
    cmdArgs = [],
    maxBytes = 10 * 1024 * 1024,
    cwd = process.cwd(),
    env = { ...process.env },
  } = options;

  if (!cases || cases.length === 0) {
    throw new Error('--cases must specify at least one named test');
  }

  if (!command) {
    throw new Error('Child command is required after --');
  }

  const fullCmd = [command, ...cmdArgs].join(' ');
  if (/go\s+test\s+(?:-json\s+)?(?:\.\/\.\.\.)(?!\s+-run)/i.test(fullCmd)) {
    throw new Error('Full-suite execution rejected without I20 isolated-fixture evidence');
  }

  if (path.basename(command).startsWith('go')) {
    env.GOTOOLCHAIN = 'local';
  }

  return new Promise((resolve, reject) => {
    const child = spawn(command, cmdArgs, {
      cwd,
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    const lines = [];
    let bytesRead = 0;
    let truncated = false;

    const rl = readline.createInterface({ input: child.stdout });
    rl.on('line', (line) => {
      bytesRead += Buffer.byteLength(line, 'utf8') + 1;
      if (bytesRead > maxBytes) {
        truncated = true;
        child.kill();
        return;
      }
      lines.push(line);
    });

    let stderr = '';
    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('error', (err) => {
      reject(new Error(`Failed to spawn child command: ${err.message}`));
    });

    child.on('close', (code) => {
      if (truncated) {
        return reject(new Error(`Output exceeded max bytes limit (${maxBytes} bytes)`));
      }
      if (code !== 0) {
        return reject(new Error(`Child command exited with code ${code}: ${stderr.trim()}`));
      }

      try {
        const result = parseGoEvents(lines, cases);
        resolve(result);
      } catch (err) {
        reject(err);
      }
    });
  });
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const parsed = parseArgs(process.argv.slice(2));
  runContractCheck(parsed)
    .then((res) => {
      console.log(`[run-contract-check] PASS: all ${res.executedCases.length} cases verified: ${res.executedCases.join(', ')}`);
      process.exit(0);
    })
    .catch((err) => {
      console.error(`[run-contract-check] FAILED: ${err.message}`);
      process.exit(1);
    });
}
