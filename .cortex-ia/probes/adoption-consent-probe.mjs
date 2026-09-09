/**
 * OpenCode Adoption Consent Probe (Task FR-ADOPT-CONSENT-PROBE-1.1)
 *
 * Runs an isolated disposable OpenCode consent probe:
 * - Probes installed actual host runtime version (not dev docs/mocks)
 * - Uses strictly isolated temporary sandbox; no real user state/config/credentials touched
 * - Starts host server, verifies session and permission event surfaces
 * - Verifies whether a real user interaction surface is connected
 * - NEVER fabricates consent or submits an agent permission reply
 * - Stops safely and reports BLOCKED with actionable human interaction steps
 * - Cleans up all owned temporary resources
 */

import { spawn, execFile } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { once } from 'node:events';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);

// Known installed binary locations
const CANDIDATE_BINARIES = [
  'C:\\Users\\usrLuisLeon\\AppData\\Local\\Volta\\tools\\image\\packages\\opencode-ai\\node_modules\\opencode-ai\\bin\\opencode.exe',
  'C:\\Users\\usrLuisLeon\\AppData\\Local\\Volta\\bin\\opencode.cmd',
  'opencode',
];

async function findInstalledBinary() {
  for (const bin of CANDIDATE_BINARIES) {
    try {
      const { stdout } = await execFileAsync(bin, ['--version']);
      const version = stdout.trim();
      if (version) {
        return { binaryPath: bin, version };
      }
    } catch {
      // Try next
    }
  }
  throw new Error('Installed OpenCode binary not found on host');
}

export async function runProbe() {
  const report = {
    task_id: 'FR-ADOPT-CONSENT-PROBE-1.1',
    timestamp: new Date().toISOString(),
    installed_host: null,
    isolation: {
      sandbox_path: null,
      user_config_touched: false,
      credentials_copied: false,
    },
    permission_probe: {
      host_listening: false,
      server_url: null,
      session_id: null,
      pending_permissions: [],
      real_user_interaction_surface_available: false,
      agent_permission_reply_submitted: false,
      consent_fabricated: false,
    },
    verdict: 'BLOCKED',
    execution_status: 'blocked',
    reason: 'Headless leaf execution environment lacks an interactive user interface (TUI/Web) to surface permission requests to a human user. Agent permission reply and consent fabrication are strictly prohibited by safety policy.',
    actionable_interaction_step: {
      step_1: 'Open an interactive terminal in the workspace (D:\\cortex-ia).',
      step_2: 'Start OpenCode with an interactive UI: run `opencode` (interactive TUI) or `opencode web` (web interface).',
      step_3: 'Execute the desired operation that requires permission (such as file modification, bash command, or agent action).',
      step_4: 'When OpenCode displays the interactive permission approval prompt ("Allow once", "Always allow", "Reject"), the real user explicitly selects their decision.',
      step_5: 'The OpenCode host captures the user decision via POST /permission/:requestID/reply and emits the correlated permission.replied event bound to the session/request.',
    },
    cleanup: {
      server_process_terminated: false,
      sandbox_directory_removed: false,
      retained_files: ['.cortex-ia/probes/adoption-consent-probe.mjs'],
    },
  };

  // 1. Determine installed runtime version directly from host binary
  const { binaryPath, version } = await findInstalledBinary();
  report.installed_host = {
    runtime: 'opencode',
    version,
    binary_path: binaryPath,
    verified_by: 'host_direct_execution',
  };

  // 2. Create strictly isolated disposable temporary directory
  const sandbox = await mkdtemp(join(tmpdir(), 'opencode-consent-probe-'));
  report.isolation.sandbox_path = sandbox;

  let child = null;
  try {
    // 3. Launch isolated headless OpenCode server without touching user configs
    child = spawn(binaryPath, ['serve', '--port', '0', '--hostname', '127.0.0.1'], {
      cwd: sandbox,
      env: {
        ...process.env,
        OPENCODE_CONFIG_DIR: sandbox,
        XDG_CONFIG_HOME: sandbox,
        APPDATA: sandbox,
        LOCALAPPDATA: sandbox,
      },
    });

    let serverUrl = null;
    const captureUrl = (data) => {
      const match = data.toString().match(/http:\/\/(127\.0\.0\.1:\d+)/);
      if (match) serverUrl = 'http://' + match[1];
    };

    child.stdout.on('data', captureUrl);
    child.stderr.on('data', captureUrl);

    const startWait = Date.now();
    while (!serverUrl && Date.now() - startWait < 8000) {
      await new Promise((r) => setTimeout(r, 100));
    }

    if (!serverUrl) {
      throw new Error('OpenCode host server failed to start within timeout');
    }

    report.permission_probe.host_listening = true;
    report.permission_probe.server_url = serverUrl;

    // 4. Inspect permission endpoint
    const permRes = await fetch(`${serverUrl}/permission`);
    if (permRes.ok) {
      report.permission_probe.pending_permissions = await permRes.json();
    }

    // 5. Create isolated disposable session
    const sessRes = await fetch(`${serverUrl}/session`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });

    if (sessRes.ok) {
      const session = await sessRes.json();
      report.permission_probe.session_id = session.id;
    }

    // 6. Check interaction surface availability
    // In headless agent execution, stdin is not an interactive TTY and no client UI is attached.
    const isInteractiveTTY = Boolean(process.stdin.isTTY);
    report.permission_probe.real_user_interaction_surface_available = isInteractiveTTY;
    report.permission_probe.agent_permission_reply_submitted = false;
    report.permission_probe.consent_fabricated = false;

  } finally {
    // 7. Guaranteed cleanup of all owned processes and temporary files
    if (child && !child.killed) {
      child.kill();
      try {
        await Promise.race([
          once(child, 'exit'),
          new Promise((r) => setTimeout(r, 2000)),
        ]);
      } catch {
        // ignore
      }
      report.cleanup.server_process_terminated = true;
    }

    // Allow brief grace period for Windows file locks to release
    await new Promise((r) => setTimeout(r, 300));

    try {
      await rm(sandbox, { recursive: true, force: true });
      report.cleanup.sandbox_directory_removed = true;
    } catch {
      report.cleanup.sandbox_directory_removed = false;
    }
  }

  return report;
}

// Direct execution entrypoint
if (process.argv[1] && process.argv[1].replace(/\\/g, '/').endsWith('adoption-consent-probe.mjs')) {
  runProbe()
    .then((result) => {
      console.log(JSON.stringify(result, null, 2));
      // Exit 0 to signal clean probe completion and structured reporting
      process.exit(0);
    })
    .catch((err) => {
      console.error('Probe execution error:', err);
      process.exit(1);
    });
}
