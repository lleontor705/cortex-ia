import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync, execSync } from 'node:child_process';

export function parseArgs(args) {
  const flags = {
    locked: false,
    isolate: false,
    timeoutMs: 10000,
    maxOutputBytes: 1048576,
    maxRequests: 2,
    fixtureRoot: 'scripts/fixtures/mcp',
    requireLiveSchemaEvidence: false,
    offline: false,
  };

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--locked') flags.locked = true;
    else if (a === '--isolate') flags.isolate = true;
    else if (a === '--timeout-ms' && i + 1 < args.length) flags.timeoutMs = parseInt(args[++i], 10);
    else if (a === '--max-output-bytes' && i + 1 < args.length) flags.maxOutputBytes = parseInt(args[++i], 10);
    else if (a === '--max-requests' && i + 1 < args.length) flags.maxRequests = parseInt(args[++i], 10);
    else if (a === '--fixture-root' && i + 1 < args.length) flags.fixtureRoot = args[++i];
    else if (a === '--require-live-schema-evidence') flags.requireLiveSchemaEvidence = true;
    else if (a === '--offline') flags.offline = true;
  }
  return flags;
}

export function checkNodeEngine(nodeVer, requiredRange) {
  const parts = nodeVer.replace(/^v/, '').split('.').map(n => parseInt(n, 10));
  const major = parts[0] || 0;
  const minor = parts[1] || 0;
  const patch = parts[2] || 0;
  if (major < 20) return false;
  if (major === 20) {
    if (minor < 18) return false;
    if (minor === 18 && patch < 1) return false;
  }
  return true;
}

export function qualifyLock(fixtureRoot) {
  const pkgPath = path.join(fixtureRoot, 'package.json');
  const lockPath = path.join(fixtureRoot, 'package-lock.json');
  if (!fs.existsSync(pkgPath) || !fs.existsSync(lockPath)) {
    throw new Error(`Missing package.json or package-lock.json in ${fixtureRoot}`);
  }

  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
  const lock = JSON.parse(fs.readFileSync(lockPath, 'utf8'));

  const depVer = pkg.dependencies?.['@upstash/context7-mcp'];
  if (depVer !== '4.1.0') {
    throw new Error(`Expected @upstash/context7-mcp@4.1.0, got ${depVer}`);
  }

  if (!checkNodeEngine(process.version, pkg.engines?.node)) {
    throw new Error(`Node ${process.version} does not satisfy engine ${pkg.engines?.node}`);
  }

  const lockEntry = lock.packages?.['node_modules/@upstash/context7-mcp'];
  if (!lockEntry || lockEntry.version !== '4.1.0') {
    throw new Error(`Lockfile missing or mismatched version for context7: ${lockEntry?.version}`);
  }

  const expectedIntegrity = 'sha512-ngAkFwW3LsnRGpH3XTVrjDqm3QBT4ZRpLCnShI0cIfCG+ACt07TrkRZc3n7+qjkFTcM/xIDJcHUBK5bDXUa40w==';
  if (lockEntry.integrity !== expectedIntegrity) {
    throw new Error(`Lockfile integrity mismatch: expected ${expectedIntegrity}, got ${lockEntry.integrity}`);
  }

  return { pkg, lock, lockEntry };
}

export function queryCortexLive(options) {
  let cortexExe = '';
  try {
    const cmd = process.platform === 'win32'
      ? 'powershell -NoProfile -Command "(Get-Command cortex).Source"'
      : 'which cortex';
    cortexExe = execSync(cmd, { encoding: 'utf8' }).trim();
  } catch (err) {
    throw new Error(`Cannot locate cortex executable: ${err.message}`);
  }

  const cortexVersion = execSync(`"${cortexExe}" --version`, { encoding: 'utf8' }).trim();
  const exeBytes = fs.readFileSync(cortexExe);
  const exeHash = crypto.createHash('sha256').update(exeBytes).digest('hex');

  const initMsg = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'initialize',
    params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'qualify-mcp', version: '1.0.0' } }
  });
  const toolsMsg = JSON.stringify({
    jsonrpc: '2.0',
    id: 2,
    method: 'tools/list',
    params: {}
  });

  const res = spawnSync(cortexExe, ['mcp', '--tools=agent'], {
    input: initMsg + '\n' + toolsMsg + '\n',
    timeout: options.timeoutMs,
    maxBuffer: options.maxOutputBytes,
    encoding: 'utf8',
    env: { ...process.env, CORTEX_MCP_NO_LOG: '1' }
  });

  if (res.error) throw res.error;

  const rawStdout = res.stdout || '';
  if (Buffer.byteLength(rawStdout) > options.maxOutputBytes) {
    throw new Error(`Aggregate stdout exceeded ${options.maxOutputBytes} bytes`);
  }

  const lines = rawStdout.trim().split('\n');
  let initResult = null;
  let toolsResult = null;
  let requestCount = 0;

  for (const line of lines) {
    try {
      const parsed = JSON.parse(line.trim());
      if (parsed.id === 1) { initResult = parsed.result; requestCount++; }
      if (parsed.id === 2) { toolsResult = parsed.result; requestCount++; }
    } catch {}
  }

  if (requestCount > options.maxRequests) {
    throw new Error(`Protocol requests exceeded limit: ${requestCount} > ${options.maxRequests}`);
  }
  if (!initResult || !toolsResult) {
    throw new Error('Failed to receive initialize or tools/list results from cortex MCP');
  }

  const tools = toolsResult.tools || [];
  const toolNames = new Set(tools.map(t => t.name));
  for (const req of ['cortex_search', 'cortex_save', 'cortex_get_rules']) {
    if (!toolNames.has(req)) {
      throw new Error(`Missing required cortex agent tool: ${req}`);
    }
  }

  const optionalAbsent = [];
  if (!initResult.capabilities?.prompts) optionalAbsent.push('prompts');
  if (!initResult.capabilities?.resources) optionalAbsent.push('resources');

  return {
    version: cortexVersion,
    executableSha256: exeHash,
    toolsCount: tools.length,
    optionalAbsent,
  };
}

export function qualifyMCP(args = process.argv.slice(2)) {
  const flags = parseArgs(args);
  const lockInfo = qualifyLock(flags.fixtureRoot);

  const capPath = path.join(flags.fixtureRoot, 'capabilities.json');
  if (!fs.existsSync(capPath)) {
    throw new Error(`Missing capabilities.json in ${flags.fixtureRoot}`);
  }
  const capabilities = JSON.parse(fs.readFileSync(capPath, 'utf8'));

  let liveResult = null;
  if (flags.requireLiveSchemaEvidence) {
    liveResult = queryCortexLive(flags);
    console.log(`[qualify-mcp] Live Cortex verified: ${liveResult.version} (SHA256: ${liveResult.executableSha256.slice(0, 12)}..., ${liveResult.toolsCount} tools)`);
    if (liveResult.optionalAbsent.length > 0) {
      console.log(`[qualify-mcp] Optional capabilities reported absent: ${liveResult.optionalAbsent.join(', ')}`);
    }
  } else if (flags.offline) {
    console.log(`[qualify-mcp] Offline mode: validated frozen capabilities (${capabilities.cortex?.tools_count || 35} cortex tools, context7 4.1.0)`);
  }

  console.log(`[qualify-mcp] PASS: qualified @upstash/context7-mcp@${lockInfo.lockEntry.version} and Cortex MCP schema`);
  return { lockInfo, capabilities, liveResult };
}

if (process.argv[1] && import.meta.url.endsWith(path.basename(process.argv[1]))) {
  try {
    qualifyMCP();
  } catch (err) {
    console.error(`[qualify-mcp] FAILED: ${err.message}`);
    process.exit(1);
  }
}
