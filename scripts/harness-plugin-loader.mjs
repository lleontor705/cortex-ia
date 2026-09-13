import { createRequire } from 'node:module';
import vm from 'node:vm';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import os from 'node:os';

const require = createRequire(import.meta.url);
const tsPath = path.resolve('internal/tuiassets/node_modules/typescript');
const ts = require(tsPath);

export function transpileTS(tsSource) {
  const result = ts.transpileModule(tsSource, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
      esModuleInterop: true,
    }
  });
  return result.outputText;
}

export function createIsolatedSandbox(options = {}) {
  const allowedBuiltins = new Set([
    'node:crypto', 'crypto',
    'node:path', 'path',
    'node:fs', 'fs',
    'node:buffer', 'buffer',
    'node:url', 'url',
    'node:os', 'os',
  ]);

  const customEnv = { ...(options.env || {}) };
  const mockFs = options.fs || {
    readFileSync: (p) => {
      if (options.virtualFiles && options.virtualFiles[p]) return options.virtualFiles[p];
      return '';
    },
    existsSync: (p) => {
      if (options.virtualFiles && options.virtualFiles[p] !== undefined) return true;
      return false;
    },
    statSync: () => ({ size: 0 }),
    writeFileSync: () => {},
    mkdirSync: () => {},
  };

  const customRequire = (id) => {
    if (id === '@opencode-ai/plugin' || id === '@opencode-ai/sdk') {
      return options.mockPluginSDK || { tool: (def) => def };
    }
    if (id === 'node:crypto' || id === 'crypto') return crypto;
    if (id === 'node:path' || id === 'path') return path;
    if (id === 'node:fs' || id === 'fs') return mockFs;
    if (id === 'node:os' || id === 'os') return os;
    if (id === 'node:child_process' || id === 'child_process') {
      if (options.childProcess) return options.childProcess;
      if (options.allowChildProcess) return require('node:child_process');
      throw new Error(`UNAUTHORIZED_MODULE_IMPORT: ${id} is not permitted in isolated harness sandbox`);
    }
    if (allowedBuiltins.has(id)) {
      return require(id);
    }
    throw new Error(`UNAUTHORIZED_MODULE_IMPORT: ${id} is not permitted in isolated harness sandbox`);
  };

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: customRequire,
    process: {
      env: customEnv,
      cwd: () => options.cwd || '/isolated/harness',
      platform: process.platform,
    },
    console: {
      log: () => {},
      warn: () => {},
      error: () => {},
      info: () => {},
    },
    setTimeout,
    clearTimeout,
    AbortController,
    Promise,
    Math,
    RegExp,
    Symbol,
    Map,
    Set,
    Date,
    JSON,
    Error,
    TypeError,
    Boolean,
    Number,
    String,
    Array,
    Object,
  };

  vm.createContext(sandbox);
  return sandbox;
}

export async function loadPluginSource(tsSource, options = {}) {
  const jsSource = transpileTS(tsSource);
  const sandbox = createIsolatedSandbox(options);

  vm.runInContext(jsSource, sandbox, {
    timeout: options.timeoutMs || 5000,
  });

  const exported = sandbox.module.exports && Object.keys(sandbox.module.exports).length > 0
    ? sandbox.module.exports
    : sandbox.exports;

  const pluginFn = exported.default || exported.CortexSubagentTransportPlugin || exported.CortexDelegationBridge || exported.CortexPlugin || Object.values(exported).find(v => typeof v === 'function');

  return {
    exports: exported,
    pluginFn,
    instantiate: async (context = {}) => {
      if (typeof pluginFn !== 'function') {
        throw new Error('No plugin function found in exported module');
      }
      return await pluginFn(context);
    },
  };
}

export async function loadPluginFile(filePath, options = {}) {
  const resolved = path.resolve(filePath);
  if (!fs.existsSync(resolved)) {
    throw new Error(`Plugin file not found: ${resolved}`);
  }
  const source = fs.readFileSync(resolved, 'utf8');
  return await loadPluginSource(source, options);
}
