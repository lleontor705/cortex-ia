import fs from 'node:fs';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

function parseArgs(args) {
  const flags = {
    locked: false,
    isolate: false,
    requirePlugin: null,
    requireTypescript: null,
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--locked') {
      flags.locked = true;
    } else if (arg === '--isolate') {
      flags.isolate = true;
    } else if (arg === '--require-plugin' && i + 1 < args.length) {
      flags.requirePlugin = args[++i];
    } else if (arg === '--require-typescript' && i + 1 < args.length) {
      flags.requireTypescript = args[++i];
    }
  }
  return flags;
}

async function qualifySDK() {
  const flags = parseArgs(process.argv.slice(2));

  const pkgPath = path.resolve('internal/tuiassets/package.json');
  const lockPath = path.resolve('internal/tuiassets/package-lock.json');
  const pnpmPath = path.resolve('internal/tuiassets/pnpm-lock.yaml');

  if (!fs.existsSync(pkgPath) || !fs.existsSync(lockPath) || !fs.existsSync(pnpmPath)) {
    console.error('[qualify-harness-sdk] FAILED: Missing required package or lock manifests in internal/tuiassets');
    process.exit(1);
  }

  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
  const lock = JSON.parse(fs.readFileSync(lockPath, 'utf8'));
  const pnpmContent = fs.readFileSync(pnpmPath, 'utf8');

  const pluginVer = pkg.devDependencies?.['@opencode-ai/plugin'];
  const tsVer = pkg.devDependencies?.['typescript'];

  if (flags.requirePlugin && pluginVer !== flags.requirePlugin) {
    console.error(`[qualify-harness-sdk] FAILED: @opencode-ai/plugin version mismatch. Expected ${flags.requirePlugin}, got ${pluginVer}`);
    process.exit(1);
  }

  if (flags.requireTypescript && tsVer !== flags.requireTypescript) {
    console.error(`[qualify-harness-sdk] FAILED: typescript version mismatch. Expected ${flags.requireTypescript}, got ${tsVer}`);
    process.exit(1);
  }

  // Check package-lock.json packages
  const lockPlugin = lock.packages?.['node_modules/@opencode-ai/plugin'];
  const lockSDK = lock.packages?.['node_modules/@opencode-ai/sdk'];
  const lockTS = lock.packages?.['node_modules/typescript'];

  if (!lockPlugin || !lockPlugin.version || !lockPlugin.integrity) {
    console.error('[qualify-harness-sdk] FAILED: Missing @opencode-ai/plugin or integrity in package-lock.json');
    process.exit(1);
  }
  if (!lockSDK || !lockSDK.version || !lockSDK.integrity) {
    console.error('[qualify-harness-sdk] FAILED: Missing @opencode-ai/sdk or integrity in package-lock.json');
    process.exit(1);
  }
  if (!lockTS || !lockTS.version || !lockTS.integrity) {
    console.error('[qualify-harness-sdk] FAILED: Missing typescript or integrity in package-lock.json');
    process.exit(1);
  }

  if (lockPlugin.version !== pluginVer) {
    console.error(`[qualify-harness-sdk] FAILED: Lockfile disagreement for @opencode-ai/plugin: ${lockPlugin.version} vs ${pluginVer}`);
    process.exit(1);
  }

  // Check pnpm-lock.yaml agreement
  if (!pnpmContent.includes(`@opencode-ai/plugin@${pluginVer}`)) {
    console.error(`[qualify-harness-sdk] FAILED: pnpm-lock.yaml missing @opencode-ai/plugin@${pluginVer}`);
    process.exit(1);
  }
  if (!pnpmContent.includes(`typescript@${tsVer}`)) {
    console.error(`[qualify-harness-sdk] FAILED: pnpm-lock.yaml missing typescript@${tsVer}`);
    process.exit(1);
  }

  // Exercise isolated real plugin execution
  try {
    const loaded = await loadPluginFile('internal/assets/plugins/cortex-subagent-transport.ts', {
      env: { SUBAGENT_TRANSPORT_LOG: '0' },
      timeoutMs: 4000,
    });
    const instance = await loaded.instantiate({ directory: process.cwd() });
    if (!instance || typeof instance['event'] !== 'function') {
      throw new Error('Plugin failed to return required event hook function');
    }
  } catch (err) {
    console.error('[qualify-harness-sdk] FAILED: Plugin execution check failed:', err.message);
    process.exit(1);
  }

  const receipt = {
    status: 'QUALIFIED',
    qualified_at: new Date().toISOString(),
    plugin: {
      version: lockPlugin.version,
      integrity: lockPlugin.integrity,
    },
    sdk: {
      version: lockSDK.version,
      integrity: lockSDK.integrity,
    },
    typescript: {
      version: lockTS.version,
      integrity: lockTS.integrity,
    },
    flags: {
      locked: flags.locked,
      isolate: flags.isolate,
    },
  };

  console.log('[qualify-harness-sdk] QUALIFIED: plugin: %s, sdk: %s, typescript: %s (locked & isolated)',
    receipt.plugin.version, receipt.sdk.version, receipt.typescript.version);
  console.log(JSON.stringify(receipt, null, 2));
}

qualifySDK().catch((err) => {
  console.error('[qualify-harness-sdk] Unexpected error:', err);
  process.exit(1);
});
